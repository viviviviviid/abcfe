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

// Consensus constants
const (
	MinStakeAmount     = 1000   // Minimum stake amount
	MaxValidators      = 100    // Maximum validators
	BlockProduceTimeMs = 3000   // Block production interval (milliseconds)
	RoundTimeoutMs     = 20000  // Round timeout (milliseconds)
)

// ConsensusState consensus state
type ConsensusState string

const (
	StateIdle       ConsensusState = "IDLE"
	StateProposing  ConsensusState = "PROPOSING"
	StateVoting     ConsensusState = "VOTING"
	StateCommitting ConsensusState = "COMMITTING"
)

// Consensus consensus engine
type Consensus struct {
	mu sync.RWMutex

	stop chan struct{}
	Conf conf.Config
	DB   *leveldb.DB

	// Consensus state
	State        ConsensusState
	CurrentHeight uint64
	CurrentRound  uint32

	// Validator/Staker management
	StakerSet    *StakerSet
	ValidatorSet *ValidatorSet
	Selector     *ProposerSelector

	// If current node is validator
	LocalValidator *Validator
	LocalProposer  *Proposer
}

// NewConsensus creates new consensus engine
func NewConsensus(cfg *conf.Config, db *leveldb.DB) (*Consensus, error) {
	// Load StakerSet
	stakerSet, err := LoadStakerSet(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load staker set: %w", err)
	}

	// Load or create ValidatorSet
	validatorSet, err := LoadValidatorSet(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load validator set: %w", err)
	}

	// Load genesis validators (PoA)
	if len(cfg.Validators.List) > 0 {
		logger.Info("[Consensus] Loading genesis validators from config: ", len(cfg.Validators.List))
		if err := loadGenesisValidators(validatorSet, cfg.Validators.List); err != nil {
			return nil, fmt.Errorf("failed to load genesis validators: %w", err)
		}
		logger.Info("[Consensus] Genesis validators loaded, total voting power: ", validatorSet.TotalVotingPower)
	} else {
		// If no genesis validators, update validators based on stakers
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

// loadGenesisValidators loads genesis validators from config
func loadGenesisValidators(vs *ValidatorSet, validators []conf.ValidatorConfig) error {
	vs.Validators = make(map[string]*Validator)
	vs.TotalVotingPower = 0

	for _, v := range validators {
		// Parse address
		addr, err := utils.StringToAddress(v.Address)
		if err != nil {
			return fmt.Errorf("invalid validator address %s: %w", v.Address, err)
		}

		// Parse public key (hex encoding)
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

// RegisterValidator registers local node as validator
func (c *Consensus) RegisterValidator(address prt.Address, publicKey []byte, privateKey []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find matching validator in genesis validator list
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

// Stake staking
func (c *Consensus) Stake(address prt.Address, amount uint64, publicKey []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.StakerSet.AddStaker(address, amount, publicKey); err != nil {
		return err
	}

	// Update ValidatorSet
	c.ValidatorSet.UpdateFromStakerSet(c.StakerSet, MinStakeAmount)

	// Save to DB
	if err := SaveStakerSet(c.DB, c.StakerSet); err != nil {
		return fmt.Errorf("failed to save staker set: %w", err)
	}

	if err := SaveValidatorSet(c.DB, c.ValidatorSet); err != nil {
		return fmt.Errorf("failed to save validator set: %w", err)
	}

	return nil
}

// Unstake unstaking
func (c *Consensus) Unstake(address prt.Address, amount uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.StakerSet.RemoveStaker(address, amount); err != nil {
		return err
	}

	// Update ValidatorSet
	c.ValidatorSet.UpdateFromStakerSet(c.StakerSet, MinStakeAmount)

	// Save to DB
	if err := SaveStakerSet(c.DB, c.StakerSet); err != nil {
		return fmt.Errorf("failed to save staker set: %w", err)
	}

	if err := SaveValidatorSet(c.DB, c.ValidatorSet); err != nil {
		return fmt.Errorf("failed to save validator set: %w", err)
	}

	return nil
}

// GetCurrentProposer gets proposer of current block
func (c *Consensus) GetCurrentProposer() *Validator {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Selector.SelectProposer(c.CurrentHeight, c.CurrentRound)
}

// IsLocalProposer checks if local node is current proposer
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

// UpdateHeight updates height
func (c *Consensus) UpdateHeight(height uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.CurrentHeight = height
	c.CurrentRound = 0
	c.State = StateIdle
}

// IncrementRound increments round
func (c *Consensus) IncrementRound() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.CurrentRound++
}

// GetValidatorCount gets validator count
func (c *Consensus) GetValidatorCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.ValidatorSet.GetActiveValidators())
}

// GetTotalStaked gets total staked amount
func (c *Consensus) GetTotalStaked() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.StakerSet.TotalStaked
}

// Stop stops consensus engine
func (c *Consensus) Stop() {
	close(c.stop)
}

// ValidateProposerSignature validates proposer signature (implements core.ProposerValidator interface)
func (c *Consensus) ValidateProposerSignature(proposer prt.Address, blockHash prt.Hash, signature prt.Signature) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Find validator
	addrStr := utils.AddressToString(proposer)
	validator, exists := c.ValidatorSet.Validators[addrStr]
	if !exists || !validator.IsActive {
		return false
	}

	// Verify signature
	return validator.ValidateBlockSignature(blockHash, signature)
}

// IsValidProposer checks if valid proposer for the height (implements core.ProposerValidator interface)
// In BFT mode, proposer changes by round, so only check if in validator list
// Round-based proposer validation is done in HandleProposal in consensus/engine.go
func (c *Consensus) IsValidProposer(proposer prt.Address, height uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check if in validator list
	addrStr := utils.AddressToString(proposer)
	validator, exists := c.ValidatorSet.Validators[addrStr]
	if !exists || !validator.IsActive {
		return false
	}

	// BFT mode: Valid if in validator list
	// (Proposer for specific round is already verified in HandleProposal)
	return true
}
