package p2p

import (
	"fmt"
	"sync"

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
	blockHandler func(*core.Block)
	txHandler    func(*core.Transaction)

	running bool
}

// NewP2PService 새 P2P 서비스 생성
func NewP2PService(address string, port int, networkID string, blockchain *core.BlockChain) (*P2PService, error) {
	node, err := NewNode(address, port, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	service := &P2PService{
		Node:       node,
		Blockchain: blockchain,
		running:    false,
	}

	// 메시지 핸들러 설정
	node.MessageHandler = service.handleMessage

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
	}
}

// handlePing Ping 처리
func (s *P2PService) handlePing(peer *Peer) {
	pong := NewMessage(MsgTypePong, nil, s.Node.ID)
	s.Node.sendMessage(peer, pong)
}

// handleNewBlock 새 블록 수신 처리
func (s *P2PService) handleNewBlock(msg *Message, peer *Peer) {
	var payload NewBlockPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		return
	}

	// 블록 역직렬화
	var block core.Block
	if err := utils.DeserializeData(payload.BlockData, &block, utils.SerializationFormatGob); err != nil {
		return
	}

	// 블록 핸들러 호출
	s.mu.RLock()
	handler := s.blockHandler
	s.mu.RUnlock()

	if handler != nil {
		handler(&block)
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
		return
	}

	if s.Blockchain == nil {
		return
	}

	// 요청된 블록들 가져오기
	blocksData := make([][]byte, 0)
	for height := payload.StartHeight; height <= payload.EndHeight && len(blocksData) < 100; height++ {
		block, err := s.Blockchain.GetBlockByHeight(height)
		if err != nil {
			break
		}
		blockData, err := utils.SerializeData(block, utils.SerializationFormatGob)
		if err != nil {
			continue
		}
		blocksData = append(blocksData, blockData)
	}

	// 블록 응답 전송
	responsePayload := BlocksPayload{BlocksData: blocksData}
	payloadBytes, err := MarshalPayload(responsePayload)
	if err != nil {
		return
	}

	response := NewMessage(MsgTypeBlocks, payloadBytes, s.Node.ID)
	s.Node.sendMessage(peer, response)
}

// handleBlocks 블록 응답 처리
func (s *P2PService) handleBlocks(msg *Message, peer *Peer) {
	var payload BlocksPayload
	if err := UnmarshalPayload(msg.Payload, &payload); err != nil {
		return
	}

	// 수신된 블록들 처리
	s.mu.RLock()
	handler := s.blockHandler
	s.mu.RUnlock()

	if handler != nil {
		for _, blockData := range payload.BlocksData {
			var block core.Block
			if err := utils.DeserializeData(blockData, &block, utils.SerializationFormatGob); err != nil {
				continue
			}
			handler(&block)
		}
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
		return fmt.Errorf("blockchain not set")
	}

	peers := s.Node.GetPeers()
	if len(peers) == 0 {
		return fmt.Errorf("no peers available")
	}

	// 가장 높은 블록을 가진 피어 찾기
	var bestPeer *Peer
	var bestHeight uint64

	for _, peer := range peers {
		if peer.State == PeerStateActive && peer.BestHeight > bestHeight {
			bestHeight = peer.BestHeight
			bestPeer = peer
		}
	}

	if bestPeer == nil {
		return fmt.Errorf("no active peers found")
	}

	// 현재 높이보다 높은 블록 요청
	currentHeight, _ := s.Blockchain.GetLatestHeight()
	if bestHeight <= currentHeight {
		return nil // 이미 동기화됨
	}

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
func (s *P2PService) BroadcastProposal(height uint64, round uint32, blockHash prt.Hash, proposerID string) error {
	payload := ProposalPayload{
		Height:     height,
		Round:      round,
		BlockHash:  blockHash,
		ProposerID: proposerID,
	}
	payloadBytes, err := MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal proposal payload: %w", err)
	}

	msg := NewMessage(MsgTypeProposal, payloadBytes, s.Node.ID)
	s.Node.Broadcast(msg)
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
