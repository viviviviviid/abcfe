package core

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

// Transaction struct
type Transaction struct {
	Version   string   `json:"version"`   // Transaction version
	ID        prt.Hash `json:"id"`        // Transaction ID (hash)
	Timestamp int64    `json:"timestamp"` // Transaction creation time

	Inputs  []*TxInput  `json:"inputs"`  // Transaction inputs
	Outputs []*TxOutput `json:"outputs"` // Transaction outputs // Must be multiple outputs for change

	Memo string `json:"memo"` // Transaction memo (replaces inputData)
	Data []byte `json:"data"` // Arbitrary data (smart contract calls, etc.)
	// Status // TODO
	// Signature // TODO
}

type TxInput struct {
	TxID        prt.Hash      `json:"txId"`        // Referenced transaction ID
	OutputIndex uint64        `json:"outputIndex"` // Referenced output index
	Signature   prt.Signature `json:"signature"`   // Signature
	PublicKey   []byte        `json:"publicKey"`   // Public key
	// Sequence    uint64        `json:"sequence"`    // Sequence number (RBF support)
}

type TxOutput struct {
	Address prt.Address `json:"address"` // Receiver address
	Amount  uint64      `json:"amount"`  // Amount (changed to uint64)
	TxType  uint8       `json:"txType"`  // Script type (General/Staking/Etc)
}

// Tx Input and Output pair
type TxIOPair struct {
	TxIns  []*TxInput  `json:"txIns"`
	TxOuts []*TxOutput `json:"txOuts"`
}

func (p *BlockChain) SetTransferTx(from prt.Address, to prt.Address, amount uint64, fee uint64, memo string, data []byte, txType uint8) (*Transaction, error) {
	utxos, err := p.GetUtxoList(from, true)
	if err != nil {
		return nil, err
	}

	// Verify total required amount including fee
	requiredAmount := amount + fee
	if p.CalBalanceUtxo(utxos) < requiredAmount {
		return &Transaction{}, fmt.Errorf("not enough balance: need %d (amount) + %d (fee) = %d", amount, fee, requiredAmount)
	}

	txInAndOut, err := p.setTxIOPair(utxos, from, to, amount, fee, txType)
	if err != nil {
		return &Transaction{}, err
	}

	// Resolve nil becoming empty slice after GOB deserialization
	// Normalize nil to empty slice when creating transaction to maintain hash consistency
	normalizedData := data
	if normalizedData == nil {
		normalizedData = []byte{}
	}
	normalizedInputs := txInAndOut.TxIns
	if normalizedInputs == nil {
		normalizedInputs = []*TxInput{}
	}

	tx := Transaction{
		Version:   p.cfg.Version.Transaction,
		Timestamp: time.Now().Unix(),
		Inputs:    normalizedInputs,
		Outputs:   txInAndOut.TxOuts,
		Memo:      memo,
		Data:      normalizedData,
	}

	txHash := utils.Hash(tx)
	tx.ID = txHash

	return &tx, nil
}

// Configure tx input and output (including fee)
func (p *BlockChain) setTxIOPair(utxos []*UTXO, from prt.Address, to prt.Address, amount uint64, fee uint64, txType uint8) (*TxIOPair, error) {
	var txInAndOut TxIOPair
	var total uint64

	// Total required amount including fee
	requiredAmount := amount + fee

	// set tx in
	for _, utxo := range utxos {
		// Stop if amount including fee is satisfied
		if total >= requiredAmount {
			break
		}
		// ! Pubkey needs pre-processing, signature needs post-processing
		txIn := p.setTxInput(utxo.TxId, utxo.OutputIndex, prt.Signature{}, nil)
		txInAndOut.TxIns = append(txInAndOut.TxIns, txIn)

		total += utxo.TxOut.Amount
	}

	// Error if not enough funds even after using all UTXOs
	if total < requiredAmount {
		return nil, fmt.Errorf("not enough balance: required %d (amount %d + fee %d), have %d", requiredAmount, amount, fee, total)
	}

	// set tx out - Amount to receiver
	txOut := p.setTxOutput(to, amount, txType)
	txInAndOut.TxOuts = append(txInAndOut.TxOuts, txOut)

	// Return change if needed (Fee is not included in Output = Implicit fee)
	if total > requiredAmount {
		changeOut := total - requiredAmount // total - amount - fee
		txOut := p.setTxOutput(from, changeOut, txType)
		txInAndOut.TxOuts = append(txInAndOut.TxOuts, txOut)
	}

	return &txInAndOut, nil
}

