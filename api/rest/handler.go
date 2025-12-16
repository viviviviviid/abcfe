package rest

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/abcfe/abcfe-node/api"
	"github.com/abcfe/abcfe-node/common/utils"
	"github.com/abcfe/abcfe-node/consensus"
	"github.com/abcfe/abcfe-node/core"
	"github.com/abcfe/abcfe-node/p2p"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/abcfe/abcfe-node/wallet"
	"github.com/gorilla/mux"
)

// get home response
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	info := map[string]string{
		"name":    "ABCFE Blockchain API",
		"version": "1.0.0",
	}
	sendResp(w, http.StatusOK, info, nil)
}

// get chain status response
func GetStatus(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := bc.GetChainStatus()

		// Get Genesis block hash
		genesisHash := ""
		if genesisBlock, err := bc.GetBlockByHeight(0); err == nil {
			genesisHash = utils.HashToString(genesisBlock.Header.Hash)
		}

		// Get Network ID (from config)
		networkId := "abcfe-mainnet" // default value

		// Mempool size
		mempoolSize := 0
		if bc != nil && bc.Mempool != nil {
			mempoolSize = len(bc.Mempool.GetTxs())
		}

		response := map[string]interface{}{
			"currentHeight":    status.LatestHeight,
			"currentBlockHash": status.LatestBlockHash,
			"genesisHash":      genesisHash,
			"networkId":        networkId,
			"mempoolSize":      mempoolSize,
		}

		sendResp(w, http.StatusOK, response, nil)
	}
}

// get latest block response
func GetLatestBlock(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		height, err := bc.GetLatestHeight()
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		block, err := bc.GetBlockByHeight(height)
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		response, err := formatBlockResp(block, bc)
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		sendResp(w, http.StatusOK, response, nil)
	}
}

// get block by height response
func GetBlockByHeight(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		heightStr := vars["height"]

		height, err := strconv.ParseUint(heightStr, 10, 64)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		block, err := bc.GetBlockByHeight(height)
		if err != nil {
			sendResp(w, http.StatusNotFound, nil, err)
			return
		}

		response, err := formatBlockResp(block, bc)
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		sendResp(w, http.StatusOK, response, nil)
	}
}

// get block by hash response
func GetBlockByHash(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		hashStr := vars["hash"]

		hash, err := utils.StringToHash(hashStr)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		block, err := bc.GetBlockByHash(hash)
		if err != nil {
			sendResp(w, http.StatusNotFound, nil, err)
			return
		}

		response, err := formatBlockResp(block, bc)
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		sendResp(w, http.StatusOK, response, nil)
	}
}

// get tx response
func GetTx(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		txIDStr := vars["txid"]

		txID, err := utils.StringToHash(txIDStr)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		tx, err := bc.GetTx(txID)
		if err != nil {
			sendResp(w, http.StatusNotFound, nil, err)
			return
		}

		response := formatTxResp(tx, bc)
		sendResp(w, http.StatusOK, response, nil)
	}
}

// get utxo response
func GetAddressUtxo(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		addrStr := vars["address"]

		address, err := utils.StringToAddress(addrStr)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		utxos, err := bc.GetUtxoList(address, false)
		if err != nil {
			sendResp(w, http.StatusNotFound, nil, err)
			return
		}

		response := map[string]interface{}{
			"utxos": formatUtxoResp(utxos),
		}
		sendResp(w, http.StatusOK, response, nil)
	}
}

// get balance response
func GetBalanceByUtxo(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		addrStr := vars["address"]

		address, err := utils.StringToAddress(addrStr)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		balance, err := bc.GetBalance(address)
		if err != nil {
			sendResp(w, http.StatusNotFound, nil, err)
			return
		}

		response := map[string]interface{}{
			"address": addrStr,
			"balance": balance,
		}
		sendResp(w, http.StatusOK, response, nil)
	}
}

// get address transactions response
func GetAddressTransactions(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		addrStr := vars["address"]

		address, err := utils.StringToAddress(addrStr)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		txInfos, err := bc.GetAddressTransactions(address)
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		response := map[string]interface{}{
			"address":      addrStr,
			"transactions": formatAddressTxInfos(txInfos),
		}
		sendResp(w, http.StatusOK, response, nil)
	}
}

func SubmitTransferTx(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SubmitTxReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		from, err := utils.StringToAddress(req.From)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		to, err := utils.StringToAddress(req.To)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		// If fee is 0, apply minimum fee
		fee := req.Fee
		if fee == 0 {
			fee = bc.GetMinFee()
		}

		txType := core.TxTypeGeneral // General transaction

		if err := bc.SubmitTx(from, to, req.Amount, fee, req.Memo, req.Data, txType); err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		sendResp(w, http.StatusOK, true, nil)
	}
}

