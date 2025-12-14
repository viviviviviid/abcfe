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

// 트랜잭션 구조체
type Transaction struct {
	Version   string   `json:"version"`   // 트랜잭션 버전
	ID        prt.Hash `json:"id"`        // 트랜잭션 ID (해시)
	Timestamp int64    `json:"timestamp"` // 트랜잭션 생성 시간

	Inputs  []*TxInput  `json:"inputs"`  // 트랜잭션 입력
	Outputs []*TxOutput `json:"outputs"` // 트랜잭션 출력 // 잔돈 때문에 단일 출력이 아닌 다중 출력이어야함

	Memo string `json:"memo"` // 트랜잭션 메모 (inputData 대체)
	Data []byte `json:"data"` // 임의 데이터 (스마트 컨트랙트 호출 등)
	// Status // TODO
	// Signature // TODO
}

type TxInput struct {
	TxID        prt.Hash      `json:"txId"`        // 참조 트랜잭션 ID
	OutputIndex uint64        `json:"outputIndex"` // 참조 출력 인덱스
	Signature   prt.Signature `json:"signature"`   // 서명
	PublicKey   []byte        `json:"publicKey"`   // 공개키
	// Sequence    uint64        `json:"sequence"`    // 시퀀스 번호 (RBF 지원)
}

type TxOutput struct {
	Address prt.Address `json:"address"` // 수신자 주소
	Amount  uint64      `json:"amount"`  // 금액 (int에서 uint64로 변경)
	TxType  uint8       `json:"txType"`  // 스크립트 타입 (일반/스테이킹/기타)
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

	// 수수료 포함한 총 필요 금액 검증
	requiredAmount := amount + fee
	if p.CalBalanceUtxo(utxos) < requiredAmount {
		return &Transaction{}, fmt.Errorf("not enough balance: need %d (amount) + %d (fee) = %d", amount, fee, requiredAmount)
	}

	txInAndOut, err := p.setTxIOPair(utxos, from, to, amount, fee, txType)
	if err != nil {
		return &Transaction{}, err
	}

	// GOB 역직렬화 후 nil이 빈 슬라이스로 바뀌는 문제 해결
	// 트랜잭션 생성 시 nil을 빈 슬라이스로 정규화하여 해시 일관성 유지
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

// tx input과 output을 구성 (fee 포함)
func (p *BlockChain) setTxIOPair(utxos []*UTXO, from prt.Address, to prt.Address, amount uint64, fee uint64, txType uint8) (*TxIOPair, error) {
	var txInAndOut TxIOPair
	var total uint64

	// 수수료 포함한 총 필요 금액
	requiredAmount := amount + fee

	// set tx in
	for _, utxo := range utxos {
		// 수수료 포함한 금액을 충족하면 중단
		if total >= requiredAmount {
			break
		}
		// ! pubkey는 전처리 시그니처 후처리 필요
		txIn := p.setTxInput(utxo.TxId, utxo.OutputIndex, prt.Signature{}, nil)
		txInAndOut.TxIns = append(txInAndOut.TxIns, txIn)

		total += utxo.TxOut.Amount
	}

	// utxo 탈탈 털었는데도 돈이 부족하다면 에러
	if total < requiredAmount {
		return nil, fmt.Errorf("not enough balance: required %d (amount %d + fee %d), have %d", requiredAmount, amount, fee, total)
	}

	// set tx out - 수신자에게 보내는 금액
	txOut := p.setTxOutput(to, amount, txType)
	txInAndOut.TxOuts = append(txInAndOut.TxOuts, txOut)

	// 거슬러줘야한다면 잔액 반환 (수수료는 Output에 포함되지 않음 = 암묵적 수수료)
	if total > requiredAmount {
		changeOut := total - requiredAmount // total - amount - fee
		txOut := p.setTxOutput(from, changeOut, txType)
		txInAndOut.TxOuts = append(txInAndOut.TxOuts, txOut)
	}

	return &txInAndOut, nil
}

func (p *BlockChain) setTxInput(txOutID prt.Hash, txOutIdx uint64, sig prt.Signature, pubKey []byte) *TxInput {
	// GOB 역직렬화 후 nil이 빈 슬라이스로 바뀌는 문제 해결
	// nil을 빈 슬라이스로 정규화하여 해시 일관성 유지
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
		return prt.Hash{} // 빈 해시 반환
	}

	// 각 트랜잭션의 ID(해시)를 사용
	// TX.ID는 이미 트랜잭션의 해시이므로 직접 사용
	hashes := make([]prt.Hash, len(txs))
	for i, tx := range txs {
		hashes[i] = tx.ID
	}

	// 머클 트리 계산 // 재귀 호출
	return buildMerkleTree(hashes)
}

func buildMerkleTree(hashes []prt.Hash) prt.Hash {
	if len(hashes) == 1 {
		return hashes[0] // 루트에 도달
	}

	// 짝수 개로 맞추기 => 홀수 개면 마지막 해시를 복제
	if len(hashes)%2 != 0 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// 다음 레벨 계산
	nextLevel := make([]prt.Hash, len(hashes)/2)
	for i := 0; i < len(hashes); i += 2 {
		combined := append(hashes[i][:], hashes[i+1][:]...)
		nextLevel[i/2] = sha256.Sum256(combined)
	}

	return buildMerkleTree(nextLevel)
}

