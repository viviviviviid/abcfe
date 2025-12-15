package p2p

import (
	"encoding/json"
	"fmt"

	prt "github.com/abcfe/abcfe-node/protocol"
)

// MessageType message type
type MessageType uint8

const (
	// Handshake
	MsgTypeHandshake MessageType = iota
	MsgTypeHandshakeAck

	// Block related
	MsgTypeNewBlock
	MsgTypeGetBlock
	MsgTypeBlock
	MsgTypeGetBlocks // Block range request
	MsgTypeBlocks    // Multi-block response

	// Transaction related
	MsgTypeNewTx
	MsgTypeGetTx
	MsgTypeTx

	// Peer management
	MsgTypePing
	MsgTypePong
	MsgTypeGetPeers
	MsgTypePeers

	// Consensus
	MsgTypeProposal
	MsgTypeVote
)

// Message P2P message
type Message struct {
	Type      MessageType `json:"type"`
	Payload   []byte      `json:"payload"`
	From      string      `json:"from"`      // Sender Peer ID
	Timestamp int64       `json:"timestamp"`
}

// NewMessage creates new message
func NewMessage(msgType MessageType, payload []byte, from string) *Message {
	return &Message{
		Type:      msgType,
		Payload:   payload,
		From:      from,
		Timestamp: 0, // Set on send
	}
}

// Serialize message serialization
func (m *Message) Serialize() ([]byte, error) {
	return json.Marshal(m)
}

// DeserializeMessage message deserialization
func DeserializeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to deserialize message: %w", err)
	}
	return &msg, nil
}

// ===== Payload Type Definitions =====

// HandshakePayload handshake payload
type HandshakePayload struct {
	Version    string `json:"version"`
	NodeID     string `json:"nodeId"`
	NetworkID  string `json:"networkId"`
	ListenPort int    `json:"listenPort"`
	BestHeight uint64 `json:"bestHeight"`
	BestHash   string `json:"bestHash"`
}

// NewBlockPayload new block notification payload
type NewBlockPayload struct {
	Height    uint64   `json:"height"`
	Hash      prt.Hash `json:"hash"`
	BlockData []byte   `json:"blockData"` // Serialized block
}

// GetBlockPayload block request payload
type GetBlockPayload struct {
	Height uint64   `json:"height,omitempty"`
	Hash   prt.Hash `json:"hash,omitempty"`
}

// GetBlocksPayload block range request payload
type GetBlocksPayload struct {
	StartHeight uint64 `json:"startHeight"`
	EndHeight   uint64 `json:"endHeight"`
}

// NewTxPayload new transaction notification payload
type NewTxPayload struct {
	TxID   prt.Hash `json:"txId"`
	TxData []byte   `json:"txData"` // Serialized transaction
}

// GetTxPayload transaction request payload
type GetTxPayload struct {
	TxID prt.Hash `json:"txId"`
}

// PeersPayload peer list payload
type PeersPayload struct {
	Peers []PeerInfo `json:"peers"`
}

// PeerInfo peer info
type PeerInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// ProposalPayload block proposal payload
type ProposalPayload struct {
	Height     uint64        `json:"height"`
	Round      uint32        `json:"round"`
	BlockHash  prt.Hash      `json:"blockHash"`
	BlockData  []byte        `json:"blockData"`
	ProposerID string        `json:"proposerId"`
	Signature  prt.Signature `json:"signature"`
}

// VotePayload vote payload
type VotePayload struct {
	Height    uint64        `json:"height"`
	Round     uint32        `json:"round"`
	VoteType  uint8         `json:"voteType"` // 0: Prevote, 1: Precommit
	BlockHash prt.Hash      `json:"blockHash"`
	VoterID   string        `json:"voterId"`
	Signature prt.Signature `json:"signature"`
}

// ===== Payload Serialization Helpers =====

// MarshalPayload payload serialization
func MarshalPayload(payload interface{}) ([]byte, error) {
	return json.Marshal(payload)
}

// UnmarshalPayload payload deserialization
func UnmarshalPayload(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
