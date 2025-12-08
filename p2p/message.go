package p2p

import (
	"encoding/json"
	"fmt"

	prt "github.com/abcfe/abcfe-node/protocol"
)

// MessageType 메시지 유형
type MessageType uint8

const (
	// 핸드셰이크
	MsgTypeHandshake MessageType = iota
	MsgTypeHandshakeAck

	// 블록 관련
	MsgTypeNewBlock
	MsgTypeGetBlock
	MsgTypeBlock
	MsgTypeGetBlocks // 블록 범위 요청
	MsgTypeBlocks    // 다중 블록 응답

	// 트랜잭션 관련
	MsgTypeNewTx
	MsgTypeGetTx
	MsgTypeTx

	// 피어 관리
	MsgTypePing
	MsgTypePong
	MsgTypeGetPeers
	MsgTypePeers

	// 컨센서스
	MsgTypeProposal
	MsgTypeVote
)

// Message P2P 메시지
type Message struct {
	Type      MessageType `json:"type"`
	Payload   []byte      `json:"payload"`
	From      string      `json:"from"`      // 발신자 피어 ID
	Timestamp int64       `json:"timestamp"`
}

// NewMessage 새 메시지 생성
func NewMessage(msgType MessageType, payload []byte, from string) *Message {
	return &Message{
		Type:      msgType,
		Payload:   payload,
		From:      from,
		Timestamp: 0, // 전송 시 설정
	}
}

// Serialize 메시지 직렬화
func (m *Message) Serialize() ([]byte, error) {
	return json.Marshal(m)
}

// DeserializeMessage 메시지 역직렬화
func DeserializeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to deserialize message: %w", err)
	}
	return &msg, nil
}

// ===== 페이로드 타입 정의 =====

// HandshakePayload 핸드셰이크 페이로드
type HandshakePayload struct {
	Version    string `json:"version"`
	NodeID     string `json:"nodeId"`
	NetworkID  string `json:"networkId"`
	ListenPort int    `json:"listenPort"`
	BestHeight uint64 `json:"bestHeight"`
	BestHash   string `json:"bestHash"`
}

// NewBlockPayload 새 블록 알림 페이로드
type NewBlockPayload struct {
	Height    uint64   `json:"height"`
	Hash      prt.Hash `json:"hash"`
	BlockData []byte   `json:"blockData"` // 직렬화된 블록
}

// GetBlockPayload 블록 요청 페이로드
type GetBlockPayload struct {
	Height uint64   `json:"height,omitempty"`
	Hash   prt.Hash `json:"hash,omitempty"`
}

// GetBlocksPayload 블록 범위 요청 페이로드
type GetBlocksPayload struct {
	StartHeight uint64 `json:"startHeight"`
	EndHeight   uint64 `json:"endHeight"`
}

// NewTxPayload 새 트랜잭션 알림 페이로드
type NewTxPayload struct {
	TxID   prt.Hash `json:"txId"`
	TxData []byte   `json:"txData"` // 직렬화된 트랜잭션
}

// GetTxPayload 트랜잭션 요청 페이로드
type GetTxPayload struct {
	TxID prt.Hash `json:"txId"`
}

// PeersPayload 피어 목록 페이로드
type PeersPayload struct {
	Peers []PeerInfo `json:"peers"`
}

// PeerInfo 피어 정보
type PeerInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// ProposalPayload 블록 제안 페이로드
type ProposalPayload struct {
	Height     uint64        `json:"height"`
	Round      uint32        `json:"round"`
	BlockHash  prt.Hash      `json:"blockHash"`
	BlockData  []byte        `json:"blockData"`
	ProposerID string        `json:"proposerId"`
	Signature  prt.Signature `json:"signature"`
}

// VotePayload 투표 페이로드
type VotePayload struct {
	Height    uint64        `json:"height"`
	Round     uint32        `json:"round"`
	VoteType  uint8         `json:"voteType"` // 0: Prevote, 1: Precommit
	BlockHash prt.Hash      `json:"blockHash"`
	VoterID   string        `json:"voterId"`
	Signature prt.Signature `json:"signature"`
}

// ===== 페이로드 직렬화 헬퍼 =====

// MarshalPayload 페이로드 직렬화
func MarshalPayload(payload interface{}) ([]byte, error) {
	return json.Marshal(payload)
}

// UnmarshalPayload 페이로드 역직렬화
func UnmarshalPayload(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
