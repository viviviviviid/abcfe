package core

import (
	"fmt"

	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

type Block struct {
	Header       BlockHeader    `json:"header"`       // Block header
	Transactions []*Transaction `json:"transactions"` // Transaction list

	// PoA consensus info
	Proposer  prt.Address   `json:"proposer"`  // Block proposer address
	Signature prt.Signature `json:"signature"` // Proposer's block signature

	// BFT consensus info (2/3 vote evidence)
	CommitSignatures []CommitSignature `json:"commitSignatures,omitempty"` // Validators' commit signatures
}

// CommitSignature validator's commit signature info
type CommitSignature struct {
	ValidatorAddress prt.Address   `json:"validatorAddress"` // Validator address
	Signature        prt.Signature `json:"signature"`        // Signature for block hash
	Timestamp        int64         `json:"timestamp"`        // Signature time
}

type BlockHeader struct {
	Hash       prt.Hash `json:"hash"`       // Block hash
	PrevHash   prt.Hash `json:"prevHash"`   // Previous block hash
	Version    string   `json:"version"`    // Blockchain protocol version
	Height     uint64   `json:"height"`     // Block height (changed to uint64)
	MerkleRoot prt.Hash `json:"merkleRoot"` // Transaction Merkle root
	Timestamp  int64    `json:"timestamp"`  // Block creation time (Unix timestamp)
	// StateRoot  Hash   `json:"stateRoot"`  // State Merkle root (UTXO or account state)
}

func (p *BlockChain) SetBlock(prevHash prt.Hash, height uint64, proposer prt.Address, blockTimestamp int64) *Block {
	// Get transactions from mempool
	candidateTxs := p.Mempool.GetTxs()

	// logger.Info("[SetBlock] Mempool has ", len(candidateTxs), " candidate TXs")

	// Filter only valid transactions (including signature verification)
	var validTxs []*Transaction
	var invalidTxIds []prt.Hash
	var totalFees uint64 = 0

	for _, tx := range candidateTxs {
		if err := p.ValidateTransaction(tx); err != nil {
			// Invalid transactions will be removed from mempool
			logger.Warn("[SetBlock] TX validation failed: ", utils.HashToString(tx.ID)[:16], " error: ", err)
			invalidTxIds = append(invalidTxIds, tx.ID)
			continue
		}

		// Calculate fee
		fee, err := p.CalculateTxFee(tx)
		if err != nil {
			logger.Warn("[SetBlock] Failed to calculate fee for TX: ", utils.HashToString(tx.ID)[:16], " error: ", err)
			invalidTxIds = append(invalidTxIds, tx.ID)
			continue
		}

		totalFees += fee
		logger.Info("[SetBlock] TX validated: ", utils.HashToString(tx.ID)[:16], " fee: ", fee)
		validTxs = append(validTxs, tx)
	}

	// Remove invalid transactions from mempool
	for _, txId := range invalidTxIds {
		logger.Warn("[SetBlock] Removing invalid TX from mempool: ", utils.HashToString(txId)[:16])
		p.Mempool.DelTx(txId)
	}

	// Create Coinbase TX (Block reward + fees) - Use block timestamp
	var emptyProposer prt.Address
	if proposer != emptyProposer {
		coinbaseTx := p.createCoinbaseTx(proposer, height, totalFees, blockTimestamp)
		// Add Coinbase TX to the front
		validTxs = append([]*Transaction{coinbaseTx}, validTxs...)
		logger.Info("[SetBlock] Coinbase TX created: reward=", p.GetBlockReward(), " fees=", totalFees, " total=", p.GetBlockReward()+totalFees)
	}

	txs := validTxs

	// Calculate Merkle root
	merkleRoot := calculateMerkleRoot(txs)

	// Configure header
	blkHeader := &BlockHeader{
		Version:    p.cfg.Version.Protocol,
		Height:     height,
		PrevHash:   prevHash,
		Timestamp:  blockTimestamp,
		MerkleRoot: merkleRoot,
	}

	blk := &Block{
		Header:       *blkHeader,
		Transactions: txs,
		Proposer:     proposer,
	}

	// Block hash is calculated only with Header (Header already includes MerkleRoot so transaction integrity is guaranteed)
	blkHash := utils.Hash(blk.Header)
	blk.Header.Hash = blkHash

	return blk
}

// createCoinbaseTx creates Coinbase transaction (Pay block reward + fees to proposer)
func (p *BlockChain) createCoinbaseTx(proposer prt.Address, height uint64, totalFees uint64, blockTimestamp int64) *Transaction {
	blockReward := p.GetBlockReward()
	totalReward := blockReward + totalFees

	coinbaseTx := &Transaction{
		Version:   p.cfg.Version.Transaction,
		Timestamp: blockTimestamp, // Use block timestamp (same value across all nodes)
		Inputs:    []*TxInput{},   // Coinbase TX has no Inputs
		Outputs: []*TxOutput{
			{
				Address: proposer,
				Amount:  totalReward,
				TxType:  TxTypeCoinbase,
			},
		},
		Memo: fmt.Sprintf("Block %d Coinbase: reward=%d, fees=%d", height, blockReward, totalFees),
		Data: []byte{}, // Empty data (explicitly set to distinguish from nil)
	}

	// Calculate TX ID
	coinbaseTx.ID = utils.Hash(coinbaseTx)

	return coinbaseTx
}

// SignBlock adds signature to block
func (blk *Block) SignBlock(signature prt.Signature) {
	blk.Signature = signature
}

// TODO: Modularize if content grows larger later
// func (p *BlockChain) setBlockHeader(height uint64, prevHash, merkleRoot prt.Hash) *BlockHeader {
// 	blkHeader := &BlockHeader{
// 		Version:    p.cfg.Version.Protocol,
// 		Height:     height,
// 		PrevHash:   prevHash,
// 		Timestamp:  time.Now().Unix(),
// 		MerkleRoot: merkleRoot,
// 	}
// 	return blkHeader
// }

// add block to chain
func (p *BlockChain) AddBlock(blk Block) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// db batch process ready
	batch := new(leveldb.Batch)

	// block data save
	err := p.saveBlockData(batch, blk)
	if err != nil {
		return false, fmt.Errorf("failed to save block into db: %w", err)
	}

	// tx data save
	err = p.saveTxData(batch, blk)
	if err != nil {
		return false, fmt.Errorf("failed to save tx into db: %w", err)
	}

	// utxo data save
	err = p.UpdateUtxo(batch, blk)
	if err != nil {
		return false, fmt.Errorf("failed to save utxo into db: %w", err)
	}

	// batch excute
	if err := p.db.Write(batch, nil); err != nil {
		return false, fmt.Errorf("failed to write batch: %w", err)
	}

	// mempool update
	for _, tx := range blk.Transactions {
		p.Mempool.DelTx(tx.ID)
	}

	// chain status update
	if blk.Header.Height > p.LatestHeight || blk.Header.Height == 0 {
		if err := p.UpdateChainState(blk.Header.Height, utils.HashToString(blk.Header.Hash)); err != nil {
			return false, fmt.Errorf("failed to update chain status: %w", err)
		}
	}

	return true, nil
}

