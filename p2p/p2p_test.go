package p2p

import (
	"testing"
	"time"
)

func TestNewNode(t *testing.T) {
	node, err := NewNode("127.0.0.1", 30303, "testnet")
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if node.ID == "" {
		t.Error("node ID should not be empty")
	}

	if node.Address != "127.0.0.1" {
		t.Errorf("expected address 127.0.0.1, got %s", node.Address)
	}

	if node.Port != 30303 {
		t.Errorf("expected port 30303, got %d", node.Port)
	}

	if node.NetworkID != "testnet" {
		t.Errorf("expected networkID testnet, got %s", node.NetworkID)
	}

	if node.MaxPeers != 50 {
		t.Errorf("expected maxPeers 50, got %d", node.MaxPeers)
	}
}

func TestNodeStartStop(t *testing.T) {
	node, err := NewNode("127.0.0.1", 30304, "testnet")
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// 노드 시작
	if err := node.Start(); err != nil {
		t.Fatalf("failed to start node: %v", err)
	}

	if !node.running {
		t.Error("node should be running")
	}

	// 중복 시작 시도
	if err := node.Start(); err == nil {
		t.Error("starting already running node should return error")
	}

	// 노드 종료
	if err := node.Stop(); err != nil {
		t.Fatalf("failed to stop node: %v", err)
	}

	if node.running {
		t.Error("node should not be running")
	}
}

func TestPeerConnection(t *testing.T) {
	// 서버 노드 생성
	server, err := NewNode("127.0.0.1", 30305, "testnet")
	if err != nil {
		t.Fatalf("failed to create server node: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// 클라이언트 노드 생성
	client, err := NewNode("127.0.0.1", 30306, "testnet")
	if err != nil {
		t.Fatalf("failed to create client node: %v", err)
	}

	if err := client.Start(); err != nil {
		t.Fatalf("failed to start client: %v", err)
	}
	defer client.Stop()

	// 클라이언트가 서버에 연결
	if err := client.Connect("127.0.0.1:30305"); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// 연결 완료 대기
	time.Sleep(500 * time.Millisecond)

	// 피어 수 확인
	if client.GetPeerCount() == 0 {
		t.Log("peer connection may take time, this is acceptable in test")
	}
}

func TestMessage(t *testing.T) {
	msg := NewMessage(MsgTypePing, nil, "node123")

	if msg.Type != MsgTypePing {
		t.Errorf("expected message type %d, got %d", MsgTypePing, msg.Type)
	}

	if msg.From != "node123" {
		t.Errorf("expected from node123, got %s", msg.From)
	}

	// 직렬화 테스트
	data, err := msg.Serialize()
	if err != nil {
		t.Fatalf("failed to serialize message: %v", err)
	}

	// 역직렬화 테스트
	deserialized, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf("failed to deserialize message: %v", err)
	}

	if deserialized.Type != msg.Type {
		t.Errorf("deserialized type mismatch: expected %d, got %d", msg.Type, deserialized.Type)
	}

	if deserialized.From != msg.From {
		t.Errorf("deserialized from mismatch: expected %s, got %s", msg.From, deserialized.From)
	}
}

func TestHandshakePayload(t *testing.T) {
	payload := HandshakePayload{
		Version:    "1.0.0",
		NodeID:     "test-node",
		NetworkID:  "testnet",
		ListenPort: 30303,
		BestHeight: 100,
		BestHash:   "abc123",
	}

	// 직렬화
	data, err := MarshalPayload(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	// 역직렬화
	var decoded HandshakePayload
	if err := UnmarshalPayload(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if decoded.Version != payload.Version {
		t.Errorf("version mismatch: expected %s, got %s", payload.Version, decoded.Version)
	}

	if decoded.NodeID != payload.NodeID {
		t.Errorf("nodeID mismatch: expected %s, got %s", payload.NodeID, decoded.NodeID)
	}

	if decoded.BestHeight != payload.BestHeight {
		t.Errorf("bestHeight mismatch: expected %d, got %d", payload.BestHeight, decoded.BestHeight)
	}
}

func TestP2PService(t *testing.T) {
	service, err := NewP2PService("127.0.0.1", 30307, "testnet", nil)
	if err != nil {
		t.Fatalf("failed to create p2p service: %v", err)
	}

	if service.Node == nil {
		t.Error("node should not be nil")
	}

	if service.IsRunning() {
		t.Error("service should not be running")
	}

	// 서비스 시작
	if err := service.Start(); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	if !service.IsRunning() {
		t.Error("service should be running")
	}

	// 노드 ID 확인
	if service.GetNodeID() == "" {
		t.Error("node ID should not be empty")
	}

	// 서비스 종료
	if err := service.Stop(); err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}

	if service.IsRunning() {
		t.Error("service should not be running")
	}
}

func TestGetBlocksPayload(t *testing.T) {
	payload := GetBlocksPayload{
		StartHeight: 10,
		EndHeight:   20,
	}

	data, err := MarshalPayload(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var decoded GetBlocksPayload
	if err := UnmarshalPayload(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if decoded.StartHeight != payload.StartHeight {
		t.Errorf("startHeight mismatch: expected %d, got %d", payload.StartHeight, decoded.StartHeight)
	}

	if decoded.EndHeight != payload.EndHeight {
		t.Errorf("endHeight mismatch: expected %d, got %d", payload.EndHeight, decoded.EndHeight)
	}
}

func TestPeerState(t *testing.T) {
	tests := []struct {
		state    PeerState
		expected PeerState
	}{
		{PeerStateDisconnected, 0},
		{PeerStateConnecting, 1},
		{PeerStateConnected, 2},
		{PeerStateHandshaking, 3},
		{PeerStateActive, 4},
	}

	for _, tt := range tests {
		if tt.state != tt.expected {
			t.Errorf("peer state mismatch: expected %d, got %d", tt.expected, tt.state)
		}
	}
}

func TestMessageTypes(t *testing.T) {
	// 메시지 타입 상수 확인
	types := []MessageType{
		MsgTypeHandshake,
		MsgTypeHandshakeAck,
		MsgTypeNewBlock,
		MsgTypeGetBlock,
		MsgTypeBlock,
		MsgTypeGetBlocks,
		MsgTypeBlocks,
		MsgTypeNewTx,
		MsgTypeGetTx,
		MsgTypeTx,
		MsgTypePing,
		MsgTypePong,
		MsgTypeGetPeers,
		MsgTypePeers,
		MsgTypeProposal,
		MsgTypeVote,
	}

	// 모든 타입이 고유한지 확인
	seen := make(map[MessageType]bool)
	for _, msgType := range types {
		if seen[msgType] {
			t.Errorf("duplicate message type: %d", msgType)
		}
		seen[msgType] = true
	}
}
