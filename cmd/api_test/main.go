package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	"github.com/abcfe/abcfe-node/core"
	"github.com/abcfe/abcfe-node/wallet"
)

const (
	BaseURL = "http://localhost:8000/api/v1"
)

type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

type UTXOResponse struct {
	TxID        string `json:"txId"`
	OutputIndex uint64 `json:"outputIndex"`
	Amount      uint64 `json:"amount"`
}

func main() {
	fmt.Println("=== Starting API Test ===")

	// 1. Create a new separate wallet
	fmt.Println("\n[1] Creating separate wallet...")
	w := wallet.NewWallet(nil)
	if err := w.CreateWallet(); err != nil {
		panic(fmt.Sprintf("Failed to create wallet: %v", err))
	}
	myAddr := utils.AddressToString(w.Address)
	fmt.Printf("Created Test Wallet Address: %s\n", myAddr)
	printKeys(w)

	// 2. Fund the wallet using Server Wallet (/tx/send)
	fmt.Println("\n[2] Funding wallet via /tx/send...")
	// We use Account Index 0 of the server wallet (Node 1)
	sendReq := map[string]interface{}{
		"accountIndex": 0,
		"to":           myAddr,
		"amount":       1000,
		"fee":          1,
		"memo":         "funding test wallet",
	}
	sendBody, _ := json.Marshal(sendReq)
	
	resp, err := http.Post(BaseURL+"/tx/send", "application/json", bytes.NewBuffer(sendBody))
	if err != nil {
		panic(fmt.Sprintf("Failed to send funding tx: %v", err))
	}
	readAndPrintResponse(resp)
	if resp.StatusCode != 200 {
		panic("Funding failed check server logs")
	}

	// Wait for confirmation (Polling)
	fmt.Println("Waiting for block confirmation (polling for UTXOs)...")
	maxWait := 60 * time.Second
	start := time.Now()
	var fundsConfirmed bool

	for time.Since(start) < maxWait {
		utxos := getUtxos(myAddr)
		if len(utxos) > 0 {
			fundsConfirmed = true
			break
		}
		fmt.Print(".")
		time.Sleep(1 * time.Second)
	}
	fmt.Println()

	if !fundsConfirmed {
		panic("Timeout waiting for funding transaction confirmation")
	}

	// Check balance
	checkBalance(myAddr)

	// 3. Create and Send Signed TX (/tx/signed)
	fmt.Println("\n[3] Testing /tx/signed...")
	
	// Get UTXOs
	utxos := getUtxos(myAddr)
	if len(utxos) == 0 {
		panic("No UTXOs found for test wallet, cannot proceed with signing test")
	}
	
	fmt.Printf("Found %d UTXOs\n", len(utxos))
	
	// Target Address (send back to genesis or burn)
	toAddrStr := "0000000000000000000000000000000000000001" 
	toAddr, _ := utils.StringToAddress(toAddrStr)
	
	// Prepare Tx Data
	utxo := utxos[0] // Use the first UTXO
	amountToSend := uint64(500)
	fee := uint64(1)
	
	fmt.Printf("Using UTXO: %s (Idx: %d, Amt: %d)\n", utxo.TxID, utxo.OutputIndex, utxo.Amount)

	if utxo.Amount < amountToSend+fee {
		panic("Not enough balance in first UTXO")
	}

	// Construct Transaction object to calculate Hash and Sign
	// Note: We used normalized empty slices in core/transaction.go, we should mimic that if needed,
	// but mostly we just need correct inputs/outputs structure for hashing.

	tx := &core.Transaction{
		Version:   "1.0.0",
		Timestamp: time.Now().Unix(),
		Memo:      "signed by client test",
		Data:      []byte{}, // Empty data
	}
	
	// Inputs
	// We need PubKey in bytes
	pubKeyBytes, err := crypto.PublicKeyToBytes(w.PublicKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert public key: %v", err))
	}
	
	inputTxID, _ := utils.StringToHash(utxo.TxID)
	
	tx.Inputs = []*core.TxInput{
		{
			TxID:        inputTxID,
			OutputIndex: utxo.OutputIndex,
			PublicKey:   pubKeyBytes,
			// Signature is empty for hashing (usually, but verify core/transaction.go)
		},
	}

	// Outputs
	tx.Outputs = []*core.TxOutput{
		{
			Address: toAddr,
			Amount:  amountToSend,
			TxType:  core.TxTypeGeneral, // General
		},
	}
	
	// Change Output
	change := utxo.Amount - amountToSend - fee
	if change > 0 {
		tx.Outputs = append(tx.Outputs, &core.TxOutput{
			Address: w.Address,
			Amount:  change,
			TxType:  core.TxTypeGeneral,
		})
	}

	// Calculate Hash
	tx.ID = utils.Hash(tx)
	
	fmt.Printf("Calculated TX ID (Client): %s\n", utils.HashToString(tx.ID))
	fmt.Printf("TX Dump (Client): %+v\n", tx)

	// Sign
	txHashBytes := utils.HashToBytes(tx.ID)
	sig, err := crypto.SignData(w.PrivateKey, txHashBytes)
	if err != nil {
		panic(err)
	}

	// Construct API Request
	hexSig := utils.SignatureToString(sig)
	hexPub := hex.EncodeToString(pubKeyBytes)

	req := map[string]interface{}{
		"version":   tx.Version,
		"timestamp": tx.Timestamp,
		"memo":      tx.Memo,
		"data":      []byte{}, // Marshals to base64 string automatically
		"inputs": []map[string]interface{}{
			{
				"txId":        utxo.TxID,
				"outputIndex": utxo.OutputIndex,
				"signature":   hexSig,
				"publicKey":   hexPub,
			},
		},
		"outputs": []map[string]interface{}{
			{
				"address": toAddrStr,
				"amount":  amountToSend,
				"txType":  core.TxTypeGeneral,
			},
		},
	}

	if change > 0 {
		req["outputs"] = append(req["outputs"].([]map[string]interface{}), map[string]interface{}{
			"address": myAddr,
			"amount":  change,
			"txType":  core.TxTypeGeneral,
		})
	}

	reqBody, _ := json.MarshalIndent(req, "", "  ")
	fmt.Println("Sending /tx/signed Request:")
	// fmt.Println(string(reqBody))

	resp2, err := http.Post(BaseURL+"/tx/signed", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		panic(err)
	}
	
	body2, _ := ioutil.ReadAll(resp2.Body)
	resp2.Body.Close()
	
	fmt.Printf("Response: %s\n", string(body2))
	
	if resp2.StatusCode == 200 {
		fmt.Println("\n✅ SUCCESS: /tx/signed passed")
	} else {
		fmt.Println("\n❌ FAILED: /tx/signed failed")
	}
}

func readAndPrintResponse(resp *http.Response) {
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Status: %s\nBody: %s\n", resp.Status, string(body))
}

func checkBalance(addr string) {
	resp, err := http.Get(BaseURL + "/address/" + addr + "/balance")
	if err != nil {
		fmt.Printf("Failed to get balance: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("Balance Info: %s\n", string(body))
}

func getUtxos(addr string) []UTXOResponse {
	resp, err := http.Get(BaseURL + "/address/" + addr + "/utxo")
	if err != nil {
		fmt.Printf("Failed to get UTXOs: %v\n", err)
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
		fmt.Printf("Failed to parse UTXO response: %v\n", err)
		return nil
	}
	
	return apiResp.Data.Utxos
}

func printKeys(w *wallet.Wallet) {
	privBytes, _ := crypto.PrivateKeyToBytes(w.PrivateKey)
	priv := hex.EncodeToString(privBytes)
	
	pubBytes, _ := crypto.PublicKeyToBytes(w.PublicKey)
	pub := hex.EncodeToString(pubBytes)
	
	fmt.Printf("  Priv: %s...\n", priv[:10])
	fmt.Printf("  Pub:  %s...\n", pub[:10])
}
