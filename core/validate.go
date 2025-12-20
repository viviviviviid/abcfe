package core

import (
	"fmt"
	"time"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

const (
	MaxFutureBlockTime = 2 * 60 * 60 // 2 hours (in seconds)
)

// ValidatePrevHash validates previous block hash
func ValidatePrevHash(block *Block, expectedPrevHash prt.Hash) error {
	if block.Header.PrevHash != expectedPrevHash {
		return fmt.Errorf("prev hash mismatch: expected %s, got %s",
			utils.HashToString(expectedPrevHash),
			utils.HashToString(block.Header.PrevHash))
	}
	return nil
}

// ValidateMerkleRoot validates merkle root
func ValidateMerkleRoot(block *Block) error {
	calculatedRoot := calculateMerkleRoot(block.Transactions)
	if block.Header.MerkleRoot != calculatedRoot {
		return fmt.Errorf("merkle root mismatch: expected %s, got %s",
			utils.HashToString(calculatedRoot),
			utils.HashToString(block.Header.MerkleRoot))
	}
	return nil
}

// ValidateBlockHash validates block hash
func ValidateBlockHash(block *Block) error {
	// Temporarily empty hash field for hash calculation
	storedHash := block.Header.Hash
	block.Header.Hash = prt.Hash{}

	// Calculate hash using only Header (same way as SetBlock)
	calculatedHash := utils.Hash(block.Header)
	block.Header.Hash = storedHash

	if storedHash != calculatedHash {
		return fmt.Errorf("block hash mismatch: expected %s, got %s",
			utils.HashToString(calculatedHash),
			utils.HashToString(storedHash))
	}
	return nil
}

// ValidateHeightContinuity validates block height continuity
func ValidateHeightContinuity(blockHeight uint64, prevBlockHeight uint64) error {
	if blockHeight != prevBlockHeight+1 {
		return fmt.Errorf("height not continuous: expected %d, got %d",
			prevBlockHeight+1, blockHeight)
	}
	return nil
}

// ValidateTimestamp validates timestamp
func ValidateTimestamp(blockTimestamp int64, prevBlockTimestamp int64) error {
	// Must be after or equal to previous block (multiple blocks possible in same second)
	if blockTimestamp < prevBlockTimestamp {
		return fmt.Errorf("timestamp must be >= prev block: prev=%d, current=%d",
			prevBlockTimestamp, blockTimestamp)
	}

	// Must not be too far in the future compared to current time
	now := time.Now().Unix()
	if blockTimestamp > now+MaxFutureBlockTime {
		return fmt.Errorf("timestamp too far in future: max=%d, got=%d",
			now+MaxFutureBlockTime, blockTimestamp)
	}

	return nil
}

// ValidateTxCount validates transaction count limit
func ValidateTxCount(txs []*Transaction) error {
	if len(txs) > prt.MaxTxsPerBlock {
		return fmt.Errorf("too many transactions: max=%d, got=%d",
			prt.MaxTxsPerBlock, len(txs))
	}
	return nil
}

// ValidateDuplicateTx validates duplicate transactions
func ValidateDuplicateTx(txs []*Transaction) error {
	seen := make(map[prt.Hash]bool)
	for _, tx := range txs {
		if seen[tx.ID] {
			return fmt.Errorf("duplicate transaction: %s", utils.HashToString(tx.ID))
		}
		seen[tx.ID] = true
	}
	return nil
}

// ValidateDuplicateUTXO validates duplicate UTXO usage within the same block
func ValidateDuplicateUTXO(txs []*Transaction) error {
	seen := make(map[string]bool)
	for _, tx := range txs {
		for _, input := range tx.Inputs {
			key := fmt.Sprintf("%s:%d", utils.HashToString(input.TxID), input.OutputIndex)
			if seen[key] {
				return fmt.Errorf("duplicate UTXO usage in block: %s", key)
			}
			seen[key] = true
		}
	}
	return nil
}

// ValidateProposer validates proposer address (check if not empty)
func ValidateProposer(block *Block) error {
	var emptyAddr prt.Address
	if block.Proposer == emptyAddr {
		return fmt.Errorf("block proposer is empty")
	}
	return nil
}

// ValidateProposerSignature validates proposer signature (signature on block hash)
func ValidateProposerSignature(block *Block, validator ProposerValidator) error {
	if validator == nil {
		// Skip if validator is not set (e.g., solo node)
		return nil
	}

	// Check if proposer is a valid validator
	if !validator.IsValidProposer(block.Proposer, block.Header.Height) {
		return fmt.Errorf("proposer %s is not a valid validator for height %d",
			utils.AddressToString(block.Proposer), block.Header.Height)
	}

	// Verify signature
	if !validator.ValidateProposerSignature(block.Proposer, block.Header.Hash, block.Signature) {
		return fmt.Errorf("invalid proposer signature for block %s",
			utils.HashToString(block.Header.Hash))
	}

	return nil
}

// ValidateBlock validates the entire block (BlockChain method)
func (p *BlockChain) ValidateBlock(block Block, checkCommit bool) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Genesis block is validated separately
	if block.Header.Height == 0 {
		return p.validateGenesisBlock(&block)
	}

	// Get previous block (lock already acquired)
	prevBlock, err := p.getBlockByHeightNoLock(block.Header.Height - 1)
	if err != nil {
		return fmt.Errorf("failed to get prev block: %w", err)
	}

	// 1. Validate previous hash
	if err := ValidatePrevHash(&block, prevBlock.Header.Hash); err != nil {
		return err
	}

	// 2. Validate merkle root
	if err := ValidateMerkleRoot(&block); err != nil {
		return err
	}

	// 3. Validate block hash
	if err := ValidateBlockHash(&block); err != nil {
		return err
	}

	// 4. Validate height continuity
	if err := ValidateHeightContinuity(block.Header.Height, prevBlock.Header.Height); err != nil {
		return err
	}

	// 5. Validate timestamp
	if err := ValidateTimestamp(block.Header.Timestamp, prevBlock.Header.Timestamp); err != nil {
		return err
	}

	// 6. Validate proposer address
	if err := ValidateProposer(&block); err != nil {
		return err
	}

	// 7. Validate proposer signature (PoA)
	if err := ValidateProposerSignature(&block, p.proposerValidator); err != nil {
		return err
	}

	// 8. Validate BFT commit signatures (2/3+ majority) - conditionally
	if checkCommit && p.proposerValidator != nil {
		if err := p.proposerValidator.ValidateCommitSignatures(block.Header.Hash, block.CommitSignatures); err != nil {
			return fmt.Errorf("BFT consensus validation failed: %w", err)
		}
	}

	// 9. Validate transaction count
	if err := ValidateTxCount(block.Transactions); err != nil {
		return err
	}

	// 10. Validate duplicate transactions
	if err := ValidateDuplicateTx(block.Transactions); err != nil {
		return err
	}

	// 11. Validate duplicate UTXO usage
	if err := ValidateDuplicateUTXO(block.Transactions); err != nil {
		return err
	}

	// 12. Validate each transaction
	for _, tx := range block.Transactions {
		if err := p.ValidateTransaction(tx); err != nil {
			return fmt.Errorf("invalid transaction %s: %w", utils.HashToString(tx.ID), err)
		}
	}

	return nil
}

