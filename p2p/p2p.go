package p2p

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/common/utils"
	"github.com/abcfe/abcfe-node/core"
	prt "github.com/abcfe/abcfe-node/protocol"
)

// P2PService P2P networking service
type P2PService struct {
	mu sync.RWMutex

	Node       *Node
	Blockchain *core.BlockChain

	// Message handler
	blockHandler    func(*core.Block)
	txHandler       func(*core.Transaction)
	proposalHandler func(height uint64, round uint32, blockHash prt.Hash, block *core.Block)
	voteHandler     func(height uint64, round uint32, voteType uint8, blockHash prt.Hash, voterID prt.Address, signature prt.Signature)

	// Message deduplication cache (prevent infinite relay)
	seenMessages   map[string]time.Time
	seenMessagesMu sync.RWMutex

	// Proposal/Vote deduplication (based on height:round:proposer)
	seenProposals   map[string]time.Time // key: "height:round:proposer"
	seenProposalsMu sync.RWMutex
	seenVotes       map[string]time.Time // key: "height:round:voteType:voter"
	seenVotesMu     sync.RWMutex

	// Track connecting peer IDs (prevent duplicate connections)
	connectingPeers   map[string]time.Time
	connectingPeersMu sync.Mutex

	running bool
}

// NewP2PService creates a new P2P service
func NewP2PService(address string, port int, networkID string, blockchain *core.BlockChain) (*P2PService, error) {
	node, err := NewNode(address, port, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	service := &P2PService{
		Node:            node,
		Blockchain:      blockchain,
		seenMessages:    make(map[string]time.Time),
		seenProposals:   make(map[string]time.Time),
		seenVotes:       make(map[string]time.Time),
		connectingPeers: make(map[string]time.Time),
		running:         false,
	}

	// Set blockchain reference to node (for handshake)
	node.Blockchain = blockchain

	// Set message handler
	node.MessageHandler = service.handleMessage

	// Request peer list on peer connection (peer discovery)
	node.OnPeerConnected = func(peer *Peer) {
		// Wait a moment then request peer list (after handshake completes)
		go func() {
			// Wait 100ms
			select {
			case <-node.stopCh:
				return
			default:
			}
			if err := service.RequestPeers(peer); err != nil {
				logger.Debug("[P2P] Failed to request peers from ", peer.Address, ": ", err)
			} else {
				logger.Debug("[P2P] Requested peers from ", peer.Address)
			}
		}()
	}

	// Periodic peer exchange (every 30 seconds)
	node.OnPeerExchange = func(peers []*Peer) {
		logger.Debug("[P2P] Periodic peer exchange with ", len(peers), " peers")
		for _, peer := range peers {
			if peer.State == PeerStateActive {
				go func(p *Peer) {
					if err := service.RequestPeers(p); err != nil {
						logger.Debug("[P2P] Failed to request peers from ", p.Address, ": ", err)
					}
				}(peer)
			}
		}
	}

	return service, nil
}

// Start P2P service
func (s *P2PService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("p2p service already running")
	}

	if err := s.Node.Start(); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	s.running = true
	return nil
}

// Stop P2P service
func (s *P2PService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if err := s.Node.Stop(); err != nil {
		return fmt.Errorf("failed to stop node: %w", err)
	}

	s.running = false
	return nil
}

// SetBlockHandler sets block reception handler
func (s *P2PService) SetBlockHandler(handler func(*core.Block)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockHandler = handler
}

// SetTxHandler sets transaction reception handler
func (s *P2PService) SetTxHandler(handler func(*core.Transaction)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txHandler = handler
}

// SetProposalHandler sets block proposal reception handler
func (s *P2PService) SetProposalHandler(handler func(height uint64, round uint32, blockHash prt.Hash, block *core.Block)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proposalHandler = handler
}

// SetVoteHandler sets vote reception handler
func (s *P2PService) SetVoteHandler(handler func(height uint64, round uint32, voteType uint8, blockHash prt.Hash, voterID prt.Address, signature prt.Signature)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.voteHandler = handler
}

