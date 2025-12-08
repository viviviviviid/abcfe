package consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

// ProposerSelector 제안자 선택기
type ProposerSelector struct {
	ValidatorSet *ValidatorSet
}

// NewProposerSelector 새 제안자 선택기 생성
func NewProposerSelector(vs *ValidatorSet) *ProposerSelector {
	return &ProposerSelector{
		ValidatorSet: vs,
	}
}

// SelectProposer 라운드 로빈 방식으로 제안자 선택
// height와 round를 기반으로 결정적으로 선택
func (ps *ProposerSelector) SelectProposer(height uint64, round uint32) *Validator {
	validators := ps.ValidatorSet.GetActiveValidators()
	if len(validators) == 0 {
		return nil
	}

	// 주소 기준으로 정렬 (결정적 순서)
	sort.Slice(validators, func(i, j int) bool {
		return utils.AddressToString(validators[i].Address) < utils.AddressToString(validators[j].Address)
	})

	// 라운드 로빈
	index := (height + uint64(round)) % uint64(len(validators))
	return validators[index]
}

// SelectProposerWeighted 가중치 기반 제안자 선택 (VRF 대신 해시 기반)
// 스테이킹 금액에 비례한 확률로 선택
func (ps *ProposerSelector) SelectProposerWeighted(height uint64, prevBlockHash prt.Hash) *Validator {
	validators := ps.ValidatorSet.GetActiveValidators()
	if len(validators) == 0 {
		return nil
	}

	// 결정적 시드 생성 (height + prevBlockHash)
	seed := make([]byte, 8+32)
	binary.BigEndian.PutUint64(seed[:8], height)
	copy(seed[8:], prevBlockHash[:])
	hash := sha256.Sum256(seed)

	// 해시를 0~TotalVotingPower 범위의 숫자로 변환
	hashNum := binary.BigEndian.Uint64(hash[:8])
	target := hashNum % ps.ValidatorSet.TotalVotingPower

	// 누적 가중치로 선택
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

// IsProposer 주어진 주소가 해당 높이의 제안자인지 확인
func (ps *ProposerSelector) IsProposer(address prt.Address, height uint64, round uint32) bool {
	proposer := ps.SelectProposer(height, round)
	if proposer == nil {
		return false
	}
	return proposer.Address == address
}

// GetNextProposers 다음 N개 블록의 제안자 목록
func (ps *ProposerSelector) GetNextProposers(startHeight uint64, count int) []*Validator {
	proposers := make([]*Validator, count)
	for i := 0; i < count; i++ {
		proposers[i] = ps.SelectProposer(startHeight+uint64(i), 0)
	}
	return proposers
}
