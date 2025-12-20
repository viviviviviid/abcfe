package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	"github.com/abcfe/abcfe-node/core"
	"github.com/abcfe/abcfe-node/wallet"
)

var (
	baseURL       = "http://localhost:8000/api/v1"
	walletCount   int
	fundingAmount uint64
	sendAmount    uint64
	txFee         uint64
	concurrency   int
	verbose       bool
	walletPath    string
)

type UTXOResponse struct {
	TxID        string `json:"txId"`
	OutputIndex uint64 `json:"outputIndex"`
	Amount      uint64 `json:"amount"`
}

type WalletInfo struct {
	Index     int
	Address   string
	Manager   *wallet.WalletManager
	HasFunds  bool
	Spent     bool
}

type Stats struct {
	TotalTx       int64
	SuccessTx     int64
	FailedTx      int64
	StartTime     time.Time
	EndTime       time.Time
}

func main() {
	// Parse flags
	flag.IntVar(&walletCount, "wallets", 10, "Number of wallets to create from mnemonic")
	flag.Uint64Var(&fundingAmount, "fund", 1000, "Amount to fund each wallet")
	flag.Uint64Var(&sendAmount, "send", 100, "Amount each wallet sends")
	flag.Uint64Var(&txFee, "fee", 1, "Transaction fee")
	flag.IntVar(&concurrency, "concurrency", 5, "Number of concurrent senders")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.StringVar(&baseURL, "url", "http://localhost:8000/api/v1", "Base API URL")
	flag.StringVar(&walletPath, "wallet", "./resource/wallet", "Path to server wallet (Node 1)")
	flag.Parse()

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║         ABCFe Load Test Tool                 ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Wallets:     %d\n", walletCount)
	fmt.Printf("  Fund Amount: %d\n", fundingAmount)
	fmt.Printf("  Send Amount: %d\n", sendAmount)
	fmt.Printf("  Fee:         %d\n", txFee)
	fmt.Printf("  Concurrency: %d\n", concurrency)
	fmt.Printf("  API URL:     %s\n", baseURL)
	fmt.Printf("  Wallet Path: %s\n\n", walletPath)

	// Step 1: Load server wallet (Node 1) to get mnemonic
	fmt.Println("[1/5] Loading server wallet mnemonic...")
	serverWM := wallet.NewWalletManager(walletPath)
	if err := serverWM.LoadWalletFile(); err != nil {
		panic(fmt.Sprintf("Failed to load server wallet: %v\n  Make sure the wallet exists at: %s", err, walletPath))
	}

	mnemonic, err := serverWM.GetMnemonic()
	if err != nil {
		panic(fmt.Sprintf("Failed to get mnemonic: %v", err))
	}
	fmt.Printf("  ✓ Loaded mnemonic from server wallet\n")
	if verbose {
		fmt.Printf("  Mnemonic: %s\n", mnemonic)
	}

	// Step 2: Derive N wallets from the same mnemonic
	// Note: Index 0 is the server wallet (genesis), so we start from index 1
	fmt.Printf("\n[2/5] Deriving %d wallets from mnemonic (index 1-%d)...\n", walletCount, walletCount)
	wallets := make([]*WalletInfo, walletCount)

	for i := 0; i < walletCount; i++ {
		accountIndex := i + 1 // Start from index 1 (index 0 is server wallet)

		wm := wallet.NewWalletManager(fmt.Sprintf("/tmp/load_test_wallet_%d", accountIndex))
		_, err := wm.RestoreWallet(mnemonic)
		if err != nil {
			panic(fmt.Sprintf("Failed to restore wallet %d: %v", accountIndex, err))
		}

		// Derive additional accounts to reach target index
		for j := 1; j <= accountIndex; j++ {
			_, err = wm.AddAccount()
			if err != nil {
				panic(fmt.Sprintf("Failed to add account %d: %v", j, err))
			}
		}

		// Switch to the target account
		if err := wm.SwitchAccount(accountIndex); err != nil {
			panic(fmt.Sprintf("Failed to switch to account %d: %v", accountIndex, err))
		}

		account, err := wm.GetCurrentAccount()
		if err != nil {
			panic(fmt.Sprintf("Failed to get account: %v", err))
		}

		addrStr := utils.AddressToString(account.Address)
		wallets[i] = &WalletInfo{
			Index:   accountIndex,
			Address: addrStr,
			Manager: wm,
		}

		if verbose {
			fmt.Printf("  Wallet %d (index %d): %s\n", i+1, accountIndex, addrStr)
		}
	}
	fmt.Printf("  ✓ Created %d wallets (index 1-%d)\n", walletCount, walletCount)

	// Step 3: Fund all wallets using server wallet
	fmt.Printf("\n[3/5] Funding %d wallets...\n", walletCount)
	fundedCount := 0
	for _, w := range wallets {
		if fundWallet(w.Address, fundingAmount) {
			fundedCount++
			if verbose {
				fmt.Printf("  ✓ Funded wallet %d: %s\n", w.Index, w.Address[:16])
			}
		} else {
			fmt.Printf("  ✗ Failed to fund wallet %d\n", w.Index)
		}
		// Small delay to avoid overwhelming the server
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Printf("  ✓ Funded %d/%d wallets\n", fundedCount, walletCount)

	// Step 4: Wait for confirmations
	fmt.Println("\n[4/5] Waiting for block confirmations...")
	confirmedCount := waitForConfirmations(wallets, 120*time.Second)
	fmt.Printf("  ✓ Confirmed %d/%d wallets have funds\n", confirmedCount, walletCount)

	// Step 5: Send transactions from each wallet (only once per wallet)
	fmt.Printf("\n[5/5] Sending transactions from %d wallets...\n", confirmedCount)

	stats := &Stats{StartTime: time.Now()}

	// Target address (burn address)
	targetAddr := "0000000000000000000000000000000000000001"

	// Use worker pool for concurrent sending
	jobs := make(chan *WalletInfo, walletCount)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for w := range jobs {
				if w.HasFunds && !w.Spent {
					success := sendTransaction(w, targetAddr, sendAmount, txFee)
					atomic.AddInt64(&stats.TotalTx, 1)
					if success {
						atomic.AddInt64(&stats.SuccessTx, 1)
						w.Spent = true
						if verbose {
							fmt.Printf("  [Worker %d] ✓ Wallet %d sent tx\n", workerID, w.Index)
						}
					} else {
						atomic.AddInt64(&stats.FailedTx, 1)
						if verbose {
							fmt.Printf("  [Worker %d] ✗ Wallet %d failed\n", workerID, w.Index)
						}
					}
				}
			}
		}(i)
	}

	// Queue jobs
	for _, w := range wallets {
		jobs <- w
	}
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()
	stats.EndTime = time.Now()

	// Print results
	fmt.Println("\n╔══════════════════════════════════════════════╗")
	fmt.Println("║              Load Test Results               ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	duration := stats.EndTime.Sub(stats.StartTime)
	tps := float64(stats.SuccessTx) / duration.Seconds()

	fmt.Printf("\n  Total Transactions:   %d\n", stats.TotalTx)
	fmt.Printf("  Successful:           %d\n", stats.SuccessTx)
	fmt.Printf("  Failed:               %d\n", stats.FailedTx)
	fmt.Printf("  Duration:             %v\n", duration.Round(time.Millisecond))
	fmt.Printf("  TPS (Transactions/s): %.2f\n", tps)
	fmt.Printf("  Success Rate:         %.1f%%\n", float64(stats.SuccessTx)/float64(stats.TotalTx)*100)
	fmt.Println()
}

