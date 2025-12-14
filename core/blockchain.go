package core

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/abcfe/abcfe-node/config"
	proto "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// ProposerValidator 제안자 서명 검증을 위한 인터페이스
type ProposerValidator interface {
	ValidateProposerSignature(proposer proto.Address, blockHash proto.Hash, signature proto.Signature) bool
	IsValidProposer(proposer proto.Address, height uint64) bool
}

type BlockChain struct {
	LatestHeight    uint64
	LatestBlockHash string
	db              *leveldb.DB
	cfg             *config.Config
	Mempool         *Mempool
	mu              sync.RWMutex // 쓰기가 없는 경우, 읽기 고루틴이 여러개 접근 가능

	// PoA 검증을 위한 콜백 (consensus 패키지에서 설정)
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
	
	// boot 노드나 블록 생성자인 경우에만 제네시스 블록 생성
	// sync-only 노드는 P2P를 통해 제네시스 블록을 받음
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

	// 빈 체인인 경우에도 에러 대신 0을 리턴 (sync-only 노드 지원)
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

// SetProposerValidator PoA 검증을 위한 인터페이스 설정
func (p *BlockChain) SetProposerValidator(validator ProposerValidator) {
	p.proposerValidator = validator
}

// GetProposerValidator PoA 검증 인터페이스 반환
func (p *BlockChain) GetProposerValidator() ProposerValidator {
	return p.proposerValidator
}

// GetMinFee 최소 수수료 반환
func (p *BlockChain) GetMinFee() uint64 {
	return p.cfg.Fee.MinFee
}

// GetBlockReward 블록 보상 반환
func (p *BlockChain) GetBlockReward() uint64 {
	return p.cfg.Fee.BlockReward
}

// GetMaxMemoSize 최대 메모 크기 반환
func (p *BlockChain) GetMaxMemoSize() uint64 {
	return p.cfg.Transaction.MaxMemoSize
}

// GetMaxDataSize 최대 데이터 크기 반환
func (p *BlockChain) GetMaxDataSize() uint64 {
	return p.cfg.Transaction.MaxDataSize
}
