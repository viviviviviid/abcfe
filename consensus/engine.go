package consensus

import (
	"fmt"
	"sync"
	"time"

	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/common/utils"
	"github.com/abcfe/abcfe-node/core"
	prt "github.com/abcfe/abcfe-node/protocol"
)

// ConsensusEngine 컨센서스 엔진 (실행 로직)
type ConsensusEngine struct {
	mu sync.RWMutex

	consensus  *Consensus
	blockchain *core.BlockChain

	// 투표 관리
	prevotes   *VoteSet
	precommits *VoteSet

	// 현재 제안된 블록
	proposedBlock *core.Block

	// 콜백
	onBlockCommit func(*core.Block) // 블록 커밋 시 호출 (P2P 브로드캐스트용)

	// 실행 제어
	running bool
	stopCh  chan struct{}
}

// NewConsensusEngine 새 컨센서스 엔진 생성
func NewConsensusEngine(consensus *Consensus, blockchain *core.BlockChain) *ConsensusEngine {
	return &ConsensusEngine{
		consensus:  consensus,
		blockchain: blockchain,
		stopCh:     make(chan struct{}),
	}
}

// SetBlockCommitCallback 블록 커밋 콜백 설정
func (e *ConsensusEngine) SetBlockCommitCallback(callback func(*core.Block)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onBlockCommit = callback
}

// Start 컨센서스 엔진 시작
func (e *ConsensusEngine) Start() error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("consensus engine already running")
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	// 현재 블록체인 높이로 초기화
	height, _ := e.blockchain.GetLatestHeight()
	e.consensus.UpdateHeight(height + 1)

	go e.runConsensusLoop()

	logger.Info("[Consensus] Engine started at height ", height+1)
	return nil
}

// Stop 컨센서스 엔진 종료
func (e *ConsensusEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.running = false
	close(e.stopCh)
	logger.Info("[Consensus] Engine stopped")
}

// runConsensusLoop 컨센서스 메인 루프
func (e *ConsensusEngine) runConsensusLoop() {
	ticker := time.NewTicker(time.Duration(BlockProduceTimeMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.runRound()
		}
	}
}

// runRound 단일 라운드 실행
func (e *ConsensusEngine) runRound() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 검증자가 없으면 단독 모드로 블록 생성
	validators := e.consensus.ValidatorSet.GetActiveValidators()

	if len(validators) == 0 {
		// 단독 노드 모드 - 바로 블록 생성
		e.produceBlockSolo()
		return
	}

	// 현재 제안자 확인
	proposer := e.consensus.GetCurrentProposer()
	if proposer == nil {
		logger.Warn("[Consensus] No proposer selected")
		e.consensus.IncrementRound()
		return
	}

	// 내가 제안자인지 확인
	if e.consensus.IsLocalProposer() {
		e.proposeBlock()
	}

	// 투표 처리는 P2P를 통해 수신된 메시지로 처리
	// 간단한 구현을 위해 단일 노드에서는 바로 커밋
	if e.proposedBlock != nil && len(validators) == 1 {
		e.commitBlock(e.proposedBlock)
	}
}

// produceBlockSolo 단독 노드 모드에서 블록 생성
func (e *ConsensusEngine) produceBlockSolo() {
	// 현재 높이 확인
	currentHeight, err := e.blockchain.GetLatestHeight()
	if err != nil {
		currentHeight = 0
	}

	// 이전 블록 해시 가져오기 (제네시스 블록의 해시도 포함)
	var prevHash prt.Hash
	prevBlock, err := e.blockchain.GetBlockByHeight(currentHeight)
	if err == nil {
		prevHash = prevBlock.Header.Hash
	}

	// 새 블록 생성
	newBlock := e.blockchain.SetBlock(prevHash, currentHeight+1)
	if newBlock == nil {
		logger.Error("[Consensus] Failed to create block")
		return
	}

	// 블록 추가
	if success, err := e.blockchain.AddBlock(*newBlock); !success || err != nil {
		logger.Error("[Consensus] Failed to add block: ", err)
		return
	}

	logger.Info("[Consensus] Block ", newBlock.Header.Height, " created (hash: ", utils.HashToString(newBlock.Header.Hash)[:16], ", txs: ", len(newBlock.Transactions), ")")

	// 콜백 호출
	if e.onBlockCommit != nil {
		e.onBlockCommit(newBlock)
	}

	// 다음 높이로
	e.consensus.UpdateHeight(newBlock.Header.Height + 1)
}

// proposeBlock 블록 제안
func (e *ConsensusEngine) proposeBlock() {
	e.consensus.mu.Lock()
	e.consensus.State = StateProposing
	e.consensus.mu.Unlock()

	// 현재 높이 확인
	currentHeight, err := e.blockchain.GetLatestHeight()
	if err != nil {
		currentHeight = 0
	}

	// 이전 블록 해시 (제네시스 블록의 해시도 포함)
	var prevHash prt.Hash
	prevBlock, err := e.blockchain.GetBlockByHeight(currentHeight)
	if err == nil {
		prevHash = prevBlock.Header.Hash
	}

	// 새 블록 생성
	newBlock := e.blockchain.SetBlock(prevHash, currentHeight+1)
	if newBlock == nil {
		logger.Error("[Consensus] Failed to create proposed block")
		return
	}

	e.proposedBlock = newBlock

	// 투표셋 초기화
	e.prevotes = NewVoteSet(newBlock.Header.Height, e.consensus.CurrentRound, VoteTypePrevote)
	e.precommits = NewVoteSet(newBlock.Header.Height, e.consensus.CurrentRound, VoteTypePrecommit)

	logger.Info("[Consensus] Proposed block ", newBlock.Header.Height, " (hash: ", utils.HashToString(newBlock.Header.Hash)[:16], ")")

	e.consensus.mu.Lock()
	e.consensus.State = StateVoting
	e.consensus.mu.Unlock()
}

