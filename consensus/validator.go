package consensus

import (
	"fmt"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// Validator information
type Validator struct {
	Address     prt.Address `json:"address"`
	PublicKey   []byte      `json:"publicKey"`
	VotingPower uint64      `json:"votingPower"` // Based on staked amount
	IsActive    bool        `json:"isActive"`
}

// ValidatorSet list of validators
type ValidatorSet struct {
	Validators       map[string]*Validator `json:"validators"` // key: address string
	TotalVotingPower uint64                `json:"totalVotingPower"`
}

// NewValidatorSet creates a new validator set
func NewValidatorSet() *ValidatorSet {
	return &ValidatorSet{
		Validators:       make(map[string]*Validator),
		TotalVotingPower: 0,
	}
}

// Validator interface implementation
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

// SignBlock signs a block
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

// ValidateBlockSignature validates block signature
func (v *Validator) ValidateBlockSignature(blockHash prt.Hash, sig prt.Signature) bool {
	publicKey, err := crypto.BytesToPublicKey(v.PublicKey)
	if err != nil {
		fmt.Printf("[DEBUG] ValidateBlockSignature: failed to parse public key for validator %s: %v\n", utils.AddressToString(v.Address), err)
		fmt.Printf("[DEBUG] PublicKey bytes (hex): %x\n", v.PublicKey)
		fmt.Printf("[DEBUG] PublicKey len: %d\n", len(v.PublicKey))
		return false
	}

	hashBytes := utils.HashToBytes(blockHash)
	result := crypto.VerifySignature(publicKey, hashBytes, sig)
	if !result {
		// Find actual signature length (remove trailing zeros)
		sigLen := len(sig)
		for sigLen > 0 && sig[sigLen-1] == 0 {
			sigLen--
		}
		fmt.Printf("[DEBUG] ValidateBlockSignature FAILED:\n")
		fmt.Printf("[DEBUG]   Validator: %s\n", utils.AddressToString(v.Address))
		fmt.Printf("[DEBUG]   BlockHash: %s\n", utils.HashToString(blockHash))
		fmt.Printf("[DEBUG]   Signature (full, len=%d, actual=%d): %x\n", len(sig), sigLen, sig[:sigLen])
		fmt.Printf("[DEBUG]   PublicKey (full, len=%d): %x\n", len(v.PublicKey), v.PublicKey)
	}
	return result
}

// AddValidator adds a validator
func (vs *ValidatorSet) AddValidator(validator *Validator) {
	addrStr := utils.AddressToString(validator.Address)
	vs.Validators[addrStr] = validator
	vs.TotalVotingPower += validator.VotingPower
}

// RemoveValidator removes a validator
func (vs *ValidatorSet) RemoveValidator(address prt.Address) {
	addrStr := utils.AddressToString(address)
	if v, exists := vs.Validators[addrStr]; exists {
		vs.TotalVotingPower -= v.VotingPower
		delete(vs.Validators, addrStr)
	}
}

// GetValidator retrieves a validator
func (vs *ValidatorSet) GetValidator(address prt.Address) *Validator {
	addrStr := utils.AddressToString(address)
	return vs.Validators[addrStr]
}

// GetActiveValidators returns list of active validators
func (vs *ValidatorSet) GetActiveValidators() []*Validator {
	var active []*Validator
	for _, v := range vs.Validators {
		if v.IsActive {
			active = append(active, v)
		}
	}
	return active
}

// UpdateFromStakerSet updates validator set from staker set
func (vs *ValidatorSet) UpdateFromStakerSet(stakerSet *StakerSet, minStake uint64) {
	// Initialize existing validators
	vs.Validators = make(map[string]*Validator)
	vs.TotalVotingPower = 0

	for addrStr, staker := range stakerSet.Stakers {
		// Validator only if staked amount is above minimum
		if staker.IsActive && staker.Amount >= minStake {
			vs.Validators[addrStr] = &Validator{
				Address:     staker.Address,
				PublicKey:   staker.PublicKey, // Copy staker's public key
				VotingPower: staker.Amount,
				IsActive:    true,
			}
			vs.TotalVotingPower += staker.Amount
		}
	}
}

// SaveValidatorSet saves validator set to DB
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

// LoadValidatorSet loads validator set from DB
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
