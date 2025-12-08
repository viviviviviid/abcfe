package consensus

import (
	prt "github.com/abcfe/abcfe-node/protocol"
)

// ValidatorInfo 검증자 인터페이스
type ValidatorInfo interface {
	GetAddress() prt.Address
	GetPubKey() []byte
	GetVotingPower() uint64
	GetActiveStat() bool
}

// Vote 투표 정보
type Vote struct {
	Height     uint64        `json:"height"`
	Round      uint32        `json:"round"`
	Type       VoteType      `json:"type"`
	BlockHash  prt.Hash      `json:"blockHash"`
	VoterID    prt.Address   `json:"voterId"`
	Signature  prt.Signature `json:"signature"`
	Timestamp  int64         `json:"timestamp"`
}

// VoteType 투표 유형
type VoteType uint8

const (
	VoteTypePrevote   VoteType = iota // 사전 투표
	VoteTypePrecommit                 // 사전 커밋
)

// VoteSet 투표 집합
type VoteSet struct {
	Height     uint64
	Round      uint32
	Type       VoteType
	Votes      map[string]*Vote // key: voter address string
	VotedPower uint64           // 투표한 voting power 합
}

// NewVoteSet 새 투표셋 생성
func NewVoteSet(height uint64, round uint32, voteType VoteType) *VoteSet {
	return &VoteSet{
		Height:     height,
		Round:      round,
		Type:       voteType,
		Votes:      make(map[string]*Vote),
		VotedPower: 0,
	}
}

// AddVote 투표 추가
func (vs *VoteSet) AddVote(vote *Vote, votingPower uint64) bool {
	if vote.Height != vs.Height || vote.Round != vs.Round || vote.Type != vs.Type {
		return false
	}

	key := string(vote.VoterID[:])
	if _, exists := vs.Votes[key]; exists {
		return false // 이미 투표함
	}

	vs.Votes[key] = vote
	vs.VotedPower += votingPower
	return true
}

// HasTwoThirdsMajority 2/3 이상 동의 여부
func (vs *VoteSet) HasTwoThirdsMajority(totalPower uint64) bool {
	return vs.VotedPower*3 > totalPower*2
}
