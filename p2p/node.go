package p2p

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// PeerState 피어 연결 상태
type PeerState uint8

const (
	PeerStateDisconnected PeerState = iota
	PeerStateConnecting
	PeerStateConnected
	PeerStateHandshaking
	PeerStateActive
)

// Peer 피어 정보
type Peer struct {
	ID         string
	Address    string
	Port       int
	Conn       net.Conn
	State      PeerState
	Version    string
	BestHeight uint64
	LastSeen   time.Time
	Inbound    bool // true: 상대가 연결함, false: 내가 연결함
}

// Node P2P 노드
type Node struct {
	mu sync.RWMutex

	ID        string
	Address   string
	Port      int
	NetworkID string
	Version   string

	// 피어 관리
	Peers       map[string]*Peer // key: peer ID
	MaxPeers    int
	BootNodes   []string // 부트스트랩 노드 주소

	// 메시지 핸들러
	MessageHandler func(*Message, *Peer)

	// 리스너
	listener net.Listener
	running  bool
	stopCh   chan struct{}
}

// NewNode 새 P2P 노드 생성
func NewNode(address string, port int, networkID string) (*Node, error) {
	nodeID, err := generateNodeID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate node ID: %w", err)
	}

	return &Node{
		ID:        nodeID,
		Address:   address,
		Port:      port,
		NetworkID: networkID,
		Version:   "1.0.0",
		Peers:     make(map[string]*Peer),
		MaxPeers:  50,
		BootNodes: []string{},
		stopCh:    make(chan struct{}),
	}, nil
}

// generateNodeID 노드 ID 생성
func generateNodeID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Start 노드 시작
func (n *Node) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.running {
		return fmt.Errorf("node already running")
	}

	// TCP 리스너 시작
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", n.Address, n.Port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	n.listener = listener
	n.running = true

	// 연결 수락 고루틴
	go n.acceptLoop()

	// 부트노드 연결
	go n.connectToBootNodes()

	// 피어 유지보수
	go n.maintainPeers()

	return nil
}

// Stop 노드 종료
func (n *Node) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	n.running = false
	close(n.stopCh)

	if n.listener != nil {
		n.listener.Close()
	}

	// 모든 피어 연결 종료
	for _, peer := range n.Peers {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
	}

	return nil
}

// acceptLoop 연결 수락 루프
func (n *Node) acceptLoop() {
	for {
		select {
		case <-n.stopCh:
			return
		default:
			conn, err := n.listener.Accept()
			if err != nil {
				continue
			}

			go n.handleInboundConnection(conn)
		}
	}
}

// handleInboundConnection 인바운드 연결 처리
func (n *Node) handleInboundConnection(conn net.Conn) {
	log.Println("[P2P] Inbound connection from:", conn.RemoteAddr().String())

	n.mu.Lock()
	if len(n.Peers) >= n.MaxPeers {
		n.mu.Unlock()
		conn.Close()
		log.Println("[P2P] Max peers reached, rejecting connection")
		return
	}
	n.mu.Unlock()

	// 임시 피어 생성
	peer := &Peer{
		Conn:     conn,
		Address:  conn.RemoteAddr().String(),
		State:    PeerStateHandshaking,
		LastSeen: time.Now(),
		Inbound:  true,
	}

	// 핸드셰이크 대기
	go n.handlePeer(peer)
}

// Connect 피어에 연결
func (n *Node) Connect(address string) error {
	log.Println("[P2P] Connecting to peer:", address)

	n.mu.RLock()
	if len(n.Peers) >= n.MaxPeers {
		n.mu.RUnlock()
		return fmt.Errorf("max peers reached")
	}
	n.mu.RUnlock()

	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		log.Println("[P2P] Failed to connect to", address, ":", err)
		return fmt.Errorf("failed to connect: %w", err)
	}

	log.Println("[P2P] TCP connection established to:", address)

	peer := &Peer{
		Address:  address,
		Conn:     conn,
		State:    PeerStateConnecting,
		LastSeen: time.Now(),
		Inbound:  false,
	}

	// 핸드셰이크 전송
	if err := n.sendHandshake(peer); err != nil {
		log.Println("[P2P] Failed to send handshake to", address, ":", err)
		conn.Close()
		return err
	}

	log.Println("[P2P] Handshake sent to:", address)
	peer.State = PeerStateHandshaking
	go n.handlePeer(peer)

	return nil
}

// handlePeer 피어 처리 루프
func (n *Node) handlePeer(peer *Peer) {
	defer func() {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
		n.removePeer(peer)
	}()

	buffer := make([]byte, 65536) // 64KB 버퍼

	for {
		select {
		case <-n.stopCh:
			return
		default:
			// 읽기 타임아웃 설정
			peer.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			nBytes, err := peer.Conn.Read(buffer)
			if err != nil {
				return
			}

			msg, err := DeserializeMessage(buffer[:nBytes])
			if err != nil {
				continue
			}

			peer.LastSeen = time.Now()

			// 핸드셰이크 처리
			if peer.State == PeerStateHandshaking {
				n.handleHandshake(msg, peer)
				continue
			}

			// 메시지 핸들러 호출
			if n.MessageHandler != nil {
				n.MessageHandler(msg, peer)
			}
		}
	}
}

