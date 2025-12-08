package core

import (
	"fmt"
	"time"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

type Block struct {
	Header       BlockHeader    `json:"header"`       // 블록 헤더
	Transactions []*Transaction `json:"transactions"` // 트랜잭션 목록

	// TODO: Consensus 시작시
	// Proposer      Address     `json:"proposer"`      // 블록 제안자
	// Validators    []Address   `json:"validators"`    // 검증자 목록 // 자세한 정보는 상위에 있는 컨센서스 패키지에서 처리할 예정
	// Signatures    []Signature `json:"signatures"`    // 검증자 서명
	// ConsensusData []byte      `json:"consensusData"` // 컨센서스 관련 데이터는 단방향 참조를 위해 직렬화만
}

type BlockHeader struct {
	Hash       prt.Hash `json:"hash"`       // 블록 해시
	PrevHash   prt.Hash `json:"prevHash"`   // 이전 블록 해시
	Version    string   `json:"version"`    // 블록체인 프로토콜 버전
	Height     uint64   `json:"height"`     // 블록 높이 (uint64로 변경)
	MerkleRoot prt.Hash `json:"merkleRoot"` // 트랜잭션 머클 루트
	Timestamp  int64    `json:"timestamp"`  // 블록 생성 시간 (Unix 타임스탬프)
	// StateRoot  Hash   `json:"stateRoot"`  // 상태 머클 루트 (UTXO 또는 계정 상태)
}

func (p *BlockChain) SetBlock(prevHash prt.Hash, height uint64) *Block {
	// 메모리 풀에서 트랜잭션 가져오기
	txs := p.Mempool.GetTxs()

	// 머클 루트 계산
	merkleRoot := calculateMerkleRoot(txs)

	// 헤더 구성
	blkHeader := &BlockHeader{
		Version:    p.cfg.Version.Protocol,
		Height:     height,
		PrevHash:   prevHash,
		Timestamp:  time.Now().Unix(),
		MerkleRoot: merkleRoot,
	}

	blk := &Block{
		Header:       *blkHeader,
		Transactions: txs,
	}

	blkHash := utils.Hash(blk)
	blk.Header.Hash = blkHash

	return blk
}

// TOOD: 모듈화 여부는 이후에 내용이 많아질 경우
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
		delete(p.Mempool.transactions, utils.HashToString(tx.ID))
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

		// TODO: whole은 추후 deprecated 가능성 존재
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
	p.mu.Lock()
	defer p.mu.Unlock()

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

// ValidateBlock은 validate.go로 이동됨

// BlockToJSON 블록을 JSON 형식으로 변환
func blockToJSON(block interface{}) ([]byte, error) {
	return utils.SerializeData(block, utils.SerializationFormatJSON)
}

// JSONToBlock JSON 형식에서 블록으로 변환
func jsonToBlock(data []byte, block interface{}) error {
	return utils.DeserializeData(data, block, utils.SerializationFormatJSON)
}