// validateGenesisBlock validates genesis block
func (p *BlockChain) validateGenesisBlock(block *Block) error {
	// Height must be 0
	if block.Header.Height != 0 {
		return fmt.Errorf("genesis block height must be 0")
	}

	// Previous hash must be all zeros
	var zeroPrevHash prt.Hash
	if block.Header.PrevHash != zeroPrevHash {
		return fmt.Errorf("genesis block prev hash must be zero")
	}

	// Validate merkle root
	if err := ValidateMerkleRoot(block); err != nil {
		return err
	}

	// Validate block hash
	if err := ValidateBlockHash(block); err != nil {
		return err
	}

	return nil
}

// ValidateTransaction validates transaction
func (p *BlockChain) ValidateTransaction(tx *Transaction) error {
	// Validate transaction hash
	if err := ValidateTxHash(tx); err != nil {
		return err
	}

	// Validate NetworkID (prevent replay)
	if tx.NetworkID != p.cfg.Common.NetworkID {
		return fmt.Errorf("network ID mismatch: tx=%s, node=%s", tx.NetworkID, p.cfg.Common.NetworkID)
	}

	// Validate memo size
	maxMemoSize := p.GetMaxMemoSize()
	if maxMemoSize > 0 && uint64(len(tx.Memo)) > maxMemoSize {
		return fmt.Errorf("memo too long: %d bytes > max %d bytes", len(tx.Memo), maxMemoSize)
	}

	// Validate data size
	maxDataSize := p.GetMaxDataSize()
	if maxDataSize > 0 && uint64(len(tx.Data)) > maxDataSize {
		return fmt.Errorf("data too long: %d bytes > max %d bytes", len(tx.Data), maxDataSize)
	}

	// Coinbase transaction (no inputs) - validated separately
	if len(tx.Inputs) == 0 {
		return p.ValidateCoinbaseTx(tx)
	}

	// Validate Input/Output balance
	var inputSum, outputSum uint64

	for _, input := range tx.Inputs {
		// UTXO 존재 여부 확인
		utxoKey := utils.GetUtxoKey(input.TxID, int(input.OutputIndex))
		utxoBytes, err := p.db.Get(utxoKey, nil)
		if err != nil {
			return fmt.Errorf("UTXO not found: %s:%d", utils.HashToString(input.TxID), input.OutputIndex)
		}

		var utxo UTXO
		if err := utils.DeserializeData(utxoBytes, &utxo, utils.SerializationFormatGob); err != nil {
			return fmt.Errorf("failed to deserialize UTXO: %w", err)
		}

		// 이미 사용된 UTXO인지 확인
		if utxo.Spent {
			return fmt.Errorf("UTXO already spent: %s:%d", utils.HashToString(input.TxID), input.OutputIndex)
		}

		inputSum += utxo.TxOut.Amount
	}

	for _, output := range tx.Outputs {
		outputSum += output.Amount
	}

	// Validate input >= output
	if inputSum < outputSum {
		return fmt.Errorf("insufficient input: input=%d, output=%d", inputSum, outputSum)
	}

	// Calculate implicit fee and validate minimum fee
	implicitFee := inputSum - outputSum
	minFee := p.GetMinFee()
	if implicitFee < minFee {
		return fmt.Errorf("fee too low: got %d, minimum required %d", implicitFee, minFee)
	}

	// Verify signature
	if err := p.ValidateAllTxSignatures(tx); err != nil {
		return fmt.Errorf("signature validation failed: %w", err)
	}

	return nil
}