func (p *BlockChain) setTxInput(txOutID prt.Hash, txOutIdx uint64, sig prt.Signature, pubKey []byte) *TxInput {
	// Resolve nil becoming empty slice after GOB deserialization
	// Normalize nil to empty slice to maintain hash consistency
	normalizedPubKey := pubKey
	if normalizedPubKey == nil {
		normalizedPubKey = []byte{}
	}

	txIn := &TxInput{
		TxID:        txOutID,
		OutputIndex: txOutIdx,
		Signature:   sig,
		PublicKey:   normalizedPubKey,
	}

	return txIn
}

func (p *BlockChain) setTxOutput(toAddr prt.Address, amount uint64, txType uint8) *TxOutput {
	txOut := &TxOutput{
		Address: toAddr,
		Amount:  amount,
		TxType:  txType,
	}

	return txOut
}

func (p *BlockChain) GetTx(txId prt.Hash) (*Transaction, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := utils.GetTxHashKey(txId)
	txBytes, err := p.db.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get block hash from db: %w", err)
	}

	// tx data bytes -> tx data deserialization
	var tx Transaction
	if err := utils.DeserializeData(txBytes, &tx, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize block data: %w", err)
	}

	return &tx, nil
}

func (p *BlockChain) GetBlockHashByTxId(txId prt.Hash) (prt.Hash, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// block height -> block hash bytes
	key := utils.GetTxBlockHashKey(txId)
	blkHashBytes, err := p.db.Get(key, nil)
	if err != nil {
		return prt.Hash{}, fmt.Errorf("failed to get block hash from db: %w", err)
	}

	// block hash bytes -> block hash string
	var blkHash prt.Hash
	blkHash = utils.BytesToHash(blkHashBytes)

	return blkHash, nil
}

// TODO
// func (p *BlockChain) GetTxStatus(height uint64, id prt.Hash) {
// prt.PrefixTxStatus
// }

func (p *BlockChain) GetInputTx(txId prt.Hash) (*[]TxInput, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := utils.GetTxInputKey(txId, prt.WholeTxIdx)
	txBytes, err := p.db.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx input data from db: %w", err)
	}

	// tx data bytes -> tx data deserialization
	var txInputs []TxInput
	if err := utils.DeserializeData(txBytes, &txInputs, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize tx input data: %w", err)
	}

	return &txInputs, nil
}

func (p *BlockChain) GetOutputTx(txId prt.Hash) (*[]TxOutput, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := utils.GetTxOutputKey(txId, prt.WholeTxIdx)
	txBytes, err := p.db.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx output data from db: %w", err)
	}

	// tx data bytes -> tx data deserialization
	var txOutputs []TxOutput
	if err := utils.DeserializeData(txBytes, &txOutputs, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize tx output data: %w", err)
	}

	return &txOutputs, nil
}

func (p *BlockChain) GetInputTxByIdx(txId prt.Hash, idx int) (*TxInput, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := utils.GetTxInputKey(txId, idx)
	txBytes, err := p.db.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx input data from db: %w", err)
	}

	// tx data bytes -> tx data deserialization
	var txInput TxInput
	if err := utils.DeserializeData(txBytes, &txInput, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize tx input data: %w", err)
	}

	return &txInput, nil
}

