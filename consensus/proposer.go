package consensus

import (
	"fmt"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

// BlockProposal 블록 제안 정보
type BlockProposal struct {
	Height     uint64        `json:"height"`
	Round      uint32        `json:"round"`
	BlockHash  prt.Hash      `json:"blockHash"`
	ProposerID prt.Address   `json:"proposerId"`
	Signature  prt.Signature `json:"signature"`
	Timestamp  int64         `json:"timestamp"`
}

// Proposer 블록 제안자
type Proposer struct {
	Validator    *Validator
	PrivateKey   []byte // 서명을 위한 개인키
	CurrentRound uint32
}

// NewProposer 새 제안자 생성
func NewProposer(validator *Validator, privateKey []byte) *Proposer {
	return &Proposer{
		Validator:    validator,
		PrivateKey:   privateKey,
		CurrentRound: 0,
	}
}

// ProposeBlock 블록 제안 생성
func (p *Proposer) ProposeBlock(height uint64, blockHash prt.Hash, timestamp int64) (*BlockProposal, error) {
	if p.Validator == nil {
		return nil, fmt.Errorf("validator not set")
	}

	// 블록 해시에 서명
	sig, err := p.signBlockHash(blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign block: %w", err)
	}

	proposal := &BlockProposal{
		Height:     height,
		Round:      p.CurrentRound,
		BlockHash:  blockHash,
		ProposerID: p.Validator.Address,
		Signature:  sig,
		Timestamp:  timestamp,
	}

	return proposal, nil
}

// signBlockHash 블록 해시에 서명
func (p *Proposer) signBlockHash(blockHash prt.Hash) (prt.Signature, error) {
	privateKey, err := crypto.BytesToPrivateKey(p.PrivateKey)
	if err != nil {
		return prt.Signature{}, fmt.Errorf("failed to parse private key: %w", err)
	}

	hashBytes := utils.HashToBytes(blockHash)
	return crypto.SignData(privateKey, hashBytes)
}

// VerifyProposal 블록 제안 검증
func VerifyProposal(proposal *BlockProposal, validator *Validator) bool {
	if proposal == nil || validator == nil {
		return false
	}

	// 제안자 주소 확인
	if proposal.ProposerID != validator.Address {
		return false
	}

	// 서명 검증
	return validator.ValidateBlockSignature(proposal.BlockHash, proposal.Signature)
}

// IncrementRound 라운드 증가
func (p *Proposer) IncrementRound() {
	p.CurrentRound++
}

// ResetRound 라운드 리셋
func (p *Proposer) ResetRound() {
	p.CurrentRound = 0
}