// HandleProposal 제안 메시지 처리 (P2P로부터)
func (e *ConsensusEngine) HandleProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if height != e.consensus.CurrentHeight || round != e.consensus.CurrentRound {
		return
	}

	// 블록 검증
	if err := e.blockchain.ValidateBlock(*block); err != nil {
		logger.Error("[Consensus] Invalid proposed block: ", err)
		return
	}

	e.proposedBlock = block
	e.prevotes = NewVoteSet(height, round, VoteTypePrevote)
	e.precommits = NewVoteSet(height, round, VoteTypePrecommit)

	// Prevote 전송 (로컬 검증자인 경우)
	if e.consensus.LocalValidator != nil {
		e.castVote(VoteTypePrevote, blockHash)
	}
}

// HandleVote 투표 메시지 처리 (P2P로부터)
func (e *ConsensusEngine) HandleVote(vote *Vote) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if vote.Height != e.consensus.CurrentHeight || vote.Round != e.consensus.CurrentRound {
		return
	}

	// 투표자의 voting power 조회
	validator := e.consensus.ValidatorSet.GetValidator(vote.VoterID)
	if validator == nil {
		return
	}

	totalPower := e.consensus.ValidatorSet.TotalVotingPower

	switch vote.Type {
	case VoteTypePrevote:
		if e.prevotes != nil {
			e.prevotes.AddVote(vote, validator.VotingPower)

			// 2/3 이상이면 precommit으로
			if e.prevotes.HasTwoThirdsMajority(totalPower) {
				e.castVote(VoteTypePrecommit, vote.BlockHash)
			}
		}

	case VoteTypePrecommit:
		if e.precommits != nil {
			e.precommits.AddVote(vote, validator.VotingPower)

			// 2/3 이상이면 커밋
			if e.precommits.HasTwoThirdsMajority(totalPower) && e.proposedBlock != nil {
				e.commitBlock(e.proposedBlock)
			}
		}
	}
}

// castVote 투표 생성 및 전송
func (e *ConsensusEngine) castVote(voteType VoteType, blockHash prt.Hash) {
	if e.consensus.LocalValidator == nil || e.consensus.LocalProposer == nil {
		return
	}

	// 서명 생성
	hashBytes := utils.HashToBytes(blockHash)
	sig, err := e.consensus.LocalProposer.signBlockHash(blockHash)
	if err != nil {
		logger.Error("[Consensus] Failed to sign vote: ", err)
		return
	}
	_ = hashBytes // 사용하지 않음

	vote := &Vote{
		Height:    e.consensus.CurrentHeight,
		Round:     e.consensus.CurrentRound,
		Type:      voteType,
		BlockHash: blockHash,
		VoterID:   e.consensus.LocalValidator.Address,
		Signature: sig,
		Timestamp: time.Now().Unix(),
	}

	// 로컬 투표 처리
	validator := e.consensus.LocalValidator
	totalPower := e.consensus.ValidatorSet.TotalVotingPower

	switch voteType {
	case VoteTypePrevote:
		if e.prevotes != nil {
			e.prevotes.AddVote(vote, validator.VotingPower)
			if e.prevotes.HasTwoThirdsMajority(totalPower) {
				e.castVote(VoteTypePrecommit, blockHash)
			}
		}
	case VoteTypePrecommit:
		if e.precommits != nil {
			e.precommits.AddVote(vote, validator.VotingPower)
			if e.precommits.HasTwoThirdsMajority(totalPower) && e.proposedBlock != nil {
				e.commitBlock(e.proposedBlock)
			}
		}
	}
}

// commitBlock 블록 커밋
func (e *ConsensusEngine) commitBlock(block *core.Block) {
	e.consensus.mu.Lock()
	e.consensus.State = StateCommitting
	e.consensus.mu.Unlock()

	// 블록 추가
	success, err := e.blockchain.AddBlock(*block)
	if !success || err != nil {
		logger.Error("[Consensus] Failed to commit block: ", err)
		e.consensus.IncrementRound()
		return
	}

	logger.Info("[Consensus] Block ", block.Header.Height, " committed (hash: ", utils.HashToString(block.Header.Hash)[:16], ", txs: ", len(block.Transactions), ")")

	// 콜백 호출 (P2P 브로드캐스트)
	if e.onBlockCommit != nil {
		e.onBlockCommit(block)
	}

	// 다음 높이로
	e.consensus.UpdateHeight(block.Header.Height + 1)
	e.proposedBlock = nil
	e.prevotes = nil
	e.precommits = nil
}

// GetStatus 현재 상태 조회
func (e *ConsensusEngine) GetStatus() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"running":       e.running,
		"state":         e.consensus.State,
		"currentHeight": e.consensus.CurrentHeight,
		"currentRound":  e.consensus.CurrentRound,
		"validators":    e.consensus.GetValidatorCount(),
		"totalStaked":   e.consensus.GetTotalStaked(),
	}
}

// IsRunning 실행 중인지 확인
func (e *ConsensusEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}
