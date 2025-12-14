package core

import (
	"fmt"
	"time"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

const (
	MaxFutureBlockTime = 2 * 60 * 60 // 2시간 (초 단위)
)

// ValidatePrevHash 이전 블록 해시 검증
func ValidatePrevHash(block *Block, expectedPrevHash prt.Hash) error {
	if block.Header.PrevHash != expectedPrevHash {
		return fmt.Errorf("prev hash mismatch: expected %s, got %s",
			utils.HashToString(expectedPrevHash),
			utils.HashToString(block.Header.PrevHash))
	}
	return nil
}

// ValidateMerkleRoot 머클 루트 검증
func ValidateMerkleRoot(block *Block) error {
	calculatedRoot := calculateMerkleRoot(block.Transactions)
	if block.Header.MerkleRoot != calculatedRoot {
		return fmt.Errorf("merkle root mismatch: expected %s, got %s",
			utils.HashToString(calculatedRoot),
			utils.HashToString(block.Header.MerkleRoot))
	}
	return nil
}

// ValidateBlockHash 블록 해시 검증
func ValidateBlockHash(block *Block) error {
	// 해시 계산을 위해 임시로 해시 필드를 비움
	storedHash := block.Header.Hash
	block.Header.Hash = prt.Hash{}

	// Header만으로 해시 계산 (SetBlock과 동일한 방식)
	calculatedHash := utils.Hash(block.Header)
	block.Header.Hash = storedHash

	if storedHash != calculatedHash {
		return fmt.Errorf("block hash mismatch: expected %s, got %s",
			utils.HashToString(calculatedHash),
			utils.HashToString(storedHash))
	}
	return nil
}

// ValidateHeightContinuity 블록 높이 연속성 검증
func ValidateHeightContinuity(blockHeight uint64, prevBlockHeight uint64) error {
	if blockHeight != prevBlockHeight+1 {
		return fmt.Errorf("height not continuous: expected %d, got %d",
			prevBlockHeight+1, blockHeight)
	}
	return nil
}

// ValidateTimestamp 타임스탬프 검증
func ValidateTimestamp(blockTimestamp int64, prevBlockTimestamp int64) error {
	// 이전 블록보다 이후이거나 같아야 함 (같은 초에 여러 블록 생성 가능)
	if blockTimestamp < prevBlockTimestamp {
		return fmt.Errorf("timestamp must be >= prev block: prev=%d, current=%d",
			prevBlockTimestamp, blockTimestamp)
	}

	// 현재 시간보다 너무 미래면 안됨
	now := time.Now().Unix()
	if blockTimestamp > now+MaxFutureBlockTime {
		return fmt.Errorf("timestamp too far in future: max=%d, got=%d",
			now+MaxFutureBlockTime, blockTimestamp)
	}

	return nil
}

// ValidateTxCount 트랜잭션 개수 제한 검증
func ValidateTxCount(txs []*Transaction) error {
	if len(txs) > prt.MaxTxsPerBlock {
		return fmt.Errorf("too many transactions: max=%d, got=%d",
			prt.MaxTxsPerBlock, len(txs))
	}
	return nil
}

// ValidateDuplicateTx 중복 트랜잭션 검증
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

// ValidateDuplicateUTXO 같은 블록 내 동일 UTXO 사용 검증
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

// ValidateProposer 제안자 주소 검증 (빈 주소가 아닌지 확인)
func ValidateProposer(block *Block) error {
	var emptyAddr prt.Address
	if block.Proposer == emptyAddr {
		return fmt.Errorf("block proposer is empty")
	}
	return nil
}

// ValidateProposerSignature 제안자 서명 검증 (블록 해시에 대한 서명)
func ValidateProposerSignature(block *Block, validator ProposerValidator) error {
	if validator == nil {
		// 검증자가 설정되지 않은 경우 (단독 노드 등) 건너뜀
		return nil
	}

	// 제안자가 유효한 검증자인지 확인
	if !validator.IsValidProposer(block.Proposer, block.Header.Height) {
		return fmt.Errorf("proposer %s is not a valid validator for height %d",
			utils.AddressToString(block.Proposer), block.Header.Height)
	}

	// 서명 검증
	if !validator.ValidateProposerSignature(block.Proposer, block.Header.Hash, block.Signature) {
		return fmt.Errorf("invalid proposer signature for block %s",
			utils.HashToString(block.Header.Hash))
	}

	return nil
}

// ValidateBlock 블록 전체 검증 (BlockChain 메서드)
func (p *BlockChain) ValidateBlock(block Block) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 제네시스 블록은 별도 검증
	if block.Header.Height == 0 {
		return p.validateGenesisBlock(&block)
	}

	// 이전 블록 조회 (lock 이미 획득됨)
	prevBlock, err := p.getBlockByHeightNoLock(block.Header.Height - 1)
	if err != nil {
		return fmt.Errorf("failed to get prev block: %w", err)
	}

	// 1. 이전 해시 검증
	if err := ValidatePrevHash(&block, prevBlock.Header.Hash); err != nil {
		return err
	}

	// 2. 머클 루트 검증
	if err := ValidateMerkleRoot(&block); err != nil {
		return err
	}

	// 3. 블록 해시 검증
	if err := ValidateBlockHash(&block); err != nil {
		return err
	}

	// 4. 높이 연속성 검증
	if err := ValidateHeightContinuity(block.Header.Height, prevBlock.Header.Height); err != nil {
		return err
	}

	// 5. 타임스탬프 검증
	if err := ValidateTimestamp(block.Header.Timestamp, prevBlock.Header.Timestamp); err != nil {
		return err
	}

	// 6. 제안자 주소 검증
	if err := ValidateProposer(&block); err != nil {
		return err
	}

	// 7. 제안자 서명 검증 (PoA)
	if err := ValidateProposerSignature(&block, p.proposerValidator); err != nil {
		return err
	}

	// 8. 트랜잭션 개수 검증
	if err := ValidateTxCount(block.Transactions); err != nil {
		return err
	}

	// 9. 중복 트랜잭션 검증
	if err := ValidateDuplicateTx(block.Transactions); err != nil {
		return err
	}

	// 10. 중복 UTXO 사용 검증
	if err := ValidateDuplicateUTXO(block.Transactions); err != nil {
		return err
	}

	// 11. 각 트랜잭션 검증
	for _, tx := range block.Transactions {
		if err := p.ValidateTransaction(tx); err != nil {
			return fmt.Errorf("invalid transaction %s: %w", utils.HashToString(tx.ID), err)
		}
	}

	return nil
}

