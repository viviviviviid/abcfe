package consensus

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/common/utils"
	conf "github.com/abcfe/abcfe-node/config"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// 컨센서스 상수
const (
	MinStakeAmount     = 1000   // 최소 스테이킹 금액
	MaxValidators      = 100    // 최대 검증자 수
	BlockProduceTimeMs = 15000  // 블록 생성 간격 (밀리초)
	RoundTimeoutMs     = 15000  // 라운드 타임아웃 (밀리초)
)

// ConsensusState 컨센서스 상태
type ConsensusState string

const (
	StateIdle       ConsensusState = "IDLE"
	StateProposing  ConsensusState = "PROPOSING"
	StateVoting     ConsensusState = "VOTING"
	StateCommitting ConsensusState = "COMMITTING"
)

// Consensus 컨센서스 엔진
type Consensus struct {
	mu sync.RWMutex

	stop chan struct{}
	Conf conf.Config
	DB   *leveldb.DB

	// 컨센서스 상태
	State        ConsensusState
	CurrentHeight uint64
	CurrentRound  uint32

	// 검증자/스테이커 관리
	StakerSet    *StakerSet
	ValidatorSet *ValidatorSet
	Selector     *ProposerSelector

	// 현재 노드가 검증자인 경우
	LocalValidator *Validator
	LocalProposer  *Proposer
}

// NewConsensus 새 컨센서스 엔진 생성
func NewConsensus(cfg *conf.Config, db *leveldb.DB) (*Consensus, error) {
	// 스테이커셋 로드
	stakerSet, err := LoadStakerSet(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load staker set: %w", err)
	}

	// 검증자셋 로드 또는 생성
	validatorSet, err := LoadValidatorSet(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load validator set: %w", err)
	}

	// 제네시스 검증자 로드 (PoA)
	if len(cfg.Validators.List) > 0 {
		logger.Info("[Consensus] Loading genesis validators from config: ", len(cfg.Validators.List))
		if err := loadGenesisValidators(validatorSet, cfg.Validators.List); err != nil {
			return nil, fmt.Errorf("failed to load genesis validators: %w", err)
		}
		logger.Info("[Consensus] Genesis validators loaded, total voting power: ", validatorSet.TotalVotingPower)
	} else {
		// 제네시스 검증자가 없으면 스테이커 기반으로 검증자 업데이트
		validatorSet.UpdateFromStakerSet(stakerSet, MinStakeAmount)
	}

	selector := NewProposerSelector(validatorSet)

	consensus := &Consensus{
		stop:          make(chan struct{}),
		Conf:          *cfg,
		DB:            db,
		State:         StateIdle,
		CurrentHeight: 0,
		CurrentRound:  0,
		StakerSet:     stakerSet,
		ValidatorSet:  validatorSet,
		Selector:      selector,
	}

	return consensus, nil
}

// loadGenesisValidators 설정 파일에서 제네시스 검증자 로드
func loadGenesisValidators(vs *ValidatorSet, validators []conf.ValidatorConfig) error {
	vs.Validators = make(map[string]*Validator)
	vs.TotalVotingPower = 0

	for _, v := range validators {
		// 주소 파싱
		addr, err := utils.StringToAddress(v.Address)
		if err != nil {
			return fmt.Errorf("invalid validator address %s: %w", v.Address, err)
		}

		// 공개키 파싱 (hex 인코딩)
		var pubKey []byte
		if v.PublicKey != "" {
			pubKey, err = hex.DecodeString(v.PublicKey)
			if err != nil {
				return fmt.Errorf("invalid validator public key %s: %w", v.PublicKey, err)
			}
		}

		addrStr := utils.AddressToString(addr)
		vs.Validators[addrStr] = &Validator{
			Address:     addr,
			PublicKey:   pubKey,
			VotingPower: v.VotingPower,
			IsActive:    true,
		}
		vs.TotalVotingPower += v.VotingPower

		logger.Info("[Consensus] Added genesis validator: ", addrStr, " power: ", v.VotingPower)
	}

	return nil
}

