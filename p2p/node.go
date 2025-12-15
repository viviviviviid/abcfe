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

// PeerState connection state
type PeerState uint8

const (
	PeerStateDisconnected PeerState = iota
	PeerStateConnecting
	PeerStateConnected
	PeerStateHandshaking
	PeerStateActive
)

// Peer information
type Peer struct {
	ID         string
	Address    string
	Port       int        // Remote Listen Port (received from handshake)
	Conn       net.Conn
	State      PeerState
	Version    string
	BestHeight uint64
	LastSeen   time.Time
	Inbound    bool // true: remote connected, false: I connected
}

// Node P2P node
type Node struct {
	mu sync.RWMutex

	ID         string
	Address    string
	Port       int
	NetworkID  string
	Version    string
	Blockchain *core.BlockChain // Blockchain reference (needed height info during handshake)

	// Peer management
	Peers       map[string]*Peer // key: peer ID
	MaxPeers    int
	BootNodes   []string // Bootstrap node addresses

	// Message handler
	MessageHandler func(*Message, *Peer)

	// Peer connected callback (for peer discovery)
	OnPeerConnected func(*Peer)

	// Periodic peer exchange callback
	OnPeerExchange func([]*Peer)

	// Listener
	listener net.Listener
	running  bool
	stopCh   chan struct{}
}

// NewNode creates new P2P node
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

// generateNodeID generates node ID
func generateNodeID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Start starts the node
func (n *Node) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.running {
		return fmt.Errorf("node already running")
	}

	// Start TCP listener
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", n.Address, n.Port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	n.listener = listener
	n.running = true

	// Connection accept goroutine
	go n.acceptLoop()

	// Connect to bootnodes
	go n.connectToBootNodes()

	// Peer maintenance
	go n.maintainPeers()

	return nil
}

// Stop stops the node
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

	// Close all peer connections
	for _, peer := range n.Peers {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
	}

	return nil
}

// acceptLoop connection accept loop
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

// handleInboundConnection handles inbound connection
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

	// Create temporary peer
	peer := &Peer{
		Conn:     conn,
		Address:  conn.RemoteAddr().String(),
		State:    PeerStateHandshaking,
		LastSeen: time.Now(),
		Inbound:  true,
	}

	// Wait for handshake
	go n.handlePeer(peer)
}

// Connect to peer
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

	// Send handshake
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

// handlePeer peer handling loop
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
			// Set read timeout
			peer.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			// 1. First read 4-byte length header
			header := make([]byte, 4)
			if _, err := io.ReadFull(peer.Conn, header); err != nil {
				logger.Debug("[P2P] Peer ", peer.Address, " read header error: ", err)
				return
			}

			// 2. Parse message length
			msgLen := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
			
			// Validate message size (max 10MB)
			if msgLen > 10*1024*1024 {
				logger.Error("[P2P] Message too large from ", peer.Address, ": ", msgLen, " bytes")
				return
			}

			// 3. Read message with exact length
			buffer := make([]byte, msgLen)
			peer.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			if _, err := io.ReadFull(peer.Conn, buffer); err != nil {
				logger.Debug("[P2P] Peer ", peer.Address, " read body error: ", err)
				return
			}

			logger.Debug("[P2P] Received complete message (", msgLen, " bytes) from ", peer.Address)

			// 4. Deserialize message
			msg, err := DeserializeMessage(buffer)
			if err != nil {
				logger.Error("[P2P] Failed to deserialize message from ", peer.Address, ": ", err)
				continue
			}

			peer.LastSeen = time.Now()

			// Handle handshake
			if peer.State == PeerStateHandshaking {
				n.handleHandshake(msg, peer)
				continue
			}

			// Call message handler
			if n.MessageHandler != nil {
				n.MessageHandler(msg, peer)
			}
		}
	}
}

// sendHandshake sends handshake
func (n *Node) sendHandshake(peer *Peer) error {
	// Get current block height
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

// handleHandshake handles handshake
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

	// Check Network ID
	if payload.NetworkID != n.NetworkID {
		logger.Warn("[P2P] Network ID mismatch. Expected: ", n.NetworkID, " Got: ", payload.NetworkID)
		peer.Conn.Close()
		return
	}

	peer.ID = payload.NodeID
	peer.Version = payload.Version
	peer.BestHeight = payload.BestHeight
	peer.Port = payload.ListenPort // Save remote Listen Port
	peer.State = PeerStateActive

	logger.Info("[P2P] Peer info updated: ID=", peer.ID, " BestHeight=", peer.BestHeight, " ListenPort=", peer.Port)

	// ACK response (if inbound)
	if peer.Inbound && msg.Type == MsgTypeHandshake {
		logger.Debug("[P2P] Sending handshake ACK to: ", peer.Address)
		n.sendHandshakeAck(peer)
	}

	// Register peer (check duplicate connection)
	if !n.addPeer(peer) {
		// Duplicate connection rejected - close connection
		logger.Info("[P2P] Duplicate peer rejected, closing connection: ", peer.ID[:16])
		peer.Conn.Close()
		return
	}
	logger.Info("[P2P] Peer activated: ", peer.ID, " (", peer.Address, ")")

	// Call peer connected callback (trigger peer discovery)
	if n.OnPeerConnected != nil {
		go n.OnPeerConnected(peer)
	}
}

