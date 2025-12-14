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

// P2PService P2P 네트워킹 서비스
type P2PService struct {
	mu sync.RWMutex

	Node       *Node
	Blockchain *core.BlockChain

	// 메시지 핸들러
	blockHandler    func(*core.Block)
	txHandler       func(*core.Transaction)
	proposalHandler func(height uint64, round uint32, blockHash prt.Hash, block *core.Block)
	voteHandler     func(height uint64, round uint32, voteType uint8, blockHash prt.Hash, voterID prt.Address, signature prt.Signature)

	// 메시지 중복 방지 캐시 (무한 릴레이 방지)
	seenMessages   map[string]time.Time
	seenMessagesMu sync.RWMutex

	// Proposal/Vote 중복 방지 (height:round:proposer 기준)
	seenProposals   map[string]time.Time // key: "height:round:proposer"
	seenProposalsMu sync.RWMutex
	seenVotes       map[string]time.Time // key: "height:round:voteType:voter"
	seenVotesMu     sync.RWMutex

	running bool
}

// NewP2PService 새 P2P 서비스 생성
func NewP2PService(address string, port int, networkID string, blockchain *core.BlockChain) (*P2PService, error) {
	node, err := NewNode(address, port, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	service := &P2PService{
		Node:          node,
		Blockchain:    blockchain,
		seenMessages:  make(map[string]time.Time),
		seenProposals: make(map[string]time.Time),
		seenVotes:     make(map[string]time.Time),
		running:       false,
	}

	// 노드에도 블록체인 참조 설정 (핸드셰이크용)
	node.Blockchain = blockchain

	// 메시지 핸들러 설정
	node.MessageHandler = service.handleMessage

	// 피어 연결 완료 시 피어 목록 요청 (피어 디스커버리)
	node.OnPeerConnected = func(peer *Peer) {
		// 잠시 대기 후 피어 목록 요청 (핸드셰이크 완전히 끝난 후)
		go func() {
			// 100ms 대기
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

	// 주기적 피어 교환 (30초마다)
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

// Start P2P 서비스 시작
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

// Stop P2P 서비스 종료
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

// SetBlockHandler 블록 수신 핸들러 설정
func (s *P2PService) SetBlockHandler(handler func(*core.Block)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockHandler = handler
}

// SetTxHandler 트랜잭션 수신 핸들러 설정
func (s *P2PService) SetTxHandler(handler func(*core.Transaction)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txHandler = handler
}

// SetProposalHandler 블록 제안 수신 핸들러 설정
func (s *P2PService) SetProposalHandler(handler func(height uint64, round uint32, blockHash prt.Hash, block *core.Block)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proposalHandler = handler
}

// SetVoteHandler 투표 수신 핸들러 설정
func (s *P2PService) SetVoteHandler(handler func(height uint64, round uint32, voteType uint8, blockHash prt.Hash, voterID prt.Address, signature prt.Signature)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.voteHandler = handler
}

// handleMessage 수신 메시지 처리
func (s *P2PService) handleMessage(msg *Message, peer *Peer) {
	switch msg.Type {
	case MsgTypePing:
		s.handlePing(peer)
	case MsgTypePong:
		// Pong은 LastSeen 업데이트만 (handlePeer에서 처리됨)
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

// handlePing Ping 처리
func (s *P2PService) handlePing(peer *Peer) {
	pong := NewMessage(MsgTypePong, nil, s.Node.ID)
	s.Node.sendMessage(peer, pong)
}

// handleNewBlock 새 블록 수신 처리
func (s *P2PService) handleNewBlock(msg *Message, peer *Peer) {
	logger.Debug("[P2P] Received new block message from peer: ", peer.Address)

	var payload NewBlockPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal block payload: ", err)
		return
	}

	logger.Debug("[P2P] Block payload - height: ", payload.Height)

	// 블록 역직렬화
	var block core.Block
	if err := utils.DeserializeData(payload.BlockData, &block, utils.SerializationFormatGob); err != nil {
		logger.Error("[P2P] Failed to deserialize block: ", err)
		return
	}

	logger.Debug("[P2P] Received block height: ", block.Header.Height, " hash: ", fmt.Sprintf("%x", block.Header.Hash[:8]))

	// 블록 핸들러 호출
	s.mu.RLock()
	handler := s.blockHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(&block)
	} else {
		logger.Warn("[P2P] Block handler not set!")
	}
}

// handleNewTx 새 트랜잭션 수신 처리
func (s *P2PService) handleNewTx(msg *Message, peer *Peer) {
	var payload NewTxPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		return
	}

	// 트랜잭션 역직렬화
	var tx core.Transaction
	if err := utils.DeserializeData(payload.TxData, &tx, utils.SerializationFormatGob); err != nil {
		return
	}

	// 트랜잭션 핸들러 호출
	s.mu.RLock()
	handler := s.txHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(&tx)
	}
}

// handleGetBlocks 블록 요청 처리
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

	// 요청된 블록들 가져오기
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

	// 블록 응답 전송
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

// handleBlocks 블록 응답 처리
func (s *P2PService) handleBlocks(msg *Message, peer *Peer) {
	var payload BlocksPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal Blocks payload: ", err)
		return
	}

	logger.Info("[P2P] Received ", len(payload.BlocksData), " blocks from ", peer.Address)

	// 수신된 블록들 처리
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

// handleProposal 블록 제안 수신 처리
func (s *P2PService) handleProposal(msg *Message, peer *Peer) {
	logger.Debug("[P2P] Received proposal message from peer: ", peer.Address)

	var payload ProposalPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal proposal payload: ", err)
		return
	}

	// 중복 Proposal 체크 (같은 height:round:proposer는 한 번만 처리)
	proposalKey := fmt.Sprintf("%d:%d:%s", payload.Height, payload.Round, payload.ProposerID)
	if s.hasSeenProposal(proposalKey) {
		logger.Debug("[P2P] Duplicate proposal ignored: ", proposalKey)
		return
	}
	s.markProposalSeen(proposalKey)

	// 블록 역직렬화
	var block core.Block
	if err := utils.DeserializeData(payload.BlockData, &block, utils.SerializationFormatGob); err != nil {
		logger.Error("[P2P] Failed to deserialize proposed block: ", err)
		return
	}

	logger.Info("[P2P] Received proposal - height: ", payload.Height, ", round: ", payload.Round, ", proposer: ", payload.ProposerID[:16])

	// 다른 피어들에게 릴레이 (발신자 제외)
	s.relayMessage(msg, peer)

	// 제안 핸들러 호출
	s.mu.RLock()
	handler := s.proposalHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(payload.Height, payload.Round, payload.BlockHash, &block)
	} else {
		logger.Warn("[P2P] Proposal handler not set!")
	}
}