// validateGenesisBlock 제네시스 블록 검증
func (p *BlockChain) validateGenesisBlock(block *Block) error {
	// 높이가 0이어야 함
	if block.Header.Height != 0 {
		return fmt.Errorf("genesis block height must be 0")
	}

	// 이전 해시가 모두 0이어야 함
	var zeroPrevHash prt.Hash
	if block.Header.PrevHash != zeroPrevHash {
		return fmt.Errorf("genesis block prev hash must be zero")
	}

	// 머클 루트 검증
	if err := ValidateMerkleRoot(block); err != nil {
		return err
	}

	// 블록 해시 검증
	if err := ValidateBlockHash(block); err != nil {
		return err
	}

	return nil
}

// ValidateTransaction 트랜잭션 검증
func (p *BlockChain) ValidateTransaction(tx *Transaction) error {
	// 트랜잭션 해시 검증
	if err := ValidateTxHash(tx); err != nil {
		return err
	}

	// Coinbase 트랜잭션 (input이 없음) - 별도 검증
	if len(tx.Inputs) == 0 {
		return p.ValidateCoinbaseTx(tx)
	}

	// Input/Output 밸런스 검증
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

	// input >= output 검증
	if inputSum < outputSum {
		return fmt.Errorf("insufficient input: input=%d, output=%d", inputSum, outputSum)
	}

	// 암묵적 수수료 계산 및 최소 수수료 검증
	implicitFee := inputSum - outputSum
	minFee := p.GetMinFee()
	if implicitFee < minFee {
		return fmt.Errorf("fee too low: got %d, minimum required %d", implicitFee, minFee)
	}

	// 서명 검증
	if err := p.ValidateAllTxSignatures(tx); err != nil {
		return fmt.Errorf("signature validation failed: %w", err)
	}

	return nil
}