func ComposeAndAddBlock(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		curHeight := bc.LatestHeight + 1
		prevHash, err := utils.StringToHash(bc.LatestBlockHash)
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		// Compose block (Creating block directly from API is for testing, use empty proposer)
		// In actual operation, block should be created through consensus engine
		var emptyProposer [20]byte
		blockTimestamp := time.Now().Unix()
		blk := bc.SetBlock(prevHash, curHeight, emptyProposer, blockTimestamp)

		// Add block
		result, err := bc.AddBlock(*blk)
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		sendResp(w, http.StatusOK, result, nil)
	}
}

func GetMempoolList(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		mempoolTxs := bc.Mempool.GetTxs()

		response := formatTxsResp(mempoolTxs, bc)

		sendResp(w, http.StatusOK, response, nil)
	}
}

// send response
func sendResp(w http.ResponseWriter, statusCode int, data interface{}, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := RestResp{
		Success: err == nil,
		Data:    data,
	}

	if err != nil {
		response.Error = err.Error()
	}

	json.NewEncoder(w).Encode(response)
}

// get block response
func formatBlockResp(block *core.Block, bc *core.BlockChain) (BlockResp, error) {
	txDetails := make([]TxResp, len(block.Transactions))
	for i, tx := range block.Transactions {
		txDetails[i] = formatTxResp(tx, bc)
	}

	// Convert BFT CommitSignatures
	var commitSigs []CommitSignatureResp
	if len(block.CommitSignatures) > 0 {
		commitSigs = make([]CommitSignatureResp, len(block.CommitSignatures))
		for i, sig := range block.CommitSignatures {
			commitSigs[i] = CommitSignatureResp{
				ValidatorAddress: utils.AddressToString(sig.ValidatorAddress),
				Signature:        utils.SignatureToString(sig.Signature),
			}
		}
	}

	response := BlockResp{
		Header: BlockHeaderResp{
			Hash:       utils.HashToString(block.Header.Hash),
			PrevHash:   utils.HashToString(block.Header.PrevHash),
			Version:    block.Header.Version,
			Height:     block.Header.Height,
			MerkleRoot: utils.HashToString(block.Header.MerkleRoot),
			Timestamp:  block.Header.Timestamp,
		},
		Transactions:     txDetails,
		Proposer:         utils.AddressToString(block.Proposer),
		Signature:        utils.SignatureToString(block.Signature),
		CommitSignatures: commitSigs,
	}

	return response, nil
}

// get tx response (including fee info)
func formatTxResp(tx *core.Transaction, bc *core.BlockChain) TxResp {
	fee, _ := bc.CalculateTxFee(tx)
	return TxResp{
		ID:        utils.HashToString(tx.ID),
		Version:   tx.Version,
		Timestamp: tx.Timestamp,
		Inputs:    formatTxInputsResp(tx.Inputs),
		Outputs:   formatTxOutputsRespWithSpent(tx.ID, tx.Outputs, bc),
		Memo:      tx.Memo,
		Fee:       fee,
	}
}

// get tx input response
func formatTxInputsResp(inputs []*core.TxInput) []interface{} {
	result := make([]interface{}, len(inputs))
	for i, input := range inputs {
		result[i] = map[string]interface{}{
			"txid":        utils.HashToString(input.TxID),
			"outputIndex": input.OutputIndex,
			"signature":   utils.SignatureToString(input.Signature),
			"publicKey":   input.PublicKey,
		}
	}
	return result
}

// get tx output response
func formatTxOutputsResp(outputs []*core.TxOutput) []interface{} {
	result := make([]interface{}, len(outputs))
	for i, output := range outputs {
		result[i] = map[string]interface{}{
			"address": utils.AddressToString(output.Address),
			"amount":  output.Amount,
			"txType":  output.TxType,
		}
	}
	return result
}

// get tx output response with spent status
func formatTxOutputsRespWithSpent(txID prt.Hash, outputs []*core.TxOutput, bc *core.BlockChain) []interface{} {
	result := make([]interface{}, len(outputs))
	for i, output := range outputs {
		// Check if this output is spent by checking UTXO status
		isSpent := true // Default: assume spent
		utxo, err := bc.GetUtxoByTxIdAndIdx(txID, uint64(i))
		if err == nil {
			// UTXO exists in DB, check if it's actually spendable
			isSpent = utxo.Spent
		}

		result[i] = map[string]interface{}{
			"address": utils.AddressToString(output.Address),
			"amount":  output.Amount,
			"txType":  output.TxType,
			"spent":   isSpent,
		}
	}
	return result
}