// handleMessage processes received message
func (s *P2PService) handleMessage(msg *Message, peer *Peer) {
	switch msg.Type {
	case MsgTypePing:
		s.handlePing(peer)
	case MsgTypePong:
		// Pong updates LastSeen only (handled in handlePeer)
	case MsgTypeNewBlock:
		s.handleNewBlock(msg, peer)
	case MsgTypeNewTx:
		s.handleNewTx(msg, peer)
	case MsgTypeGetBlocks:
		s.handleGetBlocks(msg, peer)
	case MsgTypeBlocks:
		s.handleBlocks(msg, peer)
	case MsgTypeProposal:
		s.handleProposal(msg, peer)
	case MsgTypeVote:
		s.handleVote(msg, peer)
	case MsgTypeGetPeers:
		s.handleGetPeers(msg, peer)
	case MsgTypePeers:
		s.handlePeers(msg, peer)
	}
}

// handlePing processes Ping
func (s *P2PService) handlePing(peer *Peer) {
	pong := NewMessage(MsgTypePong, nil, s.Node.ID)
	s.Node.sendMessage(peer, pong)
}

// handleNewBlock processes new block reception
func (s *P2PService) handleNewBlock(msg *Message, peer *Peer) {
	logger.Debug("[P2P] Received new block message from peer: ", peer.Address)

	var payload NewBlockPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal block payload: ", err)
		return
	}

	logger.Debug("[P2P] Block payload - height: ", payload.Height)

	// Deserialize block
	var block core.Block
	if err := utils.DeserializeData(payload.BlockData, &block, utils.SerializationFormatGob); err != nil {
		logger.Error("[P2P] Failed to deserialize block: ", err)
		return
	}

	logger.Debug("[P2P] Received block height: ", block.Header.Height, " hash: ", fmt.Sprintf("%x", block.Header.Hash[:8]))

	// Call block handler
	s.mu.RLock()
	handler := s.blockHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(&block)
	} else {
		logger.Warn("[P2P] Block handler not set!")
	}
}

// handleNewTx processes new transaction reception
func (s *P2PService) handleNewTx(msg *Message, peer *Peer) {
	var payload NewTxPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		return
	}

	// Deserialize transaction
	var tx core.Transaction
	if err := utils.DeserializeData(payload.TxData, &tx, utils.SerializationFormatGob); err != nil {
		return
	}

	// Call transaction handler
	s.mu.RLock()
	handler := s.txHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(&tx)
	}
}

// handleGetBlocks processes block request
func (s *P2PService) handleGetBlocks(msg *Message, peer *Peer) {
	var payload GetBlocksPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal GetBlocks payload: ", err)
		return
	}

	logger.Info("[P2P] Received GetBlocks request from ", peer.Address, " for blocks ", payload.StartHeight, " to ", payload.EndHeight)

	if s.Blockchain == nil {
		logger.Error("[P2P] Blockchain not set, cannot respond to GetBlocks")
		return
	}

	// Get requested blocks
	blocksData := make([][]byte, 0)
	for height := payload.StartHeight; height <= payload.EndHeight && len(blocksData) < 100; height++ {
		block, err := s.Blockchain.GetBlockByHeight(height)
		if err != nil {
			logger.Error("[P2P] Failed to get block at height ", height, ": ", err)
			break
		}
		blockData, err := utils.SerializeData(block, utils.SerializationFormatGob)
		if err != nil {
			logger.Error("[P2P] Failed to serialize block at height ", height, ": ", err)
			continue
		}
		blocksData = append(blocksData, blockData)
	}

	logger.Info("[P2P] Sending ", len(blocksData), " blocks to ", peer.Address)

	// Send block response
	responsePayload := BlocksPayload{BlocksData: blocksData}
	payloadBytes, err := MarshalPayload(responsePayload)
	if err != nil {
		logger.Error("[P2P] Failed to marshal Blocks payload: ", err)
		return
	}

	response := NewMessage(MsgTypeBlocks, payloadBytes, s.Node.ID)
	if err := s.Node.sendMessage(peer, response); err != nil {
		logger.Error("[P2P] Failed to send Blocks message: ", err)
	}
}

