package consensus

import (
	"fmt"
	"sync"

	conf "github.com/abcfe/abcfe-node/config"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// 컨센서스 상수
const (
	MinStakeAmount     = 1000   // 최소 스테이킹 금액
	MaxValidators      = 100    // 최대 검증자 수
	BlockProduceTimeMs = 3000   // 블록 생성 간격 (밀리초)
	RoundTimeoutMs     = 10000  // 라운드 타임아웃 (밀리초)
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

	// 스테이커 기반으로 검증자 업데이트
	validatorSet.UpdateFromStakerSet(stakerSet, MinStakeAmount)

	selector := NewProposerSelector(validatorSet)

	consensus := &Consensus{
		stop:         make(chan struct{}),
		Conf:         *cfg,
		DB:           db,
		State:        StateIdle,
		CurrentHeight: 0,
		CurrentRound:  0,
		StakerSet:    stakerSet,
		ValidatorSet: validatorSet,
		Selector:     selector,
	}

	return consensus, nil
}

// RegisterValidator 로컬 노드를 검증자로 등록
func (c *Consensus) RegisterValidator(address prt.Address, publicKey []byte, privateKey []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	validator := &Validator{
		Address:     address,
		PublicKey:   publicKey,
		VotingPower: 0, // 스테이킹 후 업데이트됨
		IsActive:    true,
	}

	c.LocalValidator = validator
	c.LocalProposer = NewProposer(validator, privateKey)

	return nil
}

// Stake 스테이킹
func (c *Consensus) Stake(address prt.Address, amount uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.StakerSet.AddStaker(address, amount); err != nil {
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
