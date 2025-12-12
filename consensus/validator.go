package consensus

import (
	"fmt"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// Validator 검증자 정보
type Validator struct {
	Address     prt.Address `json:"address"`
	PublicKey   []byte      `json:"publicKey"`
	VotingPower uint64      `json:"votingPower"` // 스테이킹 금액 기반
	IsActive    bool        `json:"isActive"`
}

// ValidatorSet 검증자 목록
type ValidatorSet struct {
	Validators       map[string]*Validator `json:"validators"` // key: address string
	TotalVotingPower uint64                `json:"totalVotingPower"`
}

// NewValidatorSet 새 검증자셋 생성
func NewValidatorSet() *ValidatorSet {
	return &ValidatorSet{
		Validators:       make(map[string]*Validator),
		TotalVotingPower: 0,
	}
}

// Validator 인터페이스 구현
func (v *Validator) GetAddress() prt.Address {
	return v.Address
}

func (v *Validator) GetPubKey() []byte {
	return v.PublicKey
}

func (v *Validator) GetVotingPower() uint64 {
	return v.VotingPower
}

func (v *Validator) GetActiveStat() bool {
	return v.IsActive
}

// SignBlock 블록 서명
func (v *Validator) SignBlock(blockHash prt.Hash, privateKeyBytes []byte) (prt.Signature, error) {
	privateKey, err := crypto.BytesToPrivateKey(privateKeyBytes)
	if err != nil {
		return prt.Signature{}, fmt.Errorf("failed to parse private key: %w", err)
	}

	hashBytes := utils.HashToBytes(blockHash)
	sig, err := crypto.SignData(privateKey, hashBytes)
	if err != nil {
		return prt.Signature{}, fmt.Errorf("failed to sign block: %w", err)
	}

	return sig, nil
}

// ValidateBlockSignature 블록 서명 검증
func (v *Validator) ValidateBlockSignature(blockHash prt.Hash, sig prt.Signature) bool {
	publicKey, err := crypto.BytesToPublicKey(v.PublicKey)
	if err != nil {
		return false
	}

	hashBytes := utils.HashToBytes(blockHash)
	return crypto.VerifySignature(publicKey, hashBytes, sig)
}

// AddValidator 검증자 추가
func (vs *ValidatorSet) AddValidator(validator *Validator) {
	addrStr := utils.AddressToString(validator.Address)
	vs.Validators[addrStr] = validator
	vs.TotalVotingPower += validator.VotingPower
}

// RemoveValidator 검증자 제거
func (vs *ValidatorSet) RemoveValidator(address prt.Address) {
	addrStr := utils.AddressToString(address)
	if v, exists := vs.Validators[addrStr]; exists {
		vs.TotalVotingPower -= v.VotingPower
		delete(vs.Validators, addrStr)
	}
}

// GetValidator 검증자 조회
func (vs *ValidatorSet) GetValidator(address prt.Address) *Validator {
	addrStr := utils.AddressToString(address)
	return vs.Validators[addrStr]
}

// GetActiveValidators 활성 검증자 목록
func (vs *ValidatorSet) GetActiveValidators() []*Validator {
	var active []*Validator
	for _, v := range vs.Validators {
		if v.IsActive {
			active = append(active, v)
		}
	}
	return active
}

// UpdateFromStakerSet 스테이커셋에서 검증자셋 업데이트
func (vs *ValidatorSet) UpdateFromStakerSet(stakerSet *StakerSet, minStake uint64) {
	// 기존 검증자 초기화
	vs.Validators = make(map[string]*Validator)
	vs.TotalVotingPower = 0

	for addrStr, staker := range stakerSet.Stakers {
		// 최소 스테이킹 금액 이상인 경우만 검증자
		if staker.IsActive && staker.Amount >= minStake {
			vs.Validators[addrStr] = &Validator{
				Address:     staker.Address,
				PublicKey:   staker.PublicKey, // 스테이커의 공개키 복사
				VotingPower: staker.Amount,
				IsActive:    true,
			}
			vs.TotalVotingPower += staker.Amount
		}
	}
}

// SaveValidatorSet DB에 검증자셋 저장
func SaveValidatorSet(db *leveldb.DB, validatorSet *ValidatorSet) error {
	key := []byte("consensus:validators")
	data, err := utils.SerializeData(validatorSet, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize validator set: %w", err)
	}

	if err := db.Put(key, data, nil); err != nil {
		return fmt.Errorf("failed to save validator set: %w", err)
	}

	return nil
}

// LoadValidatorSet DB에서 검증자셋 로드
func LoadValidatorSet(db *leveldb.DB) (*ValidatorSet, error) {
	key := []byte("consensus:validators")
	data, err := db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return NewValidatorSet(), nil
		}
		return nil, fmt.Errorf("failed to load validator set: %w", err)
	}

	var validatorSet ValidatorSet
	if err := utils.DeserializeData(data, &validatorSet, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize validator set: %w", err)
	}

	return &validatorSet, nil
}