// CreateSignedTx 서명된 트랜잭션 생성 (fee 포함)
func (p *BlockChain) CreateSignedTx(from, to prt.Address, amount uint64, fee uint64, memo string, data []byte, txType uint8, privateKeyBytes, publicKeyBytes []byte) (*Transaction, error) {
	utxos, err := p.GetUtxoList(from, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get UTXO list: %w", err)
	}

	// 디버깅: UTXO 목록 확인
	logger.Debug("[CreateSignedTx] from: ", utils.AddressToString(from))
	logger.Debug("[CreateSignedTx] UTXO count: ", len(utxos))
	for i, utxo := range utxos {
		logger.Debug("[CreateSignedTx] UTXO[", i, "]: txId=", utils.HashToString(utxo.TxId)[:16], " amount=", utxo.TxOut.Amount, " spent=", utxo.Spent)
	}

	// 수수료 포함한 총 필요 금액
	requiredAmount := amount + fee
	balance := p.CalBalanceUtxo(utxos)
	logger.Debug("[CreateSignedTx] balance: ", balance, " requiredAmount: ", requiredAmount)
	if balance < requiredAmount {
		return nil, fmt.Errorf("not enough balance: have %d, need %d (amount %d + fee %d)", balance, requiredAmount, amount, fee)
	}

	// TX Input/Output 구성
	var txIns []*TxInput
	var total uint64

	// GOB 역직렬화 후 nil이 빈 슬라이스로 바뀌는 문제 해결
	// nil을 빈 슬라이스로 정규화하여 해시 일관성 유지
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
			// Signature는 나중에 설정
		}
		txIns = append(txIns, txIn)
		total += utxo.TxOut.Amount
	}

	if total < requiredAmount {
		return nil, fmt.Errorf("not enough balance after collecting UTXOs: have %d, need %d", total, requiredAmount)
	}

	// TX Output 구성 - 수신자에게 보내는 금액
	var txOuts []*TxOutput
	txOuts = append(txOuts, &TxOutput{
		Address: to,
		Amount:  amount,
		TxType:  txType,
	})

	// 거스름돈 (수수료는 Output에 포함되지 않음 = 암묵적 수수료)
	if total > requiredAmount {
		txOuts = append(txOuts, &TxOutput{
			Address: from,
			Amount:  total - requiredAmount, // total - amount - fee
			TxType:  txType,
		})
	}

	// GOB 역직렬화 후 nil이 빈 슬라이스로 바뀌는 문제 해결
	// 트랜잭션 생성 시 nil을 빈 슬라이스로 정규화하여 해시 일관성 유지
	normalizedData := data
	if normalizedData == nil {
		normalizedData = []byte{}
	}
	normalizedInputs := txIns
	if normalizedInputs == nil {
		normalizedInputs = []*TxInput{}
	}

	// 트랜잭션 생성
	tx := &Transaction{
		Version:   p.cfg.Version.Transaction,
		Timestamp: time.Now().Unix(),
		Inputs:    normalizedInputs,
		Outputs:   txOuts,
		Memo:      memo,
		Data:      normalizedData,
	}

	// TX ID 계산 (서명 전에 계산)
	tx.ID = utils.Hash(tx)

	// 각 입력에 서명
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

// ValidateTxSignatures 트랜잭션의 모든 입력 서명 검증
func (p *BlockChain) ValidateTxSignatures(tx *Transaction) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}

	// Coinbase TX는 서명 검증 불필요
	if len(tx.Inputs) == 0 {
		return nil
	}

	// TX 해시 계산 (서명 검증용)
	// 서명은 TX ID를 기준으로 함
	txHashBytes := utils.HashToBytes(tx.ID)

	for i, input := range tx.Inputs {
		// 공개키가 없으면 에러
		if len(input.PublicKey) == 0 {
			return fmt.Errorf("input[%d]: public key is empty", i)
		}

		// 서명 검증
		valid, err := crypto.VerifySignatureWithBytes(input.PublicKey, txHashBytes, input.Signature)
		if err != nil {
			return fmt.Errorf("input[%d]: signature verification error: %w", i, err)
		}
		if !valid {
			return fmt.Errorf("input[%d]: invalid signature", i)
		}

		// UTXO 소유권 검증 (공개키 -> 주소 -> UTXO 소유자 확인)
		pubKey, err := crypto.BytesToPublicKey(input.PublicKey)
		if err != nil {
			return fmt.Errorf("input[%d]: failed to parse public key: %w", i, err)
		}

		signerAddr, err := crypto.PublicKeyToAddress(pubKey)
		if err != nil {
			return fmt.Errorf("input[%d]: failed to derive address from public key: %w", i, err)
		}

		// 참조하는 UTXO 가져오기
		utxo, err := p.GetUtxoByTxIdAndIdx(input.TxID, input.OutputIndex)
		if err != nil {
			return fmt.Errorf("input[%d]: failed to get referenced UTXO: %w", i, err)
		}

		// UTXO 소유자와 서명자 주소 비교
		if signerAddr != utxo.TxOut.Address {
			return fmt.Errorf("input[%d]: signer address does not match UTXO owner", i)
		}
	}

	return nil
}