// ValidateCoinbaseTx validates Coinbase transaction
func (p *BlockChain) ValidateCoinbaseTx(tx *Transaction) error {
	// Coinbase TX must have no inputs
	if len(tx.Inputs) != 0 {
		return fmt.Errorf("coinbase tx must have no inputs")
	}

	// Must have at least one Output
	if len(tx.Outputs) == 0 {
		return fmt.Errorf("coinbase tx must have at least one output")
	}

	// Total output amount must be positive
	var totalOutput uint64
	for _, output := range tx.Outputs {
		totalOutput += output.Amount
	}

	if totalOutput == 0 {
		return fmt.Errorf("coinbase tx output amount must be positive")
	}

	return nil
}

// CalculateTxFee calculates implicit fee of transaction
func (p *BlockChain) CalculateTxFee(tx *Transaction) (uint64, error) {
	// Coinbase TX has no fee
	if len(tx.Inputs) == 0 {
		return 0, nil
	}

	var inputSum, outputSum uint64

	for _, input := range tx.Inputs {
		utxo, err := p.GetUtxoByTxIdAndIdx(input.TxID, input.OutputIndex)
		if err != nil {
			return 0, fmt.Errorf("failed to get UTXO: %w", err)
		}
		inputSum += utxo.TxOut.Amount
	}

	for _, output := range tx.Outputs {
		outputSum += output.Amount
	}

	if inputSum < outputSum {
		return 0, fmt.Errorf("invalid tx: inputSum < outputSum")
	}

	return inputSum - outputSum, nil
}

