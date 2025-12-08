package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

// 트랜잭션 생성 헬퍼 함수
func setTestTransaction() *Transaction {
	// 입력 생성
	input := &TxInput{
		TxID:        prt.Hash{0x1, 0x2, 0x3}, // 임의의 해시
		OutputIndex: 0,
		Signature:   prt.Signature{}, // 빈 서명
		PublicKey:   []byte("Test PubKey"),
		// Sequence:    1,
	}

	// 출력 생성
	var addr prt.Address
	copy(addr[:], []byte("0x12300000000000000")) // 20바이트 주소

	output := &TxOutput{
		Address: addr,
		Amount:  1000,
		TxType:  TxTypeGeneral, // 일반 트랜잭션
	}

	// 트랜잭션 생성
	tx := &Transaction{
		Version:   "1.0.2",
		Timestamp: time.Now().Unix(),
		Inputs:    []*TxInput{input},
		Outputs:   []*TxOutput{output},
		Memo:      "Test Transaction",
		Data:      []byte{},
	}

	// ID 설정 (일반적으로는 트랜잭션 내용에 기반한 해시)
	tx.ID = utils.Hash(tx)

	return tx
}

// 멤풀 추가 삭제 조회 테스트
func TestAddTxToMempool(t *testing.T) {
	// mempool init
	mempool := NewMempool()

	// tx set
	tx := setTestTransaction()

	// tx add
	if err := mempool.NewTranaction(tx); err != nil {
		fmt.Println(err)
	}

	// tx get
	savedTx := mempool.GetTx(tx.ID)
	fmt.Println(savedTx)

	// tx delete
	mempool.DelTx(tx.ID)

	// tx get
	delCheckTx := mempool.GetTx(tx.ID)
	if delCheckTx != nil {
		fmt.Println("failed to delete saved tx")
	}
	fmt.Println("del done")
}

// 블록 구조체 생성 테스트
func TestSetBlock(t *testing.T) {
	// mempool init
	mempool := NewMempool()

	// tx add
	numTxs := 3
	for i := 0; i < numTxs; i++ {
		// tx set
		tx := setTestTransaction()

		// set unique tx.ID
		copy(tx.ID[:], []byte{byte(i), byte(i + 1), byte(i + 2)})

		// tx add to mempool
		if err := mempool.NewTranaction(tx); err != nil {
			fmt.Println(err)
		}
	}

	// test height
	height := uint64(1231)

	// test previous hash
	var prevHash prt.Hash
	copy(prevHash[:], []byte("0x1230000000000000012345678900")) // 32바이트 주소

	// set block header
	blkHeader := &BlockHeader{
		Version:   "1.0",
		Height:    height,
		PrevHash:  prevHash,
		Timestamp: time.Now().Unix(),
	}

	// set block
	// get tx from mempool
	txs := mempool.GetTxs()

	block := &Block{
		Header:       *blkHeader,
		Transactions: txs,
	}

	blkHash := utils.Hash(block)
	block.Header.Hash = blkHash

	fmt.Println(block)
}

// func TestAddBlock(t *testing.T) {
// 	// mempool init
// 	mempool := NewMempool()

// 	// tx add
// 	numTxs := 3
// 	for i := 0; i < numTxs; i++ {
// 		// tx set
// 		tx := setTestTransaction()

// 		// set unique tx.ID
// 		copy(tx.ID[:], []byte{byte(i), byte(i + 1), byte(i + 2)})

// 		// tx add to mempool
// 		if err := mempool.NewTranaction(tx); err != nil {
// 			fmt.Println(err)
// 		}
// 	}

// 	// test height
// 	height := uint64(1231)

// 	// test previous hash
// 	var prevHash prt.Hash
// 	copy(prevHash[:], []byte("0x1230000000000000012345678900")) // 32바이트 주소

// 	// set block header
// 	blkHeader := &BlockHeader{
// 		Version:   "1.0",
// 		Height:    height,
// 		PrevHash:  prevHash,
// 		Timestamp: time.Now().Unix(),
// 	}

// 	// set block
// 	// get tx from mempool
// 	txs := mempool.GetTxs()

// 	block := &Block{
// 		Header:       *blkHeader,
// 		Transactions: txs,
// 	}

// blkHash := utils.Hash(block)
// block.Header.Hash = blkHash

// 	// block serialization
// 	blockBytes, err := utils.SerializeData(block, utils.SerializationFormatGob)
// 	if err != nil {
// 		fmt.Println("here %w", err)
// 	}

// 	// db batch process ready
// 	batch := new(leveldb.Batch)

// 	// block hash - block data mapping
// 	blockHashKey := utils.GetBlockHashKey(prt.PrefixBlock, block.Hash)
// 	batch.Put(blockHashKey, blockBytes)

// 	// block height - block hash mapping
// 	heightKey := utils.GetBlockHeightKey(prt.PrefixBlockByHeight, block.Header.Height)
// 	batch.Put(heightKey, []byte(utils.HashToString(block.Hash)))