// RegisterValidator 로컬 노드를 검증자로 등록
func (c *Consensus) RegisterValidator(address prt.Address, publicKey []byte, privateKey []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 제네시스 검증자 목록에서 매칭되는 검증자 찾기
	addrStr := utils.AddressToString(address)
	var votingPower uint64 = 0

	if existingValidator, exists := c.ValidatorSet.Validators[addrStr]; exists {
		votingPower = existingValidator.VotingPower
		logger.Info("[Consensus] Local validator found in genesis validators: ", addrStr, " power: ", votingPower)
	} else {
		logger.Warn("[Consensus] Local validator not found in genesis validators: ", addrStr)
	}

	validator := &Validator{
		Address:     address,
		PublicKey:   publicKey,
		VotingPower: votingPower,
		IsActive:    true,
	}

	c.LocalValidator = validator
	c.LocalProposer = NewProposer(validator, privateKey)

	return nil
}

// Stake 스테이킹
func (c *Consensus) Stake(address prt.Address, amount uint64, publicKey []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.StakerSet.AddStaker(address, amount, publicKey); err != nil {
		return err
	}

	// 검증자셋 업데이트
	c.ValidatorSet.UpdateFromStakerSet(c.StakerSet, MinStakeAmount)

	// DB 저장
	if err := SaveStakerSet(c.DB, c.StakerSet); err != nil {
		return fmt.Errorf("failed to save staker set: %w", err)
	}

	if err := SaveValidatorSet(c.DB, c.ValidatorSet); err != nil {
		return fmt.Errorf("failed to save validator set: %w", err)
	}

	return nil
}

// Unstake 언스테이킹
func (c *Consensus) Unstake(address prt.Address, amount uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.StakerSet.RemoveStaker(address, amount); err != nil {
		return err
	}

	// 검증자셋 업데이트
	c.ValidatorSet.UpdateFromStakerSet(c.StakerSet, MinStakeAmount)

	// DB 저장
	if err := SaveStakerSet(c.DB, c.StakerSet); err != nil {
		return fmt.Errorf("failed to save staker set: %w", err)
	}

	if err := SaveValidatorSet(c.DB, c.ValidatorSet); err != nil {
		return fmt.Errorf("failed to save validator set: %w", err)
	}

	return nil
}

// GetCurrentProposer 현재 블록의 제안자 조회
func (c *Consensus) GetCurrentProposer() *Validator {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Selector.SelectProposer(c.CurrentHeight, c.CurrentRound)
}

// IsLocalProposer 로컬 노드가 현재 제안자인지 확인
func (c *Consensus) IsLocalProposer() bool {
	if c.LocalValidator == nil {
		return false
	}

	proposer := c.GetCurrentProposer()
	if proposer == nil {
		return false
	}

	return proposer.Address == c.LocalValidator.Address
}

// UpdateHeight 높이 업데이트
func (c *Consensus) UpdateHeight(height uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.CurrentHeight = height
	c.CurrentRound = 0
	c.State = StateIdle
}

// IncrementRound 라운드 증가
func (c *Consensus) IncrementRound() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.CurrentRound++
}

// GetValidatorCount 검증자 수 조회
func (c *Consensus) GetValidatorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.ValidatorSet.GetActiveValidators())
}

// GetTotalStaked 전체 스테이킹 금액 조회
func (c *Consensus) GetTotalStaked() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.StakerSet.TotalStaked
}

// Stop 컨센서스 엔진 종료
func (c *Consensus) Stop() {
	close(c.stop)
}

// ValidateProposerSignature 제안자 서명 검증 (core.ProposerValidator 인터페이스 구현)
func (c *Consensus) ValidateProposerSignature(proposer prt.Address, blockHash prt.Hash, signature prt.Signature) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 검증자 찾기
	addrStr := utils.AddressToString(proposer)
	validator, exists := c.ValidatorSet.Validators[addrStr]
	if !exists || !validator.IsActive {
		return false
	}

	// 서명 검증
	return validator.ValidateBlockSignature(blockHash, signature)
}

// IsValidProposer 해당 높이의 유효한 제안자인지 확인 (core.ProposerValidator 인터페이스 구현)
// BFT 모드에서는 라운드에 따라 제안자가 달라지므로, 검증자 목록에 있는지만 확인
// 라운드별 제안자 검증은 consensus/engine.go의 HandleProposal에서 수행
func (c *Consensus) IsValidProposer(proposer prt.Address, height uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 검증자 목록에 있는지 확인
	addrStr := utils.AddressToString(proposer)
	validator, exists := c.ValidatorSet.Validators[addrStr]
	if !exists || !validator.IsActive {
		return false
	}

	// BFT 모드: 검증자 목록에 있으면 유효함
	// (특정 라운드의 제안자인지는 HandleProposal에서 이미 검증됨)
	return true
}