func formatUtxoResp(utxos []*core.UTXO) []interface{} {
	result := make([]interface{}, len(utxos))
	for i, utxo := range utxos {
		result[i] = map[string]interface{}{
			"txId":        utils.HashToString(utxo.TxId),
			"outputIndex": utxo.OutputIndex,
			"amount":      utxo.TxOut.Amount,
			"address":     utils.AddressToString(utxo.TxOut.Address),
			"height":      utxo.Height,
		}
	}
	return result
}

func formatAddressTxInfos(txInfos []*core.AddressTxInfo) []interface{} {
	result := make([]interface{}, len(txInfos))
	for i, info := range txInfos {
		result[i] = map[string]interface{}{
			"txId":      utils.HashToString(info.TxID),
			"type":      info.Type,
			"amount":    info.Amount,
			"timestamp": info.Timestamp,
			"height":    info.Height,
			"spent":     info.Spent,
			"index":     info.Index,
		}
	}
	return result
}

func formatTxsResp(txs []*core.Transaction, bc *core.BlockChain) []interface{} {
	result := make([]interface{}, len(txs))
	for i, tx := range txs {
		result[i] = formatTxResp(tx, bc)
	}
	return result
}

// SubmitSignedTx submits client-signed transaction
func SubmitSignedTx(bc *core.BlockChain, p2pService *p2p.P2PService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SubmitSignedTxReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		// Convert request to core.Transaction
		tx, err := convertSignedTxReqToTx(&req)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		// Verify signature
		if err := bc.ValidateTxSignatures(tx); err != nil {
			sendResp(w, http.StatusBadRequest, nil, fmt.Errorf("signature validation failed: %w", err))
			return
		}

		// Add to mempool
		if err := bc.Mempool.NewTranaction(tx); err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		// Broadcast to P2P network
		if p2pService != nil {
			if err := p2pService.BroadcastTx(tx); err != nil {
				// Even if broadcast fails, treat as success since it's in local mempool, but log it
				fmt.Printf("[API] Failed to broadcast tx: %v\n", err)
			} else {
				fmt.Printf("[API] Broadcasted tx: %s\n", utils.HashToString(tx.ID))
			}
		}

		sendResp(w, http.StatusOK, map[string]string{
			"txId": utils.HashToString(tx.ID),
		}, nil)
	}
}

// SendTxWithWallet signs with server wallet and sends
func SendTxWithWallet(bc *core.BlockChain, wm *wallet.WalletManager, p2pService *p2p.P2PService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SendTxReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendResp(w, http.StatusBadRequest, nil, err)
			return
		}

		if wm == nil || wm.Wallet == nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("wallet not initialized"))
			return
		}

		// Get wallet accounts
		accounts := wm.Wallet.Accounts
		if req.AccountIndex < 0 || req.AccountIndex >= len(accounts) {
			sendResp(w, http.StatusBadRequest, nil, fmt.Errorf("invalid account index: %d", req.AccountIndex))
			return
		}

		account := accounts[req.AccountIndex]
		from := account.Address

		to, err := utils.StringToAddress(req.To)
		if err != nil {
			sendResp(w, http.StatusBadRequest, nil, fmt.Errorf("invalid to address: %w", err))
			return
		}

		// If fee is 0, apply minimum fee
		fee := req.Fee
		if fee == 0 {
			fee = bc.GetMinFee()
		}

		// Create signed transaction (including fee)
		tx, err := bc.CreateSignedTx(from, to, req.Amount, fee, req.Memo, req.Data, core.TxTypeGeneral, account.PrivateKey, account.PublicKey)
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("failed to create signed tx: %w", err))
			return
		}

		// Add to mempool
		if err := bc.Mempool.NewTranaction(tx); err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		// Broadcast to P2P network
		if p2pService != nil {
			if err := p2pService.BroadcastTx(tx); err != nil {
				// Even if broadcast fails, treat as success since it's in local mempool, but log it
				fmt.Printf("[API] Failed to broadcast tx: %v\n", err)
			} else {
				fmt.Printf("[API] Broadcasted tx: %s\n", utils.HashToString(tx.ID))
			}
		}

		sendResp(w, http.StatusOK, map[string]string{
			"txId": utils.HashToString(tx.ID),
			"from": utils.AddressToString(from),
			"to":   req.To,
		}, nil)
	}
}