func (p *BlockChain) saveBlockData(batch *leveldb.Batch, blk Block) error {
	// block serialization
	blkBytes, err := utils.SerializeData(blk, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to block serialization.")
	}

	// block hash - block data mapping
	blkHashKey := utils.GetBlockHashKey(blk.Header.Hash)
	batch.Put(blkHashKey, blkBytes)

	// block height - block hash mapping
	heightKey := utils.GetBlockHeightKey(blk.Header.Height)
	batch.Put(heightKey, blk.Header.Hash[:])

	return nil
}

func (p *BlockChain) saveTxData(batch *leveldb.Batch, blk Block) error {
	for _, tx := range blk.Transactions {
		txBytes, err := utils.SerializeData(tx, utils.SerializationFormatGob)
		if err != nil {
			return fmt.Errorf("failed to tx serialization.")
		}

		// tx hash -> tx data
		txHashKey := utils.GetTxHashKey(tx.ID)
		batch.Put(txHashKey, txBytes)

		// tx hash -> block hash
		txBlkHashKey := utils.GetTxBlockHashKey(tx.ID)
		batch.Put(txBlkHashKey, utils.HashToBytes(blk.Header.Hash))

		// TODO: tx hash -> tx status
		// txStatusKey := utils.GetTxStatusKey()

		// TODO: 'whole' might be deprecated later
		// whole input tx
		txInputsKey := utils.GetTxInputKey(tx.ID, prt.WholeTxIdx)
		inputsBytes, err := utils.SerializeData(tx.Inputs, utils.SerializationFormatGob)
		if err != nil {
			return fmt.Errorf("failed to tx inputs serialization.")
		}
		batch.Put(txInputsKey, inputsBytes)

		// whole output tx
		txOutputKey := utils.GetTxOutputKey(tx.ID, prt.WholeTxIdx)
		outputsBytes, err := utils.SerializeData(tx.Outputs, utils.SerializationFormatGob)
		if err != nil {
			return fmt.Errorf("failed to tx outputs serialization.")
		}
		batch.Put(txOutputKey, outputsBytes)

		// tx input/output - tx data
		for idx, input := range tx.Inputs {
			txInputKey := utils.GetTxInputKey(tx.ID, idx)
			inputBytes, err := utils.SerializeData(input, utils.SerializationFormatGob)
			if err != nil {
				return fmt.Errorf("failed to tx input serialization.")
			}
			batch.Put(txInputKey, inputBytes)
		}

		for idx, output := range tx.Outputs {
			txOutputKey := utils.GetTxOutputKey(tx.ID, idx)
			outputBytes, err := utils.SerializeData(output, utils.SerializationFormatGob)
			if err != nil {
				return fmt.Errorf("failed to tx output serialization.")
			}
			batch.Put(txOutputKey, outputBytes)
		}
	}

	return nil
}