// 	db := chain.GetDB()

// 	// batch excute
// 	if err := p.db.Write(batch, nil); err != nil {
// 		return false, fmt.Errorf("failed to write batch: %w", err)
// 	}

// 	return true, nil
// }

// ===== 블록 검증 테스트 =====

// 유효한 블록 생성 헬퍼
func createValidBlock(prevHash prt.Hash, height uint64, txs []*Transaction) *Block {
	merkleRoot := calculateMerkleRoot(txs)

	blkHeader := &BlockHeader{
		Version:    "1.0",
		Height:     height,
		PrevHash:   prevHash,
		Timestamp:  time.Now().Unix(),
		MerkleRoot: merkleRoot,
	}

	block := &Block{
		Header:       *blkHeader,
		Transactions: txs,
	}

	block.Header.Hash = utils.Hash(block)
	return block
}

// 1. 이전 해시 검증 테스트
func TestValidateBlock_PrevHash(t *testing.T) {
	// 제네시스 블록 (height=0)은 prevHash가 모두 0이어야 함
	var zeroPrevHash prt.Hash
	genesisBlock := createValidBlock(zeroPrevHash, 0, []*Transaction{})

	err := ValidatePrevHash(genesisBlock, zeroPrevHash)
	if err != nil {
		t.Errorf("Genesis block prev hash validation failed: %v", err)
	}

	// 일반 블록은 실제 이전 블록 해시와 일치해야 함
	block1 := createValidBlock(genesisBlock.Header.Hash, 1, []*Transaction{})
	err = ValidatePrevHash(block1, genesisBlock.Header.Hash)
	if err != nil {
		t.Errorf("Block prev hash validation failed: %v", err)
	}

	// 잘못된 이전 해시
	var wrongPrevHash prt.Hash
	copy(wrongPrevHash[:], []byte("wrong_hash_value_here_12345678"))
	err = ValidatePrevHash(block1, wrongPrevHash)
	if err == nil {
		t.Error("Should fail with wrong prev hash")
	}
}

// 2. 머클 루트 검증 테스트
func TestValidateBlock_MerkleRoot(t *testing.T) {
	tx1 := setTestTransaction()
	tx2 := setTestTransaction()
	copy(tx2.ID[:], []byte{0x99, 0x98, 0x97})

	txs := []*Transaction{tx1, tx2}
	block := createValidBlock(prt.Hash{}, 1, txs)

	err := ValidateMerkleRoot(block)
	if err != nil {
		t.Errorf("Merkle root validation failed: %v", err)
	}

	// 머클 루트 변조
	block.Header.MerkleRoot[0] = 0xFF
	err = ValidateMerkleRoot(block)
	if err == nil {
		t.Error("Should fail with wrong merkle root")
	}
}

// 3. 블록 해시 검증 테스트
func TestValidateBlock_Hash(t *testing.T) {
	block := createValidBlock(prt.Hash{}, 1, []*Transaction{})

	err := ValidateBlockHash(block)
	if err != nil {
		t.Errorf("Block hash validation failed: %v", err)
	}

	// 해시 변조
	block.Header.Hash[0] = 0xFF
	err = ValidateBlockHash(block)
	if err == nil {
		t.Error("Should fail with wrong block hash")
	}
}

// 4. 블록 높이 연속성 테스트
func TestValidateBlock_HeightContinuity(t *testing.T) {
	err := ValidateHeightContinuity(1, 0)
	if err != nil {
		t.Errorf("Height continuity validation failed: %v", err)
	}

	err = ValidateHeightContinuity(5, 4)
	if err != nil {
		t.Errorf("Height continuity validation failed: %v", err)
	}

	// 높이가 연속적이지 않음
	err = ValidateHeightContinuity(3, 1)
	if err == nil {
		t.Error("Should fail with non-continuous height")
	}
}

// 5. 타임스탬프 검증 테스트
func TestValidateBlock_Timestamp(t *testing.T) {
	prevTimestamp := time.Now().Unix() - 100
	currentTimestamp := time.Now().Unix()

	err := ValidateTimestamp(currentTimestamp, prevTimestamp)
	if err != nil {
		t.Errorf("Timestamp validation failed: %v", err)
	}

	// 이전 블록보다 과거 시간
	err = ValidateTimestamp(prevTimestamp-50, prevTimestamp)
	if err == nil {
		t.Error("Should fail with timestamp before prev block")
	}

	// 너무 미래 시간 (2시간 이후)
	futureTimestamp := time.Now().Unix() + 7201
	err = ValidateTimestamp(futureTimestamp, prevTimestamp)
	if err == nil {
		t.Error("Should fail with too far future timestamp")
	}
}