// GetWalletAccounts gets wallet account list
func GetWalletAccounts(wm *wallet.WalletManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if wm == nil || wm.Wallet == nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("wallet not initialized"))
			return
		}

		accounts := wm.Wallet.Accounts
		result := make([]WalletAccountResp, len(accounts))
		for i, acc := range accounts {
			result[i] = WalletAccountResp{
				Index:   acc.Index,
				Address: utils.AddressToString(acc.Address),
				Path:    acc.Path,
			}
		}

		sendResp(w, http.StatusOK, result, nil)
	}
}

// CreateNewAccount creates new account
func CreateNewAccount(wm *wallet.WalletManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if wm == nil || wm.Wallet == nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("wallet not initialized"))
			return
		}

		account, err := wm.AddAccount()
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		// Save wallet
		if err := wm.SaveWallet(); err != nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("account created but failed to save: %w", err))
			return
		}

		sendResp(w, http.StatusOK, WalletAccountResp{
			Index:   account.Index,
			Address: utils.AddressToString(account.Address),
			Path:    account.Path,
		}, nil)
	}
}

// convertSignedTxReqToTx converts request to Transaction
func convertSignedTxReqToTx(req *SubmitSignedTxReq) (*core.Transaction, error) {
	// Parse signatures and public keys first (for later use)
	signatures := make([]prt.Signature, len(req.Inputs))
	publicKeys := make([][]byte, len(req.Inputs))

	for i, in := range req.Inputs {
		sig, err := utils.StringToSignature(in.Signature)
		if err != nil {
			return nil, fmt.Errorf("invalid signature in input[%d]: %w", i, err)
		}
		signatures[i] = sig

		pubKey, err := hex.DecodeString(in.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("invalid publicKey in input[%d]: %w", i, err)
		}
		publicKeys[i] = pubKey
	}

	// Create inputs WITHOUT signatures (for correct TX ID calculation)
	inputs := make([]*core.TxInput, len(req.Inputs))
	for i, in := range req.Inputs {
		txID, err := utils.StringToHash(in.TxID)
		if err != nil {
			return nil, fmt.Errorf("invalid txId in input[%d]: %w", i, err)
		}

		inputs[i] = &core.TxInput{
			TxID:        txID,
			OutputIndex: in.OutputIndex,
			PublicKey:   publicKeys[i],
			// Signature is NOT set here - will be added AFTER TX ID calculation
		}
	}

	outputs := make([]*core.TxOutput, len(req.Outputs))
	for i, out := range req.Outputs {
		addr, err := utils.StringToAddress(out.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid address in output[%d]: %w", i, err)
		}

		outputs[i] = &core.TxOutput{
			Address: addr,
			Amount:  out.Amount,
			TxType:  out.TxType,
		}
	}

	tx := &core.Transaction{
		Version:   req.Version,
		Timestamp: req.Timestamp,
		Inputs:    inputs,
		Outputs:   outputs,
		Memo:      req.Memo,
		Data:      req.Data,
	}

	// Normalize for consistent hashing (same as ValidateTxHash)
	if tx.Data == nil {
		tx.Data = []byte{}
	}
	if tx.Inputs == nil {
		tx.Inputs = []*core.TxInput{}
	}
	if tx.Outputs == nil {
		tx.Outputs = []*core.TxOutput{}
	}

	// Calculate TX ID (WITHOUT signatures - same as client did)
	tx.ID = utils.Hash(tx)

	// NOW add signatures AFTER TX ID is calculated
	for i := range tx.Inputs {
		tx.Inputs[i].Signature = signatures[i]
	}

	fmt.Printf("[DEBUG] Server Recalculated ID: %s\n", utils.HashToString(tx.ID))
	fmt.Printf("[DEBUG] TX Dump (Server): %+v\n", tx)

	return tx, nil
}

// GetWSStatus gets WebSocket connection status
func GetWSStatus(hub *api.WSHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if hub == nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("WebSocket hub not initialized"))
			return
		}

		status := map[string]interface{}{
			"connected_clients": hub.GetClientCount(),
			"endpoint":          "/ws",
		}

		sendResp(w, http.StatusOK, status, nil)
	}
}

