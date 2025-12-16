package consensus

import (
	prt "github.com/abcfe/abcfe-node/protocol"
)

// ValidatorInfo validator interface
type ValidatorInfo interface {
	GetAddress() prt.Address
	GetPubKey() []byte
	GetVotingPower() uint64
	GetActiveStat() bool
}

// Vote vote information
type Vote struct {
	Height     uint64        `json:"height"`
	Round      uint32        `json:"round"`
	Type       VoteType      `json:"type"`
	BlockHash  prt.Hash      `json:"blockHash"`
	VoterID    prt.Address   `json:"voterId"`
	Signature  prt.Signature `json:"signature"`
	Timestamp  int64         `json:"timestamp"`
}

// VoteType vote type
type VoteType uint8

const (
	VoteTypePrevote   VoteType = iota // Prevote
	VoteTypePrecommit                 // Precommit
)

// VoteSet vote set
type VoteSet struct {
	Height     uint64
	Round      uint32
	Type       VoteType
	Votes      map[string]*Vote // key: voter address string
	VotedPower uint64           // Sum of voted voting power
}

// NewVoteSet creates a new vote set
func NewVoteSet(height uint64, round uint32, voteType VoteType) *VoteSet {
	return &VoteSet{
		Height:     height,
		Round:      round,
		Type:       voteType,
		Votes:      make(map[string]*Vote),
		VotedPower: 0,
	}
}

// AddVote adds a vote
func (vs *VoteSet) AddVote(vote *Vote, votingPower uint64) bool {
	if vote.Height != vs.Height || vote.Round != vs.Round || vote.Type != vs.Type {
		return false
	}

	key := string(vote.VoterID[:])
	if _, exists := vs.Votes[key]; exists {
		return false // Already voted
	}

	vs.Votes[key] = vote
	vs.VotedPower += votingPower
	return true
}

// HasTwoThirdsMajority checks if 2/3 majority reached
func (vs *VoteSet) HasTwoThirdsMajority(totalPower uint64) bool {
	return vs.VotedPower*3 > totalPower*2
}