// handleBlocks processes block response
func (s *P2PService) handleBlocks(msg *Message, peer *Peer) {
	var payload BlocksPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal Blocks payload: ", err)
		return
	}

	logger.Info("[P2P] Received ", len(payload.BlocksData), " blocks from ", peer.Address)

	// Process received blocks
	s.mu.RLock()
	handler := s.blockHandler
	s.mu.RUnlock()

	if handler != nil {
		for i, blockData := range payload.BlocksData {
			var block core.Block
			if err := utils.DeserializeData(blockData, &block, utils.SerializationFormatGob); err != nil {
				logger.Error("[P2P] Failed to deserialize block ", i, ": ", err)
				continue
			}
			logger.Debug("[P2P] Processing received block height=", block.Header.Height)
			handler(&block)
		}
	} else {
		logger.Error("[P2P] Block handler not set!")
	}
}

// handleProposal processes block proposal reception
func (s *P2PService) handleProposal(msg *Message, peer *Peer) {
	logger.Debug("[P2P] Received proposal message from peer: ", peer.Address)

	var payload ProposalPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal proposal payload: ", err)
		return
	}

	// Check duplicate Proposal (process same height:round:proposer only once)
	proposalKey := fmt.Sprintf("%d:%d:%s", payload.Height, payload.Round, payload.ProposerID)
	if s.hasSeenProposal(proposalKey) {
		logger.Debug("[P2P] Duplicate proposal ignored: ", proposalKey)
		return
	}
	s.markProposalSeen(proposalKey)

	// Deserialize block
	var block core.Block
	if err := utils.DeserializeData(payload.BlockData, &block, utils.SerializationFormatGob); err != nil {
		logger.Error("[P2P] Failed to deserialize proposed block: ", err)
		return
	}

	logger.Info("[P2P] Received proposal - height: ", payload.Height, ", round: ", payload.Round, ", proposer: ", payload.ProposerID[:16])

	// Relay to other peers (exclude sender)
	s.relayMessage(msg, peer)

	// Call proposal handler
	s.mu.RLock()
	handler := s.proposalHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(payload.Height, payload.Round, payload.BlockHash, &block)
	} else {
		logger.Warn("[P2P] Proposal handler not set!")
	}
}

// handleVote processes vote reception
func (s *P2PService) handleVote(msg *Message, peer *Peer) {
	logger.Debug("[P2P] Received vote message from peer: ", peer.Address)

	var payload VotePayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal vote payload: ", err)
		return
	}

	// Check duplicate Vote (process same height:round:voteType:voter only once)
	voteKey := fmt.Sprintf("%d:%d:%d:%s", payload.Height, payload.Round, payload.VoteType, payload.VoterID)
	if s.hasSeenVote(voteKey) {
		logger.Debug("[P2P] Duplicate vote ignored: ", voteKey)
		return
	}
	s.markVoteSeen(voteKey)

	voteTypeStr := "Prevote"
	if payload.VoteType == 1 {
		voteTypeStr = "Precommit"
	}
	logger.Debug("[P2P] Received ", voteTypeStr, " - height: ", payload.Height, ", round: ", payload.Round, ", voter: ", payload.VoterID[:16])

	// Relay to other peers (exclude sender)
	s.relayMessage(msg, peer)

	// Convert VoterID to Address
	voterAddr, err := utils.StringToAddress(payload.VoterID)
	if err != nil {
		logger.Error("[P2P] Failed to parse voter address: ", err)
		return
	}

	// Call vote handler
	s.mu.RLock()
	handler := s.voteHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(payload.Height, payload.Round, payload.VoteType, payload.BlockHash, voterAddr, payload.Signature)
	} else {
		logger.Warn("[P2P] Vote handler not set!")
	}
}

// handleGetPeers processes peer list request
func (s *P2PService) handleGetPeers(msg *Message, peer *Peer) {
	logger.Debug("[P2P] Received GetPeers request from ", peer.Address)

	// Collect currently connected peer list
	peers := s.Node.GetPeers()
	peerInfos := make([]PeerInfo, 0, len(peers))

	for _, p := range peers {
		// Exclude requester itself
		if p.ID == peer.ID {
			continue
		}
		// Share only Active peers
		if p.State != PeerStateActive {
			continue
		}

		// Extract host and port from peer address
		host, port := parseAddress(p.Address)
		if host == "" {
			continue
		}

		// Use ListenPort for Inbound connection
		peerPort := port
		if p.Inbound && p.Port > 0 {
			peerPort = p.Port
		}

		peerInfos = append(peerInfos, PeerInfo{
			ID:      p.ID,
			Address: host,
			Port:    peerPort,
		})
	}

	logger.Info("[P2P] Sending ", len(peerInfos), " peers to ", peer.Address)

	// Send response
	payload := PeersPayload{Peers: peerInfos}
	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		logger.Error("[P2P] Failed to marshal peers payload: ", err)
		return
	}

	response := NewMessage(MsgTypePeers, payloadBytes, s.Node.ID)
	if err := s.Node.sendMessage(peer, response); err != nil {
		logger.Error("[P2P] Failed to send peers message: ", err)
	}
}