// ValidateCoinbaseTx Coinbase 트랜잭션 검증
func (p *BlockChain) ValidateCoinbaseTx(tx *Transaction) error {
	// Coinbase TX는 Input이 없어야 함
	if len(tx.Inputs) != 0 {
		return fmt.Errorf("coinbase tx must have no inputs")
	}

	// Output이 최소 1개 이상 있어야 함
	if len(tx.Outputs) == 0 {
		return fmt.Errorf("coinbase tx must have at least one output")
	}

	// Output 금액 합계가 양수여야 함
	var totalOutput uint64
	for _, output := range tx.Outputs {
		totalOutput += output.Amount
	}

	if totalOutput == 0 {
		return fmt.Errorf("coinbase tx output amount must be positive")
	}

	return nil
}

// CalculateTxFee 트랜잭션의 암묵적 수수료 계산
func (p *BlockChain) CalculateTxFee(tx *Transaction) (uint64, error) {
	// Coinbase TX는 수수료 없음
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

// ValidateTxHash 트랜잭션 해시 검증
// TX ID는 서명 전에 계산되므로, 검증 시에도 서명을 제외하고 해시를 계산해야 함
func ValidateTxHash(tx *Transaction) error {
	storedHash := tx.ID
	tx.ID = prt.Hash{}

	// GOB 역직렬화 후 빈 슬라이스가 nil로 바뀌는 문제 해결
	// TX 생성 시 빈 슬라이스를 사용했으므로, 검증 시에도 nil을 빈 슬라이스로 정규화
	savedData := tx.Data
	savedInputs := tx.Inputs
	if tx.Data == nil {
		tx.Data = []byte{}
	}
	if tx.Inputs == nil {
		tx.Inputs = []*TxInput{}
	}

	// 서명을 임시로 백업하고 제거 (TX ID는 서명 전에 계산됨)
	savedSignatures := make([]prt.Signature, len(tx.Inputs))
	for i, input := range tx.Inputs {
		savedSignatures[i] = input.Signature
		input.Signature = prt.Signature{}
	}

	calculatedHash := utils.Hash(tx)

	// 서명 및 데이터 복원
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

// ValidateTxInputSignature 트랜잭션 입력 서명 검증
func ValidateTxInputSignature(tx *Transaction, input *TxInput, utxo *UTXO) error {
	// 공개키가 UTXO 소유자의 주소와 일치하는지 확인
	if len(input.PublicKey) == 0 {
		return fmt.Errorf("public key is empty")
	}

	// 공개키에서 주소 파생
	publicKey, err := crypto.BytesToPublicKey(input.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	derivedAddress, err := crypto.PublicKeyToAddress(publicKey)
	if err != nil {
		return fmt.Errorf("failed to derive address: %w", err)
	}

	// 주소가 UTXO 소유자와 일치하는지 확인
	if derivedAddress != utxo.TxOut.Address {
		return fmt.Errorf("public key does not match UTXO owner")
	}

	// 서명 검증을 위한 데이터 생성 (저장된 TX ID 사용)
	// 서명은 TX ID를 기준으로 생성되므로, 새로 해시를 계산하면 안됨
	txHashBytes := utils.HashToBytes(tx.ID)

	// 서명 검증
	valid := crypto.VerifySignature(publicKey, txHashBytes, input.Signature)
	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// ValidateAllTxSignatures 트랜잭션의 모든 입력 서명 검증
func (p *BlockChain) ValidateAllTxSignatures(tx *Transaction) error {
	// 제네시스 트랜잭션은 서명 검증 생략
	if len(tx.Inputs) == 0 {
		return nil
	}

	for i, input := range tx.Inputs {
		// UTXO 조회
		utxoKey := utils.GetUtxoKey(input.TxID, int(input.OutputIndex))
		utxoBytes, err := p.db.Get(utxoKey, nil)
		if err != nil {
			return fmt.Errorf("UTXO not found for input %d: %w", i, err)
		}

		var utxo UTXO
		if err := utils.DeserializeData(utxoBytes, &utxo, utils.SerializationFormatGob); err != nil {
			return fmt.Errorf("failed to deserialize UTXO: %w", err)
		}

		// 서명 검증
		if err := ValidateTxInputSignature(tx, input, &utxo); err != nil {
			return fmt.Errorf("signature validation failed for input %d: %w", i, err)
		}
	}

	return nil
}
