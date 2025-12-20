package core

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/abcfe/abcfe-node/config"
	proto "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// ProposerValidator interface for proposer signature verification
type ProposerValidator interface {
	ValidateProposerSignature(proposer proto.Address, blockHash proto.Hash, signature proto.Signature) bool
	IsValidProposer(proposer proto.Address, height uint64) bool
	ValidateCommitSignatures(blockHash proto.Hash, commitSigs []CommitSignature) error
}

type BlockChain struct {
	LatestHeight    uint64
	LatestBlockHash string
	db              *leveldb.DB
	cfg             *config.Config
	Mempool         *Mempool
	mu              sync.RWMutex // If no write, multiple read goroutines can access

	// Callback for PoA verification (set in consensus package)
	proposerValidator ProposerValidator
}

func NewChainState(db *leveldb.DB, cfg *config.Config) (*BlockChain, error) {
	bc := &BlockChain{
		db:      db,
		cfg:     cfg,
		Mempool: NewMempool(),
	}

	if err := bc.LoadChainDB(); err != nil {
		return nil, err
	}

	// Only boot node or block producer creates genesis block
	// sync-only nodes receive genesis block via P2P
	shouldCreateGenesis := (cfg.Common.Mode == "boot" || cfg.Common.BlockProducer) &&
		(bc.LatestHeight == 0 && bc.LatestBlockHash == "")

	if shouldCreateGenesis {
		genesisBlk, err := bc.SetGenesisBlock()
		if err != nil {
			return nil, err
		}

		result, err := bc.AddBlock(*genesisBlk)
		if err != nil {
			return nil, err
		}
		if !result {
			return nil, fmt.Errorf("failed to add genesis block to chain")
		}
	}

	// var height uint64
	// height = 0

	// bb, err := bc.GetBlock(0)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// fmt.Println(bb)

	return bc, nil
}

func (p *BlockChain) LoadChainDB() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	heightBytes, err := p.db.Get([]byte(proto.PrefixMetaHeight), nil)
	if err != nil && err != leveldb.ErrNotFound {
		return fmt.Errorf("failed to load latest height: %w", err)
	}

	if err == leveldb.ErrNotFound {
		p.LatestHeight = 0
		p.LatestBlockHash = ""
		return nil
	}

	height, err := strconv.ParseUint(string(heightBytes), 10, 64)
	if err != nil {
		return fmt.Errorf("invaild height format: %w", err)
	}
	p.LatestHeight = height

	blkHashBytes, err := p.db.Get([]byte(proto.PrefixMetaBlockHash), nil)
	if err != nil && err != leveldb.ErrNotFound {
		return fmt.Errorf("failed to load latest block hash: %w", err)
	}

	p.LatestBlockHash = string(blkHashBytes)

	return nil
}

func (p *BlockChain) GetChainStatus() BlockChain {
	if p == nil {
		return BlockChain{}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	return BlockChain{
		LatestHeight:    p.LatestHeight,
		LatestBlockHash: p.LatestBlockHash,
	}
}

func (p *BlockChain) GetLatestHeight() (uint64, error) {
	if p == nil {
		return 0, fmt.Errorf("blockchain is not initialized")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return 0 instead of error even if chain is empty (support sync-only node)
	return p.LatestHeight, nil
}

func (p *BlockChain) GetLatestBlockHash() (string, error) {
	if p == nil {
		return "", fmt.Errorf("blockchain is not initialized")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.LatestBlockHash == "" {
		return "", fmt.Errorf("no blocks in the chain yet")
	}

	return p.LatestBlockHash, nil
}

func (p *BlockChain) UpdateChainState(height uint64, blockHash string) error {
	if height == 0 && blockHash == "" {
		return nil
	}

	// memory update
	p.LatestBlockHash = blockHash
	p.LatestHeight = height

	// db batch update
	batch := new(leveldb.Batch)

	heightKey := []byte(proto.PrefixMetaHeight)
	batch.Put(heightKey, []byte(fmt.Sprintf("%d", height)))

	blkHashKey := []byte(proto.PrefixMetaBlockHash)
	batch.Put(blkHashKey, []byte(blockHash))

	// height - hash mapping
	heightToHashKey := []byte(fmt.Sprintf("%s%d", proto.PrefixMetaHeight, height))
	batch.Put(heightToHashKey, []byte(blockHash))

	// batch write excute
	return p.db.Write(batch, nil)
}

// SetProposerValidator sets interface for PoA verification
func (p *BlockChain) SetProposerValidator(validator ProposerValidator) {
	p.proposerValidator = validator
}

// GetProposerValidator returns PoA verification interface
func (p *BlockChain) GetProposerValidator() ProposerValidator {
	return p.proposerValidator
}

// GetMinFee returns minimum fee
func (p *BlockChain) GetMinFee() uint64 {
	return p.cfg.Fee.MinFee
}

// GetBlockReward returns block reward
func (p *BlockChain) GetBlockReward() uint64 {
	return p.cfg.Fee.BlockReward
}

// GetMaxMemoSize returns maximum memo size
func (p *BlockChain) GetMaxMemoSize() uint64 {
	return p.cfg.Transaction.MaxMemoSize
}

// GetMaxDataSize returns maximum data size
func (p *BlockChain) GetMaxDataSize() uint64 {
	return p.cfg.Transaction.MaxDataSize
}