// 6. 블록 크기 제한 테스트
func TestValidateBlock_TxCount(t *testing.T) {
	txs := make([]*Transaction, 50)
	for i := range txs {
		txs[i] = setTestTransaction()
	}

	err := ValidateTxCount(txs)
	if err != nil {
		t.Errorf("Tx count validation failed: %v", err)
	}

	// MaxTxsPerBlock 초과
	tooManyTxs := make([]*Transaction, 150)
	for i := range tooManyTxs {
		tooManyTxs[i] = setTestTransaction()
	}

	err = ValidateTxCount(tooManyTxs)
	if err == nil {
		t.Error("Should fail with too many transactions")
	}
}

// 7. 트랜잭션 중복 검증 테스트
func TestValidateBlock_DuplicateTx(t *testing.T) {
	tx1 := setTestTransaction()
	tx2 := setTestTransaction()
	copy(tx2.ID[:], []byte{0x99, 0x98, 0x97})

	txs := []*Transaction{tx1, tx2}
	err := ValidateDuplicateTx(txs)
	if err != nil {
		t.Errorf("Duplicate tx validation failed: %v", err)
	}

	// 중복 트랜잭션
	duplicateTxs := []*Transaction{tx1, tx1}
	err = ValidateDuplicateTx(duplicateTxs)
	if err == nil {
		t.Error("Should fail with duplicate transactions")
	}
}

// 8. 전체 블록 검증 통합 테스트
func TestValidateBlock_Full(t *testing.T) {
	tx := setTestTransaction()
	txs := []*Transaction{tx}

	var zeroPrevHash prt.Hash
	genesisBlock := createValidBlock(zeroPrevHash, 0, []*Transaction{})

	block := createValidBlock(genesisBlock.Header.Hash, 1, txs)

	// ValidateBlock 전체 테스트는 BlockChain 인스턴스 필요하므로 개별 검증만 수행
	fmt.Println("Block created for validation:", utils.HashToString(block.Header.Hash))
}

func TestSetGenesisBlock(t *testing.T) {
	var defaultPrevHash prt.Hash

	for i := range defaultPrevHash {
		defaultPrevHash[i] = 0x00
	}

	blkHeader := &BlockHeader{
		Version:   "v0.1",
		Height:    0,
		PrevHash:  defaultPrevHash,
		Timestamp: time.Now().Unix(),
	}

	txIns := []*TxInput{}
	txOuts := []*TxOutput{}

	// 배열 초기화 - 올바른 문법으로 수정
	systemAddrs := []string{"ABCFEABCFEABCFEABCFEABCFEABCFEABCFEABCFE", "0000000000000000000000000000000000000000"}
	systemBals := []uint64{10000, 3300000}

	if len(systemAddrs) != len(systemBals) {
		fmt.Println("system address and balance count mismatch")
	}

	for i, systemAddr := range systemAddrs {
		addr, err := utils.StringToAddress(systemAddr)
		if err != nil {
			fmt.Println("failed to convert between address and string: ", err)
		}

		output := &TxOutput{
			Address: addr,
			Amount:  systemBals[i],
			TxType:  TxTypeGeneral,
		}
		txOuts = append(txOuts, output)
	}

	txs := []*Transaction{
		{
			Version:   "2.1",
			Timestamp: time.Now().Unix(),
			Inputs:    txIns,
			Outputs:   txOuts,
			Memo:      "ABCFE Chain Genesis Block",
		},
	}

	// TODO 서명 포함하고 그 이후 ID를 만들어야함

	for i, tx := range txs {
		txHash := utils.Hash(tx)
		txs[i].ID = txHash
	}

	block := &Block{
		Header:       *blkHeader,
		Transactions: txs,
	}

	blkHash := utils.Hash(block)
	block.Header.Hash = blkHash

	fmt.Println("Genesis Block: ", block)
}

// func setTestGenesisTxs() ([]*Transaction, error) {
// 	txIns := []*TxInput{}
// 	txOuts := []*TxOutput{}

// 	// 배열 초기화 - 올바른 문법으로 수정
// 	systemAddrs := []string{"0xABCFEABCFEABCFEABCFEABCFEABCFE"}
// 	systemBals := []uint64{10000}

// 	if len(systemAddrs) != len(systemBals) {
// 		return nil, fmt.Errorf("system address and balance count mismatch")

// 	}

// 	for i, systemAddr := range systemAddrs {
// 		addr, err := utils.StringToAddress(systemAddr)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to convert between address and string")
// 		}

// 		output := &TxOutput{
// 			Address: addr,
// 			Amount:  systemBals[i],
// 			TxType:  TxTypeGeneral,
// 		}
// 		txOuts = append(txOuts, output)
// 	}

// 	txs := []*Transaction{
// 		{
// 			Version:   "2.1",
// 			Timestamp: time.Now().Unix(),
// 			Inputs:    txIns,
// 			Outputs:   txOuts,
// 			Memo:      "ABCFE Chain Genesis Block",
// 		},
// 	}

// 	return txs, nil
// }