// handlePeers processes peer list reception
func (s *P2PService) handlePeers(msg *Message, peer *Peer) {
	var payload PeersPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal peers payload: ", err)
		return
	}

	logger.Debug("[P2P] Received ", len(payload.Peers), " peers from ", peer.Address)

	// Attempt connection to received peers
	for _, peerInfo := range payload.Peers {
		// Check if already connected peer
		if s.isConnectedToPeer(peerInfo.ID) {
			continue
		}

		// Check if self
		if peerInfo.ID == s.Node.ID {
			continue
		}

		// Check if already connecting peer (prevent duplicate connection)
		if !s.tryMarkConnecting(peerInfo.ID) {
			continue
		}

		// Attempt connection (async)
		address := fmt.Sprintf("%s:%d", peerInfo.Address, peerInfo.Port)
		go func(addr string, id string) {
			defer s.unmarkConnecting(id)
			if err := s.Node.Connect(addr); err != nil {
				logger.Debug("[P2P] Failed to connect to discovered peer ", addr, ": ", err)
			} else {
				logger.Info("[P2P] Connected to discovered peer: ", addr)
			}
		}(address, peerInfo.ID)
	}
}

// tryMarkConnecting marks as connecting (returns false if already connecting)
func (s *P2PService) tryMarkConnecting(peerID string) bool {
	s.connectingPeersMu.Lock()
	defer s.connectingPeersMu.Unlock()

	// Clean up items older than 30 seconds
	threshold := time.Now().Add(-30 * time.Second)
	for id, t := range s.connectingPeers {
		if t.Before(threshold) {
			delete(s.connectingPeers, id)
		}
	}

	// Check if already connecting
	if _, exists := s.connectingPeers[peerID]; exists {
		return false
	}

	s.connectingPeers[peerID] = time.Now()
	return true
}

// unmarkConnecting marks connection attempt completion
func (s *P2PService) unmarkConnecting(peerID string) {
	s.connectingPeersMu.Lock()
	defer s.connectingPeersMu.Unlock()
	delete(s.connectingPeers, peerID)
}

// isConnectedToPeer checks if already connected to specific peer
func (s *P2PService) isConnectedToPeer(peerID string) bool {
	s.Node.mu.RLock()
	defer s.Node.mu.RUnlock()

	_, exists := s.Node.Peers[peerID]
	return exists
}

// RequestPeers requests peer list
func (s *P2PService) RequestPeers(peer *Peer) error {
	msg := NewMessage(MsgTypeGetPeers, nil, s.Node.ID)
	return s.Node.sendMessage(peer, msg)
}

// parseAddress extracts host and port from address
func parseAddress(address string) (string, int) {
	// Split from "host:port" format
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == ':' {
			host := address[:i]
			portStr := address[i+1:]
			port := 0
			fmt.Sscanf(portStr, "%d", &port)
			return host, port
		}
	}
	return address, 0
}

// generateMessageID generates unique message ID
func (s *P2PService) generateMessageID(msg *Message) string {
	// Generate unique ID with message type + original sender + payload hash
	hash := sha256.Sum256(msg.Payload)
	return fmt.Sprintf("%d:%s:%s", msg.Type, msg.From, hex.EncodeToString(hash[:8]))
}

// hasSeenMessage checks if message already seen
func (s *P2PService) hasSeenMessage(msgID string) bool {
	s.seenMessagesMu.RLock()
	defer s.seenMessagesMu.RUnlock()
	_, exists := s.seenMessages[msgID]
	return exists
}

// markMessageSeen marks message as seen
func (s *P2PService) markMessageSeen(msgID string) {
	s.seenMessagesMu.Lock()
	defer s.seenMessagesMu.Unlock()
	s.seenMessages[msgID] = time.Now()

	// Clean up old items if cache grows too large (over 1000)
	if len(s.seenMessages) > 1000 {
		threshold := time.Now().Add(-60 * time.Second)
		for id, t := range s.seenMessages {
			if t.Before(threshold) {
				delete(s.seenMessages, id)
			}
		}
	}
}

