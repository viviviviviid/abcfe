package consensus

import (
	"fmt"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

// BlockProposal block proposal info
type BlockProposal struct {
	Height     uint64        `json:"height"`
	Round      uint32        `json:"round"`
	BlockHash  prt.Hash      `json:"blockHash"`
	ProposerID prt.Address   `json:"proposerId"`
	Signature  prt.Signature `json:"signature"`
	Timestamp  int64         `json:"timestamp"`
}

// Proposer block proposer
type Proposer struct {
	Validator    *Validator
	PrivateKey   []byte // Private key for signing
	CurrentRound uint32
}

// NewProposer creates new proposer
func NewProposer(validator *Validator, privateKey []byte) *Proposer {
	return &Proposer{
		Validator:    validator,
		PrivateKey:   privateKey,
		CurrentRound: 0,
	}
}

// ProposeBlock creates block proposal
func (p *Proposer) ProposeBlock(height uint64, blockHash prt.Hash, timestamp int64) (*BlockProposal, error) {
	if p.Validator == nil {
		return nil, fmt.Errorf("validator not set")
	}

	// Sign block hash
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

// signBlockHash signs block hash
func (p *Proposer) signBlockHash(blockHash prt.Hash) (prt.Signature, error) {
	privateKey, err := crypto.BytesToPrivateKey(p.PrivateKey)
	if err != nil {
		return prt.Signature{}, fmt.Errorf("failed to parse private key: %w", err)
	}

	hashBytes := utils.HashToBytes(blockHash)
	sig, err := crypto.SignData(privateKey, hashBytes)
	if err != nil {
		return prt.Signature{}, err
	}

	// Check actual signature length
	sigLen := len(sig)
	for sigLen > 0 && sig[sigLen-1] == 0 {
		sigLen--
	}
	fmt.Printf("[DEBUG] signBlockHash: hash=%s sig_len=%d actual_len=%d sig=%x\n",
		utils.HashToString(blockHash), len(sig), sigLen, sig[:sigLen])
	return sig, nil
}

// VerifyProposal verifies block proposal
func VerifyProposal(proposal *BlockProposal, validator *Validator) bool {
	if proposal == nil || validator == nil {
		return false
	}

	// Check proposer address
	if proposal.ProposerID != validator.Address {
		return false
	}

	// Verify signature
	return validator.ValidateBlockSignature(proposal.BlockHash, proposal.Signature)
}

// IncrementRound increments round
func (p *Proposer) IncrementRound() {
	p.CurrentRound++
}

// ResetRound resets round
func (p *Proposer) ResetRound() {
	p.CurrentRound = 0
}