func (p *BlockChain) GetOutputTxByIdx(txId prt.Hash, idx int) (*TxOutput, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := utils.GetTxOutputKey(txId, idx)
	txBytes, err := p.db.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx output data from db: %w", err)
	}

	// tx data bytes -> tx data deserialization
	var txOutput TxOutput
	if err := utils.DeserializeData(txBytes, &txOutput, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize tx output data: %w", err)
	}

	return &txOutput, nil
}

func (p *BlockChain) SubmitTx(from, to prt.Address, amount uint64, fee uint64, memo string, data []byte, txType uint8) error {
	utxoList, err := p.GetUtxoList(from, true)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	// utxo 기반으로 밸런스 체크 (수수료 포함)
	requiredAmount := amount + fee
	balance := p.CalBalanceUtxo(utxoList)
	if balance < requiredAmount {
		return fmt.Errorf("not enough balance: need %d (amount %d + fee %d), have %d", requiredAmount, amount, fee, balance)
	}

	// set transaction (수수료 포함)
	tx, err := p.SetTransferTx(from, to, amount, fee, memo, data, txType)
	if err != nil {
		return fmt.Errorf("failed to set transaction: %w", err)
	}

	// mempool에 저장
	if err := p.Mempool.NewTranaction(tx); err != nil {
		return fmt.Errorf("failed to save transaction in mempool: %w", err)
	}

	return nil
}

func calculateMerkleRoot(txs []*Transaction) prt.Hash {
	if len(txs) == 0 {
		return prt.Hash{} // Return empty hash
	}

	// Use ID (hash) of each transaction
	// TX.ID is already transaction hash, so use directly
	hashes := make([]prt.Hash, len(txs))
	for i, tx := range txs {
		hashes[i] = tx.ID
	}

	// Calculate Merkle tree // Recursive call
	return buildMerkleTree(hashes)
}

func buildMerkleTree(hashes []prt.Hash) prt.Hash {
	if len(hashes) == 1 {
		return hashes[0] // Reached root
	}

	// Make even count => duplicate last hash if odd
	if len(hashes)%2 != 0 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// Calculate next level
	nextLevel := make([]prt.Hash, len(hashes)/2)
	for i := 0; i < len(hashes); i += 2 {
		combined := append(hashes[i][:], hashes[i+1][:]...)
		nextLevel[i/2] = sha256.Sum256(combined)
	}

	return buildMerkleTree(nextLevel)
}

