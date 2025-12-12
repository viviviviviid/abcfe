package consensus

import (
	"fmt"
	"time"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// Staker 스테이커 정보
type Staker struct {
	Address   prt.Address `json:"address"`
	PublicKey []byte      `json:"publicKey"` // 검증자 신원 확인용 공개키
	Amount    uint64      `json:"amount"`
	StartTime int64       `json:"startTime"` // 스테이킹 시작 시간 (unix)
	EndTime   int64       `json:"endTime"`   // 언스테이킹 예정 시간 (0이면 무기한)
	IsActive  bool        `json:"isActive"`
}

// StakerSet 전체 스테이커 목록
type StakerSet struct {
	Stakers     map[string]*Staker `json:"stakers"` // key: address string
	TotalStaked uint64             `json:"totalStaked"`
}

// NewStakerSet 새 스테이커셋 생성
func NewStakerSet() *StakerSet {
	return &StakerSet{
		Stakers:     make(map[string]*Staker),
		TotalStaked: 0,
	}
}

// AddStaker 스테이커 추가
func (s *StakerSet) AddStaker(address prt.Address, amount uint64, publicKey []byte) error {
	addrStr := utils.AddressToString(address)

	if existing, exists := s.Stakers[addrStr]; exists {
		// 기존 스테이커면 금액 추가
		existing.Amount += amount
		// 공개키가 없으면 업데이트
		if len(existing.PublicKey) == 0 && len(publicKey) > 0 {
			existing.PublicKey = publicKey
		}
		s.TotalStaked += amount
		return nil
	}

	// 새 스테이커
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

// RemoveStaker 스테이커 제거 (언스테이킹)
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

	// 금액이 0이면 비활성화
	if staker.Amount == 0 {
		staker.IsActive = false
	}

	return nil
}

// GetStaker 스테이커 조회
func (s *StakerSet) GetStaker(address prt.Address) *Staker {
	addrStr := utils.AddressToString(address)
	return s.Stakers[addrStr]
}

// GetActiveStakers 활성 스테이커 목록
func (s *StakerSet) GetActiveStakers() []*Staker {
	var active []*Staker
	for _, staker := range s.Stakers {
		if staker.IsActive && staker.Amount > 0 {
			active = append(active, staker)
		}
	}
	return active
}

// SaveStakerSet DB에 스테이커셋 저장
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

// LoadStakerSet DB에서 스테이커셋 로드
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

// GetStakers 스테이커 주소 목록 (하위 호환)
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
