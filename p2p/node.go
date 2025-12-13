package p2p

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/core"
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

	ID         string
	Address    string
	Port       int
	NetworkID  string
	Version    string
	Blockchain *core.BlockChain // 블록체인 참조 (핸드셰이크 시 높이 정보 필요)

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
	logger.Debug("[P2P] Inbound connection from: ", conn.RemoteAddr().String())

	n.mu.Lock()
	if len(n.Peers) >= n.MaxPeers {
		n.mu.Unlock()
		conn.Close()
		logger.Warn("[P2P] Max peers reached, rejecting connection")
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
	logger.Debug("[P2P] Connecting to peer: ", address)

	n.mu.RLock()
	if len(n.Peers) >= n.MaxPeers {
		n.mu.RUnlock()
		return fmt.Errorf("max peers reached")
	}
	n.mu.RUnlock()

	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		logger.Debug("[P2P] Failed to connect to ", address, ": ", err)
		return fmt.Errorf("failed to connect: %w", err)
	}

	logger.Debug("[P2P] TCP connection established to: ", address)

	peer := &Peer{
		Address:  address,
		Conn:     conn,
		State:    PeerStateConnecting,
		LastSeen: time.Now(),
		Inbound:  false,
	}

	// 핸드셰이크 전송
	if err := n.sendHandshake(peer); err != nil {
		logger.Error("[P2P] Failed to send handshake to ", address, ": ", err)
		conn.Close()
		return err
	}

	logger.Debug("[P2P] Handshake sent to: ", address)
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

	for {
		select {
		case <-n.stopCh:
			return
		default:
			// 읽기 타임아웃 설정
			peer.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			// 1. 먼저 4바이트 길이 헤더 읽기
			header := make([]byte, 4)
			if _, err := io.ReadFull(peer.Conn, header); err != nil {
				logger.Debug("[P2P] Peer ", peer.Address, " read header error: ", err)
				return
			}

			// 2. 메시지 길이 파싱
			msgLen := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
			
			// 메시지 크기 검증 (최대 10MB)
			if msgLen > 10*1024*1024 {
				logger.Error("[P2P] Message too large from ", peer.Address, ": ", msgLen, " bytes")
				return
			}

			// 3. 정확한 길이만큼 메시지 읽기
			buffer := make([]byte, msgLen)
			peer.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			if _, err := io.ReadFull(peer.Conn, buffer); err != nil {
				logger.Debug("[P2P] Peer ", peer.Address, " read body error: ", err)
				return
			}

			logger.Debug("[P2P] Received complete message (", msgLen, " bytes) from ", peer.Address)

			// 4. 메시지 역직렬화
			msg, err := DeserializeMessage(buffer)
			if err != nil {
				logger.Error("[P2P] Failed to deserialize message from ", peer.Address, ": ", err)
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
	// 현재 블록 높이 가져오기
	bestHeight := uint64(0)
	bestHash := ""
	if n.Blockchain != nil {
		if height, err := n.Blockchain.GetLatestHeight(); err == nil {
			bestHeight = height
		}
		if hash, err := n.Blockchain.GetLatestBlockHash(); err == nil {
			bestHash = hash
		}
	}
	
	payload := HandshakePayload{
		Version:    n.Version,
		NodeID:     n.ID,
		NetworkID:  n.NetworkID,
		ListenPort: n.Port,
		BestHeight: bestHeight,
		BestHash:   bestHash,
	}

	hashPrefix := "empty"
	if len(bestHash) >= 16 {
		hashPrefix = bestHash[:16]
	} else if len(bestHash) > 0 {
		hashPrefix = bestHash
	}
	logger.Debug("[P2P] Sending handshake with height=", bestHeight, " hash=", hashPrefix, "...")

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
		logger.Warn("[P2P] Unexpected message type during handshake: ", msg.Type)
		return
	}

	var payload HandshakePayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal handshake payload: ", err)
		return
	}

	logger.Info("[P2P] Received handshake from node: ", payload.NodeID, " network: ", payload.NetworkID, " height: ", payload.BestHeight)

	// 네트워크 ID 확인
	if payload.NetworkID != n.NetworkID {
		logger.Warn("[P2P] Network ID mismatch. Expected: ", n.NetworkID, " Got: ", payload.NetworkID)
		peer.Conn.Close()
		return
	}

	peer.ID = payload.NodeID
	peer.Version = payload.Version
	peer.BestHeight = payload.BestHeight
	peer.State = PeerStateActive
	
	logger.Info("[P2P] Peer info updated: ID=", peer.ID, " BestHeight=", peer.BestHeight)

	// ACK 응답 (인바운드인 경우)
	if peer.Inbound && msg.Type == MsgTypeHandshake {
		logger.Debug("[P2P] Sending handshake ACK to: ", peer.Address)
		n.sendHandshakeAck(peer)
	}

	// 피어 등록
	n.addPeer(peer)
	logger.Info("[P2P] Peer activated: ", peer.ID, " (", peer.Address, ")")
}

// sendHandshakeAck 핸드셰이크 ACK 전송
func (n *Node) sendHandshakeAck(peer *Peer) error {
	// 현재 블록 높이 가져오기
	bestHeight := uint64(0)
	bestHash := ""
	if n.Blockchain != nil {
		if height, err := n.Blockchain.GetLatestHeight(); err == nil {
			bestHeight = height
		}
		if hash, err := n.Blockchain.GetLatestBlockHash(); err == nil {
			bestHash = hash
		}
	}
	
	payload := HandshakePayload{
		Version:    n.Version,
		NodeID:     n.ID,
		NetworkID:  n.NetworkID,
		ListenPort: n.Port,
		BestHeight: bestHeight,
		BestHash:   bestHash,
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

	// 헤더 + 메시지를 하나의 버퍼로 합쳐서 원자적으로 전송
	// (별도 Write 호출 시 동시 전송에서 메시지가 뒤섞일 수 있음)
	msgLen := uint32(len(data))
	packet := make([]byte, 4+len(data))
	packet[0] = byte(msgLen >> 24)
	packet[1] = byte(msgLen >> 16)
	packet[2] = byte(msgLen >> 8)
	packet[3] = byte(msgLen)
	copy(packet[4:], data)

	peer.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = peer.Conn.Write(packet)
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

	logger.Debug("[P2P] Broadcasting message type ", msg.Type, " to ", len(peers), " peers")

	for _, peer := range peers {
		go func(p *Peer) {
			if err := n.sendMessage(p, msg); err != nil {
				logger.Error("[P2P] Failed to send broadcast to ", p.Address, ": ", err)
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
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.stopCh:
			return
		case <-ticker.C:
			n.pingPeers()
			n.removeInactivePeers()
			// 피어가 없으면 부트노드에 재연결 시도
			n.reconnectIfNeeded()
		}
	}
}

// reconnectIfNeeded 피어가 없으면 부트노드에 재연결
func (n *Node) reconnectIfNeeded() {
	activeCount := n.GetPeerCount()
	if activeCount > 0 {
		return
	}

	logger.Warn("[P2P] No active peers, attempting to reconnect to boot nodes...")

	for _, addr := range n.BootNodes {
		go func(address string) {
			if err := n.Connect(address); err != nil {
				logger.Debug("[P2P] Failed to reconnect to boot node ", address, ": ", err)
			} else {
				logger.Info("[P2P] Reconnected to boot node: ", address)
			}
		}(addr)
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