// sendHandshakeAck sends handshake ACK
func (n *Node) sendHandshakeAck(peer *Peer) error {
	// Get current block height
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

// sendMessage sends message
func (n *Node) sendMessage(peer *Peer, msg *Message) error {
	msg.Timestamp = time.Now().Unix()

	data, err := msg.Serialize()
	if err != nil {
		return err
	}

	// Combine header + message into one buffer and send atomically
	// (Separate Write calls can mix messages during concurrent transmission)
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

// Broadcast message to all peers
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

// addPeer adds peer (prevent duplicate connection)
// Returns: true=peer added, false=already exists, not added
func (n *Node) addPeer(peer *Peer) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Check if peer with same ID already exists
	if existingPeer, exists := n.Peers[peer.ID]; exists {
		// If exists, reject new connection (keep existing)
		// Decide which connection to keep: Compare IDs for consistency (lower ID acts as outbound)
		if n.ID < peer.ID {
			// If my ID is lower, I should try to connect (reject existing inbound)
			if peer.Inbound {
				logger.Debug("[P2P] Rejecting duplicate inbound connection from ", peer.ID[:16], " (I should connect)")
				return false
			}
		} else {
			// If my ID is higher, peer should try to connect (reject existing outbound)
			if !peer.Inbound {
				logger.Debug("[P2P] Rejecting duplicate outbound connection to ", peer.ID[:16], " (peer should connect)")
				return false
			}
		}
		// Replace existing connection with new one
		if existingPeer.Conn != nil {
			existingPeer.Conn.Close()
		}
		logger.Debug("[P2P] Replacing existing connection to ", peer.ID[:16])
	}

	n.Peers[peer.ID] = peer
	return true
}

// removePeer removes peer
// Important: If replaced with new connection, when old handlePeer exits
// Compare with currently registered connection to avoid deleting new one
func (n *Node) removePeer(peer *Peer) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Check if registered peer matches the one being removed
	if existingPeer, exists := n.Peers[peer.ID]; exists {
		// If different connection (replaced), do not delete
		if existingPeer.Conn != peer.Conn {
			logger.Debug("[P2P] Skipping removal of replaced connection for peer ", peer.ID[:16])
			return
		}
	}

	delete(n.Peers, peer.ID)
}

// connectToBootNodes connects to bootnodes
func (n *Node) connectToBootNodes() {
	for _, addr := range n.BootNodes {
		go n.Connect(addr)
	}
}

// maintainPeers maintains peers
func (n *Node) maintainPeers() {
	ticker := time.NewTicker(10 * time.Second)
	peerExchangeTicker := time.NewTicker(30 * time.Second) // Exchange peers every 30 seconds
	defer ticker.Stop()
	defer peerExchangeTicker.Stop()

	for {
		select {
		case <-n.stopCh:
			return
		case <-ticker.C:
			n.pingPeers()
			n.removeInactivePeers()
			// Retry bootnode connection if no peers
			n.reconnectIfNeeded()
		case <-peerExchangeTicker.C:
			// Periodic peer exchange
			n.exchangePeers()
		}
	}
}

// exchangePeers exchanges peer list periodically
func (n *Node) exchangePeers() {
	peers := n.GetPeers()
	if len(peers) == 0 {
		return
	}

	// Call callback if set
	if n.OnPeerExchange != nil {
		n.OnPeerExchange(peers)
	}
}

// reconnectIfNeeded reconnects to bootnodes if no peers
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

// pingPeers sends Ping to all peers
func (n *Node) pingPeers() {
	msg := NewMessage(MsgTypePing, nil, n.ID)
	n.Broadcast(msg)
}

// removeInactivePeers removes inactive peers
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

// GetPeerCount returns peer count
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

// GetPeers returns peer list
func (n *Node) GetPeers() []*Peer {
	n.mu.RLock()
	defer n.mu.RUnlock()

	peers := make([]*Peer, 0, len(n.Peers))
	for _, peer := range n.Peers {
		peers = append(peers, peer)
	}
	return peers
}
