package consensus

import (
	"fmt"
	"time"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// Staker information
type Staker struct {
	Address   prt.Address `json:"address"`
	PublicKey []byte      `json:"publicKey"` // Public key for validator identity verification
	Amount    uint64      `json:"amount"`
	StartTime int64       `json:"startTime"` // Staking start time (unix)
	EndTime   int64       `json:"endTime"`   // Expected unstaking time (0 if indefinite)
	IsActive  bool        `json:"isActive"`
}

// StakerSet list of all stakers
type StakerSet struct {
	Stakers     map[string]*Staker `json:"stakers"` // key: address string
	TotalStaked uint64             `json:"totalStaked"`
}

// NewStakerSet creates a new staker set
func NewStakerSet() *StakerSet {
	return &StakerSet{
		Stakers:     make(map[string]*Staker),
		TotalStaked: 0,
	}
}

// AddStaker adds a staker
func (s *StakerSet) AddStaker(address prt.Address, amount uint64, publicKey []byte) error {
	addrStr := utils.AddressToString(address)

	if existing, exists := s.Stakers[addrStr]; exists {
		// Add amount if existing staker
		existing.Amount += amount
		// Update public key if missing
		if len(existing.PublicKey) == 0 && len(publicKey) > 0 {
			existing.PublicKey = publicKey
		}
		s.TotalStaked += amount
		return nil
	}

	// New staker
	s.Stakers[addrStr] = &Staker{
		Address:   address,
		PublicKey: publicKey,
		Amount:    amount,
		StartTime: time.Now().Unix(),
		EndTime:   0,
		IsActive:  true,
	}
	s.TotalStaked += amount

	return nil
}

// RemoveStaker removes a staker (unstaking)
func (s *StakerSet) RemoveStaker(address prt.Address, amount uint64) error {
	addrStr := utils.AddressToString(address)

	staker, exists := s.Stakers[addrStr]
	if !exists {
		return fmt.Errorf("staker not found: %s", addrStr)
	}

	if staker.Amount < amount {
		return fmt.Errorf("insufficient staked amount: have %d, want %d", staker.Amount, amount)
	}

	staker.Amount -= amount
	s.TotalStaked -= amount

	// Deactivate if amount is 0
	if staker.Amount == 0 {
		staker.IsActive = false
	}

	return nil
}

// GetStaker retrieves a staker
func (s *StakerSet) GetStaker(address prt.Address) *Staker {
	addrStr := utils.AddressToString(address)
	return s.Stakers[addrStr]
}

// GetActiveStakers returns list of active stakers
func (s *StakerSet) GetActiveStakers() []*Staker {
	var active []*Staker
	for _, staker := range s.Stakers {
		if staker.IsActive && staker.Amount > 0 {
			active = append(active, staker)
		}
	}
	return active
}

// SaveStakerSet saves staker set to DB
func SaveStakerSet(db *leveldb.DB, stakerSet *StakerSet) error {
	key := []byte(prt.PrefixStakerInfo)
	data, err := utils.SerializeData(stakerSet, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize staker set: %w", err)
	}

	if err := db.Put(key, data, nil); err != nil {
		return fmt.Errorf("failed to save staker set: %w", err)
	}

	return nil
}

// LoadStakerSet loads staker set from DB
func LoadStakerSet(db *leveldb.DB) (*StakerSet, error) {
	key := []byte(prt.PrefixStakerInfo)
	data, err := db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return NewStakerSet(), nil
		}
		return nil, fmt.Errorf("failed to load staker set: %w", err)
	}

	var stakerSet StakerSet
	if err := utils.DeserializeData(data, &stakerSet, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize staker set: %w", err)
	}

	return &stakerSet, nil
}

// GetStakers returns list of staker addresses (backward compatibility)
func GetStakers(db *leveldb.DB) ([]string, error) {
	stakerSet, err := LoadStakerSet(db)
	if err != nil {
		return nil, err
	}

	var addresses []string
	for addr := range stakerSet.Stakers {
		addresses = append(addresses, addr)
	}

	return addresses, nil
}