// ValidateTxHash validates transaction hash
// TX ID is calculated before signing, so hash calculation for verification must also exclude signature
func ValidateTxHash(tx *Transaction) error {
	storedHash := tx.ID
	tx.ID = prt.Hash{}

	// Resolve issue where empty slice becomes nil after GOB deserialization
	// Since empty slice was used during TX creation, normalize nil to empty slice during verification
	savedData := tx.Data
	savedInputs := tx.Inputs
	if tx.Data == nil {
		tx.Data = []byte{}
	}
	if tx.Inputs == nil {
		tx.Inputs = []*TxInput{}
	}

	// Backup and remove signatures temporarily (TX ID is calculated before signing)
	savedSignatures := make([]prt.Signature, len(tx.Inputs))
	for i, input := range tx.Inputs {
		savedSignatures[i] = input.Signature
		input.Signature = prt.Signature{}
	}

	calculatedHash := utils.Hash(tx)

	// Restore signatures and data
	for i, input := range tx.Inputs {
		input.Signature = savedSignatures[i]
	}
	tx.ID = storedHash
	tx.Data = savedData
	tx.Inputs = savedInputs

	if storedHash != calculatedHash {
		return fmt.Errorf("tx hash mismatch: expected %s, got %s",
			utils.HashToString(calculatedHash),
			utils.HashToString(storedHash))
	}
	return nil
}

// ValidateTxInputSignature validates transaction input signature
func ValidateTxInputSignature(tx *Transaction, input *TxInput, utxo *UTXO) error {
	// Check if public key matches UTXO owner's address
	if len(input.PublicKey) == 0 {
		return fmt.Errorf("public key is empty")
	}

	// Derive address from public key
	publicKey, err := crypto.BytesToPublicKey(input.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	derivedAddress, err := crypto.PublicKeyToAddress(publicKey)
	if err != nil {
		return fmt.Errorf("failed to derive address: %w", err)
	}

	// Check if address matches UTXO owner
	if derivedAddress != utxo.TxOut.Address {
		return fmt.Errorf("public key does not match UTXO owner")
	}

	// Create data for signature verification (Use stored TX ID)
	// Signature is generating based on TX ID, so do not recalculate hash
	txHashBytes := utils.HashToBytes(tx.ID)

	// Verify signature
	valid := crypto.VerifySignature(publicKey, txHashBytes, input.Signature)
	if !valid {
		fmt.Printf("[DEBUG-VAL] Invalid signature for input %s:%d\n", utils.HashToString(input.TxID), input.OutputIndex)
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// ValidateAllTxSignatures validates all input signatures of transaction
func (p *BlockChain) ValidateAllTxSignatures(tx *Transaction) error {
	// Skip signature verification for genesis transaction
	if len(tx.Inputs) == 0 {
		return nil
	}

	for i, input := range tx.Inputs {
		// Get UTXO
		utxoKey := utils.GetUtxoKey(input.TxID, int(input.OutputIndex))
		utxoBytes, err := p.db.Get(utxoKey, nil)
		if err != nil {
			return fmt.Errorf("UTXO not found for input %d: %w", i, err)
		}

		var utxo UTXO
		if err := utils.DeserializeData(utxoBytes, &utxo, utils.SerializationFormatGob); err != nil {
			return fmt.Errorf("failed to deserialize UTXO: %w", err)
		}

		// Verify signature
		if err := ValidateTxInputSignature(tx, input, &utxo); err != nil {
			return fmt.Errorf("signature validation failed for input %d: %w", i, err)
		}
	}

	return nil
}