// GetBlocks gets block list (pagination)
// query params: page (default 1), limit (default 10), order (asc/desc, default desc)
func GetBlocks(bc *core.BlockChain) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		pageStr := r.URL.Query().Get("page")
		limitStr := r.URL.Query().Get("limit")
		order := r.URL.Query().Get("order")

		page := 1
		limit := 10
		if pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}
		if order == "" {
			order = "desc"
		}

		latestHeight, err := bc.GetLatestHeight()
		if err != nil {
			sendResp(w, http.StatusInternalServerError, nil, err)
			return
		}

		// Calculate pagination
		totalBlocks := int(latestHeight) + 1
		totalPages := (totalBlocks + limit - 1) / limit
		offset := (page - 1) * limit

		var blocks []BlockResp
		if order == "desc" {
			// From latest block
			startHeight := int(latestHeight) - offset
			for i := 0; i < limit && startHeight-i >= 0; i++ {
				block, err := bc.GetBlockByHeight(uint64(startHeight - i))
				if err != nil {
					continue
				}
				blockResp, _ := formatBlockResp(block, bc)
				blocks = append(blocks, blockResp)
			}
		} else {
			// From oldest block
			startHeight := offset
			for i := 0; i < limit && startHeight+i <= int(latestHeight); i++ {
				block, err := bc.GetBlockByHeight(uint64(startHeight + i))
				if err != nil {
					continue
				}
				blockResp, _ := formatBlockResp(block, bc)
				blocks = append(blocks, blockResp)
			}
		}

		response := map[string]interface{}{
			"blocks":     blocks,
			"page":       page,
			"limit":      limit,
			"total":      totalBlocks,
			"totalPages": totalPages,
		}

		sendResp(w, http.StatusOK, response, nil)
	}
}

// GetNetworkStats gets network statistics
func GetNetworkStats(bc *core.BlockChain, hub *api.WSHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		latestHeight, _ := bc.GetLatestHeight()
		latestHash, _ := bc.GetLatestBlockHash()

		// Mempool info
		mempoolTxs := bc.Mempool.GetTxs()
		mempoolCount := len(mempoolTxs)

		// WebSocket connection count
		wsClients := 0
		if hub != nil {
			wsClients = hub.GetClientCount()
		}

		stats := map[string]interface{}{
			"chain": map[string]interface{}{
				"height":     latestHeight,
				"latestHash": latestHash,
			},
			"mempool": map[string]interface{}{
				"pendingTxCount": mempoolCount,
			},
			"network": map[string]interface{}{
				"wsConnections": wsClients,
			},
		}

		sendResp(w, http.StatusOK, stats, nil)
	}
}

// GetConsensusStatus gets consensus status
func GetConsensusStatus(cons *consensus.Consensus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cons == nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("consensus not initialized"))
			return
		}

		// Current proposer info
		var proposerAddr string
		if proposer := cons.GetCurrentProposer(); proposer != nil {
			proposerAddr = utils.AddressToString(proposer.Address)
		}

		// Configure validator list (match document format)
		validators := []map[string]interface{}{}
		votingPower := make(map[string]uint64)

		// Get active validators from ValidatorSet
		if cons.ValidatorSet != nil {
			for addrStr, validator := range cons.ValidatorSet.Validators {
				if validator.IsActive {
					validators = append(validators, map[string]interface{}{
						"address":       addrStr,
						"stakingAmount": validator.VotingPower,
						"isActive":      validator.IsActive,
					})
					votingPower[addrStr] = validator.VotingPower
				}
			}
		}

		status := map[string]interface{}{
			"state":         string(cons.State), // IDLE, PROPOSING, VOTING, COMMITTING
			"currentHeight": cons.CurrentHeight,
			"currentRound":  cons.CurrentRound,
			"proposer":      proposerAddr,
			"validators":    validators,
			"votingPower":   votingPower,
		}

		sendResp(w, http.StatusOK, status, nil)
	}
}

// GetP2PPeers gets P2P peer list
func GetP2PPeers(p2pService *p2p.P2PService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if p2pService == nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("p2p service not initialized"))
			return
		}

		peers := p2pService.GetPeers()
		peerList := make([]map[string]interface{}, 0, len(peers))

		for _, peer := range peers {
			peerInfo := map[string]interface{}{
				"id":         peer.ID,
				"address":    peer.Address,
				"state":      peer.State,
				"version":    peer.Version,
				"bestHeight": peer.BestHeight,
				"lastSeen":   peer.LastSeen.Unix(),
				"inbound":    peer.Inbound,
			}
			peerList = append(peerList, peerInfo)
		}

		response := map[string]interface{}{
			"count": len(peerList),
			"peers": peerList,
		}

		sendResp(w, http.StatusOK, response, nil)
	}
}

// GetP2PStatus P2P 노드 상태 조회
func GetP2PStatus(p2pService *p2p.P2PService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if p2pService == nil {
			sendResp(w, http.StatusInternalServerError, nil, fmt.Errorf("p2p service not initialized"))
			return
		}

		status := map[string]interface{}{
			"nodeId":    p2pService.GetNodeID(),
			"running":   p2pService.IsRunning(),
			"peerCount": p2pService.GetPeerCount(),
		}

		sendResp(w, http.StatusOK, status, nil)
	}
}