// block height -> block data
func (p *BlockChain) GetBlockByHeight(height uint64) (*Block, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.getBlockByHeightNoLock(height)
}

// getBlockByHeightNoLock gets block without lock (internal use)
func (p *BlockChain) getBlockByHeightNoLock(height uint64) (*Block, error) {
	// block height -> block hash bytes
	heightKey := utils.GetBlockHeightKey(height)
	blkHashBytes, err := p.db.Get(heightKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get block hash from db: %w", err)
	}

	// block hash bytes -> block hash string
	var blkHash prt.Hash
	copy(blkHash[:], blkHashBytes)

	// block hash string -> block data bytes
	blkKey := utils.GetBlockHashKey(blkHash)
	blkDataBytes, err := p.db.Get(blkKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get block data from db: %w", err)
	}

	// block data bytes -> block data deserialization
	var block Block
	if err := utils.DeserializeData(blkDataBytes, &block, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize block data: %w", err)
	}

	return &block, nil
}

// block hash -> block data
func (p *BlockChain) GetBlockByHash(hash prt.Hash) (*Block, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// block hash -> block hash bytes
	blkHashKey := utils.GetBlockHashKey(hash)
	blkDataBytes, err := p.db.Get(blkHashKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get block hash from db: %w", err)
	}

	// block data bytes -> block data deserialization
	var block Block
	if err := utils.DeserializeData(blkDataBytes, &block, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize block data: %w", err)
	}

	return &block, nil
}

// ValidateBlock moved to validate.go

// blockToJSON converts block to JSON format
func blockToJSON(block interface{}) ([]byte, error) {
	return utils.SerializeData(block, utils.SerializationFormatJSON)
}

// jsonToBlock converts JSON format to block
func jsonToBlock(data []byte, block interface{}) error {
	return utils.DeserializeData(data, block, utils.SerializationFormatJSON)
}