func fundWallet(toAddr string, amount uint64) bool {
	req := map[string]interface{}{
		"accountIndex": 0,
		"to":           toAddr,
		"amount":       amount,
		"fee":          txFee,
		"memo":         "load test funding",
	}
	body, _ := json.Marshal(req)

	resp, err := http.Post(baseURL+"/tx/send", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func waitForConfirmations(wallets []*WalletInfo, timeout time.Duration) int {
	start := time.Now()
	confirmedCount := 0

	for time.Since(start) < timeout {
		confirmedCount = 0
		for _, w := range wallets {
			if w.HasFunds {
				confirmedCount++
				continue
			}

			utxos := getUtxos(w.Address)
			if len(utxos) > 0 {
				w.HasFunds = true
				confirmedCount++
			}
		}

		fmt.Printf("\r  Confirmed: %d/%d (%.0fs elapsed)", confirmedCount, len(wallets), time.Since(start).Seconds())

		if confirmedCount == len(wallets) {
			fmt.Println()
			return confirmedCount
		}

		time.Sleep(2 * time.Second)
	}

	fmt.Println()
	return confirmedCount
}

func sendTransaction(w *WalletInfo, toAddrStr string, amount, fee uint64) bool {
	// Get UTXOs
	utxos := getUtxos(w.Address)
	if len(utxos) == 0 {
		return false
	}

	utxo := utxos[0]
	if utxo.Amount < amount+fee {
		return false
	}

	// Get keys
	privateKey, err := w.Manager.GetCurrentPrivateKey()
	if err != nil {
		return false
	}

	publicKey := &privateKey.PublicKey
	pubKeyBytes, err := crypto.PublicKeyToBytes(publicKey)
	if err != nil {
		return false
	}

	// Parse addresses
	toAddr, err := utils.StringToAddress(toAddrStr)
	if err != nil {
		return false
	}

	myAddr, err := utils.StringToAddress(w.Address)
	if err != nil {
		return false
	}

	inputTxID, err := utils.StringToHash(utxo.TxID)
	if err != nil {
		return false
	}

	// Build transaction
	tx := &core.Transaction{
		Version:   "1.0.0",
		Timestamp: time.Now().Unix(),
		Memo:      "load test tx",
		Data:      []byte{},
		Inputs: []*core.TxInput{
			{
				TxID:        inputTxID,
				OutputIndex: utxo.OutputIndex,
				PublicKey:   pubKeyBytes,
			},
		},
		Outputs: []*core.TxOutput{
			{
				Address: toAddr,
				Amount:  amount,
				TxType:  core.TxTypeGeneral,
			},
		},
	}

	// Add change output
	change := utxo.Amount - amount - fee
	if change > 0 {
		tx.Outputs = append(tx.Outputs, &core.TxOutput{
			Address: myAddr,
			Amount:  change,
			TxType:  core.TxTypeGeneral,
		})
	}

	// Calculate hash and sign
	tx.ID = utils.Hash(tx)
	txHashBytes := utils.HashToBytes(tx.ID)
	sig, err := crypto.SignData(privateKey, txHashBytes)
	if err != nil {
		return false
	}

	// Build API request
	hexSig := utils.SignatureToString(sig)
	hexPub := hex.EncodeToString(pubKeyBytes)

	outputs := []map[string]interface{}{
		{
			"address": toAddrStr,
			"amount":  amount,
			"txType":  core.TxTypeGeneral,
		},
	}

	if change > 0 {
		outputs = append(outputs, map[string]interface{}{
			"address": w.Address,
			"amount":  change,
			"txType":  core.TxTypeGeneral,
		})
	}

	req := map[string]interface{}{
		"version":   tx.Version,
		"timestamp": tx.Timestamp,
		"memo":      tx.Memo,
		"data":      []byte{},
		"inputs": []map[string]interface{}{
			{
				"txId":        utxo.TxID,
				"outputIndex": utxo.OutputIndex,
				"signature":   hexSig,
				"publicKey":   hexPub,
			},
		},
		"outputs": outputs,
	}

	reqBody, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/tx/signed", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func getUtxos(addr string) []UTXOResponse {
	resp, err := http.Get(baseURL + "/address/" + addr + "/utxo")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var apiResp struct {
		Data struct {
			Utxos []UTXOResponse `json:"utxos"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil
	}

	return apiResp.Data.Utxos
}