// sendHandshake 핸드셰이크 전송
func (n *Node) sendHandshake(peer *Peer) error {
	payload := HandshakePayload{
		Version:    n.Version,
		NodeID:     n.ID,
		NetworkID:  n.NetworkID,
		ListenPort: n.Port,
		BestHeight: 0, // TODO: 실제 높이
		BestHash:   "",
	}

	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		return err
	}

	msg := NewMessage(MsgTypeHandshake, payloadBytes, n.ID)
	return n.sendMessage(peer, msg)
}

// handleHandshake 핸드셰이크 처리
func (n *Node) handleHandshake(msg *Message, peer *Peer) {
	if msg.Type != MsgTypeHandshake && msg.Type != MsgTypeHandshakeAck {
		log.Println("[P2P] Unexpected message type during handshake:", msg.Type)
		return
	}

	var payload HandshakePayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		log.Println("[P2P] Failed to unmarshal handshake payload:", err)
		return
	}

	log.Println("[P2P] Received handshake from node:", payload.NodeID, "network:", payload.NetworkID)

	// 네트워크 ID 확인
	if payload.NetworkID != n.NetworkID {
		log.Println("[P2P] Network ID mismatch. Expected:", n.NetworkID, "Got:", payload.NetworkID)
		peer.Conn.Close()
		return
	}

	peer.ID = payload.NodeID
	peer.Version = payload.Version
	peer.BestHeight = payload.BestHeight
	peer.State = PeerStateActive

	// ACK 응답 (인바운드인 경우)
	if peer.Inbound && msg.Type == MsgTypeHandshake {
		log.Println("[P2P] Sending handshake ACK to:", peer.Address)
		n.sendHandshakeAck(peer)
	}

	// 피어 등록
	n.addPeer(peer)
	log.Println("[P2P] Peer activated:", peer.ID, "(", peer.Address, ")")
}

// sendHandshakeAck 핸드셰이크 ACK 전송
func (n *Node) sendHandshakeAck(peer *Peer) error {
	payload := HandshakePayload{
		Version:    n.Version,
		NodeID:     n.ID,
		NetworkID:  n.NetworkID,
		ListenPort: n.Port,
	}

	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		return err
	}

	msg := NewMessage(MsgTypeHandshakeAck, payloadBytes, n.ID)
	return n.sendMessage(peer, msg)
}

// sendMessage 메시지 전송
func (n *Node) sendMessage(peer *Peer, msg *Message) error {
	msg.Timestamp = time.Now().Unix()

	data, err := msg.Serialize()
	if err != nil {
		return err
	}

	peer.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = peer.Conn.Write(data)
	return err
}

// Broadcast 모든 피어에게 메시지 브로드캐스트
func (n *Node) Broadcast(msg *Message) {
	n.mu.RLock()
	peers := make([]*Peer, 0, len(n.Peers))
	for _, peer := range n.Peers {
		if peer.State == PeerStateActive {
			peers = append(peers, peer)
		}
	}
	n.mu.RUnlock()

	log.Println("[P2P] Broadcasting message type", msg.Type, "to", len(peers), "peers")

	for _, peer := range peers {
		go func(p *Peer) {
			if err := n.sendMessage(p, msg); err != nil {
				log.Println("[P2P] Failed to send broadcast to", p.Address, ":", err)
			}
		}(peer)
	}
}

// addPeer 피어 추가
func (n *Node) addPeer(peer *Peer) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.Peers[peer.ID] = peer
}

// removePeer 피어 제거
func (n *Node) removePeer(peer *Peer) {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.Peers, peer.ID)
}

// connectToBootNodes 부트노드에 연결
func (n *Node) connectToBootNodes() {
	for _, addr := range n.BootNodes {
		go n.Connect(addr)
	}
}

// maintainPeers 피어 유지보수
func (n *Node) maintainPeers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.stopCh:
			return
		case <-ticker.C:
			n.pingPeers()
			n.removeInactivePeers()
		}
	}
}

// pingPeers 모든 피어에게 Ping 전송
func (n *Node) pingPeers() {
	msg := NewMessage(MsgTypePing, nil, n.ID)
	n.Broadcast(msg)
}

// removeInactivePeers 비활성 피어 제거
func (n *Node) removeInactivePeers() {
	n.mu.Lock()
	defer n.mu.Unlock()

	threshold := time.Now().Add(-2 * time.Minute)
	for id, peer := range n.Peers {
		if peer.LastSeen.Before(threshold) {
			if peer.Conn != nil {
				peer.Conn.Close()
			}
			delete(n.Peers, id)
		}
	}
}

// GetPeerCount 피어 수 조회
func (n *Node) GetPeerCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	count := 0
	for _, peer := range n.Peers {
		if peer.State == PeerStateActive {
			count++
		}
	}
	return count
}

// GetPeers 피어 목록 조회
func (n *Node) GetPeers() []*Peer {
	n.mu.RLock()
	defer n.mu.RUnlock()

	peers := make([]*Peer, 0, len(n.Peers))
	for _, peer := range n.Peers {
		peers = append(peers, peer)
	}
	return peers
}