// hasSeenProposal checks if Proposal already seen
func (s *P2PService) hasSeenProposal(key string) bool {
	s.seenProposalsMu.RLock()
	defer s.seenProposalsMu.RUnlock()
	_, exists := s.seenProposals[key]
	return exists
}

// markProposalSeen marks Proposal as seen
func (s *P2PService) markProposalSeen(key string) {
	s.seenProposalsMu.Lock()
	defer s.seenProposalsMu.Unlock()
	s.seenProposals[key] = time.Now()

	// Clean up cache (over 500, older than 30s)
	if len(s.seenProposals) > 500 {
		threshold := time.Now().Add(-30 * time.Second)
		for id, t := range s.seenProposals {
			if t.Before(threshold) {
				delete(s.seenProposals, id)
			}
		}
	}
}

// hasSeenVote checks if Vote already seen
func (s *P2PService) hasSeenVote(key string) bool {
	s.seenVotesMu.RLock()
	defer s.seenVotesMu.RUnlock()
	_, exists := s.seenVotes[key]
	return exists
}

// markVoteSeen marks Vote as seen
func (s *P2PService) markVoteSeen(key string) {
	s.seenVotesMu.Lock()
	defer s.seenVotesMu.Unlock()
	s.seenVotes[key] = time.Now()

	// Clean up cache (over 2000, older than 30s)
	if len(s.seenVotes) > 2000 {
		threshold := time.Now().Add(-30 * time.Second)
		for id, t := range s.seenVotes {
			if t.Before(threshold) {
				delete(s.seenVotes, id)
			}
		}
	}
}

// relayMessage relays message to other peers (excluding sender, prevent duplicates)
func (s *P2PService) relayMessage(msg *Message, sender *Peer) {
	// Duplicate check - do not relay already seen messages
	msgID := s.generateMessageID(msg)
	if s.hasSeenMessage(msgID) {
		return // 이미 처리한 메시지
	}
	s.markMessageSeen(msgID)

	s.Node.mu.RLock()
	peers := make([]*Peer, 0, len(s.Node.Peers))
	for _, peer := range s.Node.Peers {
		// Exclude sender and original sender
		if peer.State == PeerStateActive && peer.ID != sender.ID && peer.ID != msg.From {
			peers = append(peers, peer)
		}
	}
	s.Node.mu.RUnlock()

	if len(peers) == 0 {
		return
	}

	logger.Debug("[P2P] Relaying message type ", msg.Type, " to ", len(peers), " peers")

	for _, peer := range peers {
		go func(p *Peer) {
			if err := s.Node.sendMessage(p, msg); err != nil {
				logger.Debug("[P2P] Failed to relay to ", p.Address, ": ", err)
			}
		}(peer)
	}
}

// BroadcastBlock broadcasts block
func (s *P2PService) BroadcastBlock(block *core.Block) error {
	blockData, err := utils.SerializeData(block, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize block: %w", err)
	}

	payload := NewBlockPayload{
		Height:    block.Header.Height,
		Hash:      block.Header.Hash,
		BlockData: blockData,
	}
	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal block payload: %w", err)
	}

	msg := NewMessage(MsgTypeNewBlock, payloadBytes, s.Node.ID)
	s.Node.Broadcast(msg)
	return nil
}

// BroadcastTx broadcasts transaction
func (s *P2PService) BroadcastTx(tx *core.Transaction) error {
	txData, err := utils.SerializeData(tx, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize tx: %w", err)
	}

	payload := NewTxPayload{
		TxID:   tx.ID,
		TxData: txData,
	}
	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal tx payload: %w", err)
	}

	msg := NewMessage(MsgTypeNewTx, payloadBytes, s.Node.ID)
	s.Node.Broadcast(msg)
	return nil
}

// RequestBlocks requests blocks in specific range
func (s *P2PService) RequestBlocks(peer *Peer, startHeight, endHeight uint64) error {
	payload := GetBlocksPayload{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}
	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal get blocks payload: %w", err)
	}

	msg := NewMessage(MsgTypeGetBlocks, payloadBytes, s.Node.ID)
	return s.Node.sendMessage(peer, msg)
}