// CreateSignedTx creates signed transaction (includes fee)
func (p *BlockChain) CreateSignedTx(from, to prt.Address, amount uint64, fee uint64, memo string, data []byte, txType uint8, privateKeyBytes, publicKeyBytes []byte) (*Transaction, error) {
	utxos, err := p.GetUtxoList(from, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get UTXO list: %w", err)
	}

	// Debug: Check UTXO list
	logger.Debug("[CreateSignedTx] from: ", utils.AddressToString(from))
	logger.Debug("[CreateSignedTx] UTXO count: ", len(utxos))
	for i, utxo := range utxos {
		logger.Debug("[CreateSignedTx] UTXO[", i, "]: txId=", utils.HashToString(utxo.TxId)[:16], " amount=", utxo.TxOut.Amount, " spent=", utxo.Spent)
	}

	// Total required amount including fee
	requiredAmount := amount + fee
	balance := p.CalBalanceUtxo(utxos)
	logger.Debug("[CreateSignedTx] balance: ", balance, " requiredAmount: ", requiredAmount)
	if balance < requiredAmount {
		return nil, fmt.Errorf("not enough balance: have %d, need %d (amount %d + fee %d)", balance, requiredAmount, amount, fee)
	}

	// Configure TX Input/Output
	var txIns []*TxInput
	var total uint64

	// Resolve nil becoming empty slice after GOB deserialization
	// Normalize nil to empty slice to maintain hash consistency
	normalizedPublicKey := publicKeyBytes
	if normalizedPublicKey == nil {
		normalizedPublicKey = []byte{}
	}

	for _, utxo := range utxos {
		// 수수료 포함한 금액을 충족하면 중단
		if total >= requiredAmount {
			break
		}
		txIn := &TxInput{
			TxID:        utxo.TxId,
			OutputIndex: utxo.OutputIndex,
			PublicKey:   normalizedPublicKey,
			// Signature set later
		}
		txIns = append(txIns, txIn)
		total += utxo.TxOut.Amount
	}

	if total < requiredAmount {
		return nil, fmt.Errorf("not enough balance after collecting UTXOs: have %d, need %d", total, requiredAmount)
	}

	// Configure TX Output - Amount to receiver
	var txOuts []*TxOutput
	txOuts = append(txOuts, &TxOutput{
		Address: to,
		Amount:  amount,
		TxType:  txType,
	})

	// Change (Fee is not included in Output = Implicit fee)
	if total > requiredAmount {
		txOuts = append(txOuts, &TxOutput{
			Address: from,
			Amount:  total - requiredAmount, // total - amount - fee
			TxType:  txType,
		})
	}

	// Resolve nil becoming empty slice after GOB deserialization
	// Normalize nil to empty slice when creating transaction to maintain hash consistency
	normalizedData := data
	if normalizedData == nil {
		normalizedData = []byte{}
	}
	normalizedInputs := txIns
	if normalizedInputs == nil {
		normalizedInputs = []*TxInput{}
	}

	// Create transaction
	tx := &Transaction{
		Version:   p.cfg.Version.Transaction,
		Timestamp: time.Now().Unix(),
		Inputs:    normalizedInputs,
		Outputs:   txOuts,
		Memo:      memo,
		Data:      normalizedData,
	}

	// Calculate TX ID (before signing)
	tx.ID = utils.Hash(tx)

	// Sign each input
	privateKey, err := crypto.BytesToPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	txHashBytes := utils.HashToBytes(tx.ID)
	for i := range tx.Inputs {
		sig, err := crypto.SignData(privateKey, txHashBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to sign input[%d]: %w", i, err)
		}
		tx.Inputs[i].Signature = sig
	}

	return tx, nil
}

// ValidateTxSignatures validates all input signatures of transaction
func (p *BlockChain) ValidateTxSignatures(tx *Transaction) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}

	// Coinbase TX does not need signature verification
	if len(tx.Inputs) == 0 {
		return nil
	}

	// Calculate TX hash (for signature verification)
	// Signature is based on TX ID
	txHashBytes := utils.HashToBytes(tx.ID)

	for i, input := range tx.Inputs {
		// Error if public key is empty
		if len(input.PublicKey) == 0 {
			return fmt.Errorf("input[%d]: public key is empty", i)
		}

		// Verify signature
		valid, err := crypto.VerifySignatureWithBytes(input.PublicKey, txHashBytes, input.Signature)
		if err != nil {
			return fmt.Errorf("input[%d]: signature verification error: %w", i, err)
		}
		if !valid {
			return fmt.Errorf("input[%d]: invalid signature", i)
		}

		// Verify UTXO ownership (Public key -> Address -> Check UTXO owner)
		pubKey, err := crypto.BytesToPublicKey(input.PublicKey)
		if err != nil {
			return fmt.Errorf("input[%d]: failed to parse public key: %w", i, err)
		}

		signerAddr, err := crypto.PublicKeyToAddress(pubKey)
		if err != nil {
			return fmt.Errorf("input[%d]: failed to derive address from public key: %w", i, err)
		}

		// Get referenced UTXO
		utxo, err := p.GetUtxoByTxIdAndIdx(input.TxID, input.OutputIndex)
		if err != nil {
			return fmt.Errorf("input[%d]: failed to get referenced UTXO: %w", i, err)
		}

		// Compare UTXO owner and signer address
		if signerAddr != utxo.TxOut.Address {
			return fmt.Errorf("input[%d]: signer address does not match UTXO owner", i)
		}
	}

	return nil
}
