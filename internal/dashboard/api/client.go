package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client는 노드 API 클라이언트
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient는 새 API 클라이언트 생성
func NewClient(host string, port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// RestResp는 API 응답 래퍼
type RestResp struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// NodeStatus는 /api/v1/status 응답
type NodeStatus struct {
	CurrentHeight    uint64 `json:"currentHeight"`
	CurrentBlockHash string `json:"currentBlockHash"`
	GenesisHash      string `json:"genesisHash"`
	NetworkID        string `json:"networkId"`
	MempoolSize      int    `json:"mempoolSize"`
}

// ConsensusStatus는 /api/v1/consensus/status 응답
type ConsensusStatus struct {
	State         string            `json:"state"`
	CurrentHeight uint64            `json:"currentHeight"`
	CurrentRound  uint32            `json:"currentRound"`
	Proposer      string            `json:"proposer"`
	Validators    []ValidatorInfo   `json:"validators"`
	VotingPower   map[string]uint64 `json:"votingPower"`
}

// ValidatorInfo는 검증자 정보
type ValidatorInfo struct {
	Address       string `json:"address"`
	StakingAmount uint64 `json:"stakingAmount"`
	IsActive      bool   `json:"isActive"`
}

// P2PStatus는 /api/v1/p2p/status 응답
type P2PStatus struct {
	NodeID    string `json:"nodeId"`
	Running   bool   `json:"running"`
	PeerCount int    `json:"peerCount"`
}

// PeerInfo는 피어 정보
type PeerInfo struct {
	ID         string `json:"id"`
	Address    string `json:"address"`
	State      string `json:"state"`
	Version    string `json:"version"`
	BestHeight uint64 `json:"bestHeight"`
	LastSeen   int64  `json:"lastSeen"`
	Inbound    bool   `json:"inbound"`
}

// PeersResponse는 /api/v1/p2p/peers 응답
type PeersResponse struct {
	Count int        `json:"count"`
	Peers []PeerInfo `json:"peers"`
}

// GetStatus는 노드 상태 조회
func (c *Client) GetStatus() (*NodeStatus, error) {
	resp, err := c.get("/api/v1/status")
	if err != nil {
		return nil, err
	}

	var status NodeStatus
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	return &status, nil
}

// GetConsensusStatus는 컨센서스 상태 조회
func (c *Client) GetConsensusStatus() (*ConsensusStatus, error) {
	resp, err := c.get("/api/v1/consensus/status")
	if err != nil {
		return nil, err
	}

	var status ConsensusStatus
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return nil, fmt.Errorf("parse consensus: %w", err)
	}
	return &status, nil
}

// GetP2PStatus는 P2P 상태 조회
func (c *Client) GetP2PStatus() (*P2PStatus, error) {
	resp, err := c.get("/api/v1/p2p/status")
	if err != nil {
		return nil, err
	}

	var status P2PStatus
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return nil, fmt.Errorf("parse p2p: %w", err)
	}
	return &status, nil
}

// GetPeers는 피어 목록 조회
func (c *Client) GetPeers() (*PeersResponse, error) {
	resp, err := c.get("/api/v1/p2p/peers")
	if err != nil {
		return nil, err
	}

	var peers PeersResponse
	if err := json.Unmarshal(resp.Data, &peers); err != nil {
		return nil, fmt.Errorf("parse peers: %w", err)
	}
	return &peers, nil
}

// IsAlive는 노드 생존 여부 확인
func (c *Client) IsAlive() bool {
	_, err := c.GetStatus()
	return err == nil
}

func (c *Client) get(path string) (*RestResp, error) {
	resp, err := c.httpClient.Get(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result RestResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("api error: %s", result.Error)
	}

	return &result, nil
}