// SyncBlocks syncs blocks with peer
func (s *P2PService) SyncBlocks() error {
	if s.Blockchain == nil {
		logger.Error("[Sync] Blockchain not set")
		return fmt.Errorf("blockchain not set")
	}

	peers := s.Node.GetPeers()
	logger.Debug("[Sync] Available peers: ", len(peers))
	if len(peers) == 0 {
		return fmt.Errorf("no peers available")
	}

	// Find peer with highest block
	var bestPeer *Peer
	var bestHeight uint64

	for _, peer := range peers {
		logger.Debug("[Sync] Peer ", peer.ID, " state=", peer.State, " height=", peer.BestHeight)
		if peer.State == PeerStateActive && peer.BestHeight >= bestHeight {
			// Select peer with equal or higher height (select first active peer)
			if bestPeer == nil || peer.BestHeight > bestHeight {
				bestHeight = peer.BestHeight
				bestPeer = peer
			}
		}
	}

	if bestPeer == nil {
		logger.Debug("[Sync] No active peers with higher blocks")
		return nil // Not an error, just no need to sync
	}

	// Check current height
	currentHeight, err := s.Blockchain.GetLatestHeight()
	if err != nil {
		// Chain is empty (no genesis block) - request from height 0
		logger.Info("[Sync] Empty chain, requesting from genesis (height 0)")
		currentHeight = 0
		if bestHeight == 0 {
			return nil
		}
		logger.Info("[Sync] Requesting blocks 0 to ", bestHeight, " from peer ", bestPeer.ID)
		return s.RequestBlocks(bestPeer, 0, bestHeight)
	}

	logger.Info("[Sync] Current height: ", currentHeight, ", Best peer height: ", bestHeight)

	if bestHeight <= currentHeight {
		logger.Debug("[Sync] Already synced (current=", currentHeight, ", best=", bestHeight, ")")
		return nil // Already synced
	}

	logger.Info("[Sync] Requesting blocks ", currentHeight+1, " to ", bestHeight, " from peer ", bestPeer.ID)
	return s.RequestBlocks(bestPeer, currentHeight+1, bestHeight)
}

// AddBootNode adds bootnode
func (s *P2PService) AddBootNode(address string) {
	s.Node.BootNodes = append(s.Node.BootNodes, address)
}

// Connect connects to peer
func (s *P2PService) Connect(address string) error {
	return s.Node.Connect(address)
}

// GetPeerCount returns connected peer count
func (s *P2PService) GetPeerCount() int {
	return s.Node.GetPeerCount()
}

// GetPeers returns peer list
func (s *P2PService) GetPeers() []*Peer {
	return s.Node.GetPeers()
}

// GetNodeID returns node ID
func (s *P2PService) GetNodeID() string {
	return s.Node.ID
}

// IsRunning checks if running
func (s *P2PService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// BlocksPayload block list payload
type BlocksPayload struct {
	BlocksData [][]byte `json:"blocksData"` // Serialized blocks
}

// BroadcastProposal broadcasts block proposal (for consensus)
func (s *P2PService) BroadcastProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block, proposerID string, signature prt.Signature) error {
	// Serialize block
	blockData, err := utils.SerializeData(block, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize block: %w", err)
	}

	payload := ProposalPayload{
		Height:     height,
		Round:      round,
		BlockHash:  blockHash,
		BlockData:  blockData,
		ProposerID: proposerID,
		Signature:  signature,
	}
	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal proposal payload: %w", err)
	}

	msg := NewMessage(MsgTypeProposal, payloadBytes, s.Node.ID)
	s.Node.Broadcast(msg)
	logger.Debug("[P2P] Broadcast proposal - height: ", height, ", round: ", round)
	return nil
}

// BroadcastVote broadcasts vote (for consensus)
func (s *P2PService) BroadcastVote(height uint64, round uint32, blockHash prt.Hash, voteType uint8, voterID string, signature prt.Signature) error {
	payload := VotePayload{
		Height:    height,
		Round:     round,
		BlockHash: blockHash,
		VoteType:  voteType,
		VoterID:   voterID,
		Signature: signature,
	}
	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal vote payload: %w", err)
	}

	msg := NewMessage(MsgTypeVote, payloadBytes, s.Node.ID)
	s.Node.Broadcast(msg)
	return nil
}