// handleVote 투표 수신 처리
func (s *P2PService) handleVote(msg *Message, peer *Peer) {
	logger.Debug("[P2P] Received vote message from peer: ", peer.Address)

	var payload VotePayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal vote payload: ", err)
		return
	}

	// 중복 Vote 체크 (같은 height:round:voteType:voter는 한 번만 처리)
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

	// 다른 피어들에게 릴레이 (발신자 제외)
	s.relayMessage(msg, peer)

	// VoterID를 Address로 변환
	voterAddr, err := utils.StringToAddress(payload.VoterID)
	if err != nil {
		logger.Error("[P2P] Failed to parse voter address: ", err)
		return
	}

	// 투표 핸들러 호출
	s.mu.RLock()
	handler := s.voteHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(payload.Height, payload.Round, payload.VoteType, payload.BlockHash, voterAddr, payload.Signature)
	} else {
		logger.Warn("[P2P] Vote handler not set!")
	}
}

// handleGetPeers 피어 목록 요청 처리
func (s *P2PService) handleGetPeers(msg *Message, peer *Peer) {
	logger.Debug("[P2P] Received GetPeers request from ", peer.Address)

	// 현재 연결된 피어 목록 수집
	peers := s.Node.GetPeers()
	peerInfos := make([]PeerInfo, 0, len(peers))

	for _, p := range peers {
		// 요청자 자신은 제외
		if p.ID == peer.ID {
			continue
		}
		// Active 상태인 피어만 공유
		if p.State != PeerStateActive {
			continue
		}

		// 피어 주소에서 호스트와 포트 추출
		host, port := parseAddress(p.Address)
		if host == "" {
			continue
		}

		// Inbound 연결인 경우 ListenPort 사용
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

	// 응답 전송
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

// handlePeers 피어 목록 수신 처리
func (s *P2PService) handlePeers(msg *Message, peer *Peer) {
	var payload PeersPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		logger.Error("[P2P] Failed to unmarshal peers payload: ", err)
		return
	}

	logger.Info("[P2P] Received ", len(payload.Peers), " peers from ", peer.Address)

	// 수신한 피어들에게 연결 시도
	for _, peerInfo := range payload.Peers {
		// 이미 연결된 피어인지 확인
		if s.isConnectedToPeer(peerInfo.ID) {
			continue
		}

		// 자기 자신인지 확인
		if peerInfo.ID == s.Node.ID {
			continue
		}

		// 연결 시도 (비동기)
		address := fmt.Sprintf("%s:%d", peerInfo.Address, peerInfo.Port)
		go func(addr string, id string) {
			if err := s.Node.Connect(addr); err != nil {
				logger.Debug("[P2P] Failed to connect to discovered peer ", addr, ": ", err)
			} else {
				logger.Info("[P2P] Connected to discovered peer: ", addr)
			}
		}(address, peerInfo.ID)
	}
}

// isConnectedToPeer 특정 피어에 이미 연결되어 있는지 확인
func (s *P2PService) isConnectedToPeer(peerID string) bool {
	s.Node.mu.RLock()
	defer s.Node.mu.RUnlock()

	_, exists := s.Node.Peers[peerID]
	return exists
}

// RequestPeers 피어 목록 요청
func (s *P2PService) RequestPeers(peer *Peer) error {
	msg := NewMessage(MsgTypeGetPeers, nil, s.Node.ID)
	return s.Node.sendMessage(peer, msg)
}

// parseAddress 주소에서 호스트와 포트 추출
func parseAddress(address string) (string, int) {
	// "host:port" 형태에서 분리
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

// generateMessageID 메시지 고유 ID 생성
func (s *P2PService) generateMessageID(msg *Message) string {
	// 메시지 타입 + 원본 발신자 + 페이로드 해시로 고유 ID 생성
	hash := sha256.Sum256(msg.Payload)
	return fmt.Sprintf("%d:%s:%s", msg.Type, msg.From, hex.EncodeToString(hash[:8]))
}

// hasSeenMessage 이미 본 메시지인지 확인
func (s *P2PService) hasSeenMessage(msgID string) bool {
	s.seenMessagesMu.RLock()
	defer s.seenMessagesMu.RUnlock()
	_, exists := s.seenMessages[msgID]
	return exists
}

// markMessageSeen 메시지를 본 것으로 표시
func (s *P2PService) markMessageSeen(msgID string) {
	s.seenMessagesMu.Lock()
	defer s.seenMessagesMu.Unlock()
	s.seenMessages[msgID] = time.Now()

	// 캐시가 너무 커지면 오래된 항목 정리 (1000개 초과 시)
	if len(s.seenMessages) > 1000 {
		threshold := time.Now().Add(-60 * time.Second)
		for id, t := range s.seenMessages {
			if t.Before(threshold) {
				delete(s.seenMessages, id)
			}
		}
	}
}

// hasSeenProposal 이미 본 Proposal인지 확인
func (s *P2PService) hasSeenProposal(key string) bool {
	s.seenProposalsMu.RLock()
	defer s.seenProposalsMu.RUnlock()
	_, exists := s.seenProposals[key]
	return exists
}

// markProposalSeen Proposal을 본 것으로 표시
func (s *P2PService) markProposalSeen(key string) {
	s.seenProposalsMu.Lock()
	defer s.seenProposalsMu.Unlock()
	s.seenProposals[key] = time.Now()

	// 캐시 정리 (500개 초과 시, 30초 이상 된 항목)
	if len(s.seenProposals) > 500 {
		threshold := time.Now().Add(-30 * time.Second)
		for id, t := range s.seenProposals {
			if t.Before(threshold) {
				delete(s.seenProposals, id)
			}
		}
	}
}

// hasSeenVote 이미 본 Vote인지 확인
func (s *P2PService) hasSeenVote(key string) bool {
	s.seenVotesMu.RLock()
	defer s.seenVotesMu.RUnlock()
	_, exists := s.seenVotes[key]
	return exists
}

// markVoteSeen Vote를 본 것으로 표시
func (s *P2PService) markVoteSeen(key string) {
	s.seenVotesMu.Lock()
	defer s.seenVotesMu.Unlock()
	s.seenVotes[key] = time.Now()

	// 캐시 정리 (2000개 초과 시, 30초 이상 된 항목)
	if len(s.seenVotes) > 2000 {
		threshold := time.Now().Add(-30 * time.Second)
		for id, t := range s.seenVotes {
			if t.Before(threshold) {
				delete(s.seenVotes, id)
			}
		}
	}
}

// relayMessage 메시지를 다른 피어들에게 릴레이 (발신자 제외, 중복 방지)
func (s *P2PService) relayMessage(msg *Message, sender *Peer) {
	// 중복 체크 - 이미 본 메시지는 릴레이하지 않음
	msgID := s.generateMessageID(msg)
	if s.hasSeenMessage(msgID) {
		return // 이미 처리한 메시지
	}
	s.markMessageSeen(msgID)

	s.Node.mu.RLock()
	peers := make([]*Peer, 0, len(s.Node.Peers))
	for _, peer := range s.Node.Peers {
		// 발신자와 원본 발신자는 제외
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

// BroadcastBlock 블록 브로드캐스트
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

// BroadcastTx 트랜잭션 브로드캐스트
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

// RequestBlocks 특정 범위의 블록 요청
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

// SyncBlocks 피어와 블록 동기화
func (s *P2PService) SyncBlocks() error {
	if s.Blockchain == nil {
		logger.Error("[Sync] Blockchain not set")
		return fmt.Errorf("blockchain not set")
	}

	peers := s.Node.GetPeers()
	logger.Info("[Sync] Available peers: ", len(peers))
	if len(peers) == 0 {
		return fmt.Errorf("no peers available")
	}

	// 가장 높은 블록을 가진 피어 찾기
	var bestPeer *Peer
	var bestHeight uint64

	for _, peer := range peers {
		logger.Info("[Sync] Peer ", peer.ID, " state=", peer.State, " height=", peer.BestHeight)
		if peer.State == PeerStateActive && peer.BestHeight >= bestHeight {
			// 높이가 같거나 높은 피어 선택 (첫 번째 active 피어라도 선택되도록)
			if bestPeer == nil || peer.BestHeight > bestHeight {
				bestHeight = peer.BestHeight
				bestPeer = peer
			}
		}
	}

	if bestPeer == nil {
		logger.Debug("[Sync] No active peers with higher blocks")
		return nil // 에러가 아니라 그냥 동기화할 필요 없음
	}

	// 현재 높이 확인
	currentHeight, err := s.Blockchain.GetLatestHeight()
	if err != nil {
		// 체인이 비어있는 경우 (제네시스 블록도 없음) - height 0부터 요청
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
		logger.Info("[Sync] Already synced (current=", currentHeight, ", best=", bestHeight, ")")
		return nil // 이미 동기화됨
	}

	logger.Info("[Sync] Requesting blocks ", currentHeight+1, " to ", bestHeight, " from peer ", bestPeer.ID)
	return s.RequestBlocks(bestPeer, currentHeight+1, bestHeight)
}

// AddBootNode 부트노드 추가
func (s *P2PService) AddBootNode(address string) {
	s.Node.BootNodes = append(s.Node.BootNodes, address)
}

// Connect 피어에 연결
func (s *P2PService) Connect(address string) error {
	return s.Node.Connect(address)
}

// GetPeerCount 연결된 피어 수
func (s *P2PService) GetPeerCount() int {
	return s.Node.GetPeerCount()
}

// GetPeers 피어 목록
func (s *P2PService) GetPeers() []*Peer {
	return s.Node.GetPeers()
}

// GetNodeID 노드 ID
func (s *P2PService) GetNodeID() string {
	return s.Node.ID
}

// IsRunning 실행 중인지 확인
func (s *P2PService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// BlocksPayload 블록 목록 페이로드
type BlocksPayload struct {
	BlocksData [][]byte `json:"blocksData"` // 직렬화된 블록들
}

// BroadcastProposal 블록 제안 브로드캐스트 (컨센서스용)
func (s *P2PService) BroadcastProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block, proposerID string, signature prt.Signature) error {
	// 블록 직렬화
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

// BroadcastVote 투표 브로드캐스트 (컨센서스용)
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
