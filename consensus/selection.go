package consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

// ProposerSelector proposer selector
type ProposerSelector struct {
	ValidatorSet *ValidatorSet
}

// NewProposerSelector creates new proposer selector
func NewProposerSelector(vs *ValidatorSet) *ProposerSelector {
	return &ProposerSelector{
		ValidatorSet: vs,
	}
}

// SelectProposer selects proposer using round-robin method
// BFT mode: Determine proposer by height + round (Select next proposer by increasing round on timeout)
func (ps *ProposerSelector) SelectProposer(height uint64, round uint32) *Validator {
	validators := ps.ValidatorSet.GetActiveValidators()
	if len(validators) == 0 {
		return nil
	}

	// Sort by address (deterministic order)
	sort.Slice(validators, func(i, j int) bool {
		return utils.AddressToString(validators[i].Address) < utils.AddressToString(validators[j].Address)
	})

	// BFT round-robin: Use height + round (Change to next proposer on timeout)
	index := (height + uint64(round)) % uint64(len(validators))
	return validators[index]
}

// SelectProposerWeighted Weighted proposer selection (Hash-based instead of VRF)
// Select with probability proportional to staking amount
func (ps *ProposerSelector) SelectProposerWeighted(height uint64, prevBlockHash prt.Hash) *Validator {
	validators := ps.ValidatorSet.GetActiveValidators()
	if len(validators) == 0 {
		return nil
	}

	// Generate deterministic seed (height + prevBlockHash)
	seed := make([]byte, 8+32)
	binary.BigEndian.PutUint64(seed[:8], height)
	copy(seed[8:], prevBlockHash[:])
	hash := sha256.Sum256(seed)

	// Convert hash to number in 0~TotalVotingPower range
	hashNum := binary.BigEndian.Uint64(hash[:8])
	target := hashNum % ps.ValidatorSet.TotalVotingPower

	// Select by cumulative weight
	var cumulative uint64
	for _, v := range validators {
		cumulative += v.VotingPower
		if target < cumulative {
			return v
		}
	}

	// fallback
	return validators[0]
}

// IsProposer checks if given address is proposer for the height
func (ps *ProposerSelector) IsProposer(address prt.Address, height uint64, round uint32) bool {
	proposer := ps.SelectProposer(height, round)
	if proposer == nil {
		return false
	}
	return proposer.Address == address
}

// GetNextProposers gets list of proposers for next N blocks
func (ps *ProposerSelector) GetNextProposers(startHeight uint64, count int) []*Validator {
	proposers := make([]*Validator, count)
	for i := 0; i < count; i++ {
		proposers[i] = ps.SelectProposer(startHeight+uint64(i), 0)
	}
	return proposers
}
