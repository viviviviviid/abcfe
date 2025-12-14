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

// P2PBroadcaster P2P 브로드캐스트 인터페이스
type P2PBroadcaster interface {
	BroadcastProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block, proposerID string, signature prt.Signature) error
	BroadcastVote(height uint64, round uint32, blockHash prt.Hash, voteType uint8, voterID string, signature prt.Signature) error
}

// BlockSyncer 블록 동기화 인터페이스
type BlockSyncer interface {
	SyncBlocks() error
	GetPeerCount() int
}

// ConsensusEngine 컨센서스 엔진 (실행 로직)
type ConsensusEngine struct {
	mu sync.RWMutex

	consensus  *Consensus
	blockchain *core.BlockChain

	// P2P 브로드캐스터
	p2p P2PBroadcaster

	// 블록 동기화 (P2PService)
	syncer BlockSyncer

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

	// 라운드 타임아웃
	roundTimer          *time.Timer
	roundTimerMu        sync.Mutex
	consecutiveTimeouts int // 연속 타임아웃 카운터
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

// SetP2PBroadcaster P2P 브로드캐스터 설정
func (e *ConsensusEngine) SetP2PBroadcaster(p2p P2PBroadcaster) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.p2p = p2p
}

// SetBlockSyncer 블록 동기화 설정
func (e *ConsensusEngine) SetBlockSyncer(syncer BlockSyncer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.syncer = syncer
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

	// PoA: 블록체인 높이 기준으로 다음 블록의 제안자 결정
	// (컨센서스 높이가 아닌 실제 블록체인 높이 사용)
	currentHeight, err := e.blockchain.GetLatestHeight()
	if err != nil {
		currentHeight = 0
	}
	nextBlockHeight := currentHeight + 1

	// 컨센서스 높이 동기화 (블록체인 높이 기준)
	if e.consensus.CurrentHeight != nextBlockHeight {
		e.consensus.mu.Lock()
		e.consensus.CurrentHeight = nextBlockHeight
		e.consensus.CurrentRound = 0
		e.consensus.mu.Unlock()
	}

	// 다음 블록의 제안자 확인 (블록체인 높이 + 1)
	proposer := e.consensus.Selector.SelectProposer(nextBlockHeight, 0)
	if proposer == nil {
		logger.Warn("[Consensus] No proposer selected")
		return
	}

	// 디버그 로그: 현재 제안자 정보
	proposerAddr := utils.AddressToString(proposer.Address)
	localAddr := ""
	isLocalProposer := false
	if e.consensus.LocalValidator != nil {
		localAddr = utils.AddressToString(e.consensus.LocalValidator.Address)
		isLocalProposer = (proposer.Address == e.consensus.LocalValidator.Address)
	}
	logger.Info("[Consensus] BlockHeight ", currentHeight, " NextBlock ", nextBlockHeight, " Proposer: ", proposerAddr, " Local: ", localAddr, " IsLocal: ", isLocalProposer)

	// 내가 제안자인지 확인
	if isLocalProposer {
		e.proposeBlock()

		// BFT 모드: Proposal 브로드캐스트 후 투표 수집
		if e.proposedBlock != nil {
			e.broadcastProposal()
		}
	} else {
		// 비제안자: Proposal 대기
		// 이미 해당 높이의 VoteSet이 있으면 유지 (투표 수집 중일 수 있음)
		if e.prevotes == nil || e.prevotes.Height != nextBlockHeight {
			e.prevotes = NewVoteSet(nextBlockHeight, 0, VoteTypePrevote)
			e.precommits = NewVoteSet(nextBlockHeight, 0, VoteTypePrecommit)
			e.startRoundTimer()
		}
		// 타임아웃 타이머는 이미 실행 중이면 그대로 유지
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

	// 제안자 주소 (로컬 검증자 또는 빈 주소)
	var proposerAddr prt.Address
	if e.consensus.LocalValidator != nil {
		proposerAddr = e.consensus.LocalValidator.Address
	}

	// 새 블록 생성
	newBlock := e.blockchain.SetBlock(prevHash, currentHeight+1, proposerAddr)
	if newBlock == nil {
		logger.Error("[Consensus] Failed to create block")
		return
	}

	// 블록 서명 (로컬 제안자가 있는 경우)
	if e.consensus.LocalProposer != nil {
		sig, err := e.consensus.LocalProposer.signBlockHash(newBlock.Header.Hash)
		if err != nil {
			logger.Error("[Consensus] Failed to sign block: ", err)
		} else {
			newBlock.SignBlock(sig)
		}
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

// proposeBlock 블록 제안 (PoA 모드: 투표 없이 즉시 커밋)
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

	// 제안자 주소
	var proposerAddr prt.Address
	if e.consensus.LocalValidator != nil {
		proposerAddr = e.consensus.LocalValidator.Address
	}

	// 새 블록 생성
	newBlock := e.blockchain.SetBlock(prevHash, currentHeight+1, proposerAddr)
	if newBlock == nil {
		logger.Error("[Consensus] Failed to create proposed block")
		return
	}

	// 블록 서명
	if e.consensus.LocalProposer != nil {
		sig, err := e.consensus.LocalProposer.signBlockHash(newBlock.Header.Hash)
		if err != nil {
			logger.Error("[Consensus] Failed to sign proposed block: ", err)
		} else {
			newBlock.SignBlock(sig)
		}
	}

	e.proposedBlock = newBlock

	logger.Info("[Consensus] Proposed block ", newBlock.Header.Height, " (hash: ", utils.HashToString(newBlock.Header.Hash)[:16], ", proposer: ", utils.AddressToString(proposerAddr)[:16], ")")
}

// broadcastProposal Proposal 브로드캐스트 및 투표 시작 (BFT 모드)
func (e *ConsensusEngine) broadcastProposal() {
	if e.p2p == nil || e.proposedBlock == nil {
		return
	}

	if e.consensus.LocalValidator == nil {
		return
	}

	height := e.consensus.CurrentHeight
	round := e.consensus.CurrentRound
	blockHash := e.proposedBlock.Header.Hash
	proposerID := utils.AddressToString(e.consensus.LocalValidator.Address)

	// VoteSet 초기화
	e.prevotes = NewVoteSet(height, round, VoteTypePrevote)
	e.precommits = NewVoteSet(height, round, VoteTypePrecommit)

	// P2P로 Proposal 브로드캐스트
	if err := e.p2p.BroadcastProposal(
		height,
		round,
		blockHash,
		e.proposedBlock,
		proposerID,
		e.proposedBlock.Signature,
	); err != nil {
		logger.Error("[Consensus] Failed to broadcast proposal: ", err)
		return
	}

	logger.Info("[Consensus] Broadcast proposal at height ", height, " round ", round)

	// 라운드 타임아웃 타이머 시작
	e.startRoundTimer()

	// 로컬 prevote 투표 (제안자도 투표에 참여)
	e.castVote(VoteTypePrevote, blockHash)
}

// HandleProposal 제안 메시지 처리 (P2P로부터)
func (e *ConsensusEngine) HandleProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 이미 이 높이의 블록이 커밋되었는지 확인
	currentHeight, _ := e.blockchain.GetLatestHeight()
	if height <= currentHeight {
		logger.Debug("[Consensus] Proposal for already committed height ", height, ", ignoring")
		return
	}

	// 예상되는 다음 블록 높이
	expectedNextHeight := currentHeight + 1

	// 높이가 맞지 않으면 무시
	if height != expectedNextHeight {
		logger.Debug("[Consensus] Proposal height mismatch: expected ", expectedNextHeight, ", got ", height)
		return
	}

	// 먼저 해당 라운드의 제안자가 유효한지 확인 (라운드 동기화 전에)
	expectedProposer := e.consensus.Selector.SelectProposer(height, round)
	if expectedProposer == nil {
		logger.Error("[Consensus] No expected proposer for height ", height, " round ", round)
		return
	}
	if block.Proposer != expectedProposer.Address {
		logger.Debug("[Consensus] Invalid proposer for round ", round, ": expected ", utils.AddressToString(expectedProposer.Address)[:16], ", got ", utils.AddressToString(block.Proposer)[:16])
		return
	}

	// 유효한 Proposal이면 해당 높이/라운드로 동기화
	if e.consensus.CurrentHeight != height || e.consensus.CurrentRound != round {
		logger.Info("[Consensus] Syncing to height ", height, " round ", round, " (was ", e.consensus.CurrentHeight, "/", e.consensus.CurrentRound, ")")
		e.consensus.mu.Lock()
		e.consensus.CurrentHeight = height
		e.consensus.CurrentRound = round
		e.consensus.mu.Unlock()
	}

	// 블록 검증 (서명 포함)
	if err := e.blockchain.ValidateBlock(*block); err != nil {
		logger.Error("[Consensus] Invalid proposed block: ", err)
		return
	}

	logger.Info("[Consensus] Received valid proposal at height ", height, " round ", round, " from ", utils.AddressToString(block.Proposer)[:16])

	e.proposedBlock = block
	e.prevotes = NewVoteSet(height, round, VoteTypePrevote)
	e.precommits = NewVoteSet(height, round, VoteTypePrecommit)

	// 라운드 타임아웃 타이머 시작
	e.startRoundTimer()

	// Prevote 전송 (로컬 검증자인 경우)
	if e.consensus.LocalValidator != nil {
		e.castVote(VoteTypePrevote, blockHash)
	}
}

// HandleVote 투표 메시지 처리 (P2P로부터)
func (e *ConsensusEngine) HandleVote(vote *Vote) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 이미 커밋된 높이의 투표는 무시
	currentHeight, _ := e.blockchain.GetLatestHeight()
	if vote.Height <= currentHeight {
		return
	}

	// 다음 블록 높이가 아니면 무시
	expectedNextHeight := currentHeight + 1
	if vote.Height != expectedNextHeight {
		return
	}

	// 현재 라운드와 다르면 무시 (투표는 Proposal과 함께 동기화됨)
	// 단, VoteSet이 있고 라운드가 맞으면 처리
	if vote.Round != e.consensus.CurrentRound {
		logger.Debug("[Consensus] Vote round mismatch: current ", e.consensus.CurrentRound, ", got ", vote.Round)
		return
	}

	// 투표자의 voting power 조회
	validator := e.consensus.ValidatorSet.GetValidator(vote.VoterID)
	if validator == nil {
		logger.Debug("[Consensus] Unknown voter: ", utils.AddressToString(vote.VoterID)[:16])
		return
	}

	totalPower := e.consensus.ValidatorSet.TotalVotingPower
	voterAddr := utils.AddressToString(vote.VoterID)[:16]

	switch vote.Type {
	case VoteTypePrevote:
		if e.prevotes != nil {
			e.prevotes.AddVote(vote, validator.VotingPower)
			added := e.prevotes.AddVote(vote, validator.VotingPower)
			if added {
				logger.Debug("[Consensus] Prevote received from ", voterAddr, " (", e.prevotes.VotedPower, "/", totalPower, " = ", len(e.prevotes.Votes), " votes)")
			}

			// 2/3 이상이면 precommit으로
			if e.prevotes.HasTwoThirdsMajority(totalPower) {
				logger.Debug("[Consensus] Prevote 2/3+ reached at height ", vote.Height, " (", e.prevotes.VotedPower, "/", totalPower, ")")
				e.castVote(VoteTypePrecommit, vote.BlockHash)
			}
		}

	case VoteTypePrecommit:
		if e.precommits != nil {
			e.precommits.AddVote(vote, validator.VotingPower)
			added := e.precommits.AddVote(vote, validator.VotingPower)
			if added {
				logger.Debug("[Consensus] Precommit received from ", voterAddr, " (", e.precommits.VotedPower, "/", totalPower, " = ", len(e.precommits.Votes), " votes)")
			}

			// 2/3 이상이면 커밋
			if e.precommits.HasTwoThirdsMajority(totalPower) && e.proposedBlock != nil {
				logger.Debug("[Consensus] Precommit 2/3+ reached at height ", vote.Height, " (", e.precommits.VotedPower, "/", totalPower, "), committing block")
				e.commitBlockWithSignatures(e.proposedBlock)
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
	sig, err := e.consensus.LocalProposer.signBlockHash(blockHash)
	if err != nil {
		logger.Error("[Consensus] Failed to sign vote: ", err)
		return
	}

	vote := &Vote{
		Height:    e.consensus.CurrentHeight,
		Round:     e.consensus.CurrentRound,
		Type:      voteType,
		BlockHash: blockHash,
		VoterID:   e.consensus.LocalValidator.Address,
		Signature: sig,
		Timestamp: time.Now().Unix(),
	}

	// P2P로 투표 브로드캐스트
	if e.p2p != nil {
		voterID := utils.AddressToString(e.consensus.LocalValidator.Address)
		if err := e.p2p.BroadcastVote(
			vote.Height,
			vote.Round,
			vote.BlockHash,
			uint8(voteType),
			voterID,
			vote.Signature,
		); err != nil {
			logger.Error("[Consensus] Failed to broadcast vote: ", err)
		}
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
				logger.Info("[Consensus] Local precommit triggered 2/3+ at height ", e.consensus.CurrentHeight)
				e.commitBlockWithSignatures(e.proposedBlock)
			}
		}
	}
}

// commitBlock 블록 커밋 (단독 노드용 - 서명 없이)
func (e *ConsensusEngine) commitBlock(block *core.Block) {
	// 타임아웃 타이머 취소
	e.stopRoundTimer()

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

// commitBlockWithSignatures BFT 합의 후 블록 커밋 (검증자 서명 포함)
func (e *ConsensusEngine) commitBlockWithSignatures(block *core.Block) {
	// 타임아웃 타이머 취소
	e.stopRoundTimer()

	// 연속 타임아웃 카운터 리셋
	e.consecutiveTimeouts = 0

	e.consensus.mu.Lock()
	e.consensus.State = StateCommitting
	e.consensus.mu.Unlock()

	// precommits에서 CommitSignatures 생성
	if e.precommits != nil {
		var commitSigs []core.CommitSignature
		for _, vote := range e.precommits.Votes {
			commitSigs = append(commitSigs, core.CommitSignature{
				ValidatorAddress: vote.VoterID,
				Signature:        vote.Signature,
				Timestamp:        vote.Timestamp,
			})
		}
		block.CommitSignatures = commitSigs
		logger.Info("[Consensus] Block ", block.Header.Height, " has ", len(commitSigs), " commit signatures")
	}

	// 블록 추가
	success, err := e.blockchain.AddBlock(*block)
	if !success || err != nil {
		logger.Error("[Consensus] Failed to commit block: ", err)
		e.consensus.IncrementRound()
		return
	}

	logger.Info("[Consensus] Block ", block.Header.Height, " committed with BFT consensus (hash: ", utils.HashToString(block.Header.Hash)[:16], ", txs: ", len(block.Transactions), ", validators: ", len(block.CommitSignatures), ")")

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

// startRoundTimer 라운드 타임아웃 타이머 시작
func (e *ConsensusEngine) startRoundTimer() {
	e.roundTimerMu.Lock()
	defer e.roundTimerMu.Unlock()

	// 기존 타이머 취소
	if e.roundTimer != nil {
		e.roundTimer.Stop()
	}

	height := e.consensus.CurrentHeight
	round := e.consensus.CurrentRound

	e.roundTimer = time.AfterFunc(time.Duration(RoundTimeoutMs)*time.Millisecond, func() {
		e.handleRoundTimeout(height, round)
	})

	logger.Debug("[Consensus] Round timer started for height ", height, " round ", round, " (", RoundTimeoutMs, "ms)")
}

// stopRoundTimer 라운드 타임아웃 타이머 취소
func (e *ConsensusEngine) stopRoundTimer() {
	e.roundTimerMu.Lock()
	defer e.roundTimerMu.Unlock()

	if e.roundTimer != nil {
		e.roundTimer.Stop()
		e.roundTimer = nil
	}
}

// handleRoundTimeout 라운드 타임아웃 처리
func (e *ConsensusEngine) handleRoundTimeout(height uint64, round uint32) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 이미 다음 높이로 진행했거나 라운드가 변경된 경우 무시
	if e.consensus.CurrentHeight != height || e.consensus.CurrentRound != round {
		e.consecutiveTimeouts = 0 // 정상 진행 시 리셋
		return
	}

	e.consecutiveTimeouts++
	logger.Warn("[Consensus] Round timeout at height ", height, " round ", round, " (consecutive: ", e.consecutiveTimeouts, ")")

	// 연속 타임아웃이 3회 이상이면 블록 동기화 시도
	if e.consecutiveTimeouts >= 3 {
		logger.Warn("[Consensus] Too many consecutive timeouts, attempting block sync...")
		e.consecutiveTimeouts = 0

		// 현재 블록체인 높이 확인
		currentHeight, _ := e.blockchain.GetLatestHeight()

		// 블록 동기화 시도 (별도 고루틴에서)
		if e.syncer != nil && e.syncer.GetPeerCount() > 0 {
			go func() {
				if err := e.syncer.SyncBlocks(); err != nil {
					logger.Debug("[Consensus] Block sync during timeout: ", err)
				} else {
					logger.Info("[Consensus] Block sync completed after timeout")
				}
			}()
		}

		// 동기화 후 블록체인 높이가 변경되었으면 컨센서스 높이 업데이트
		newHeight, _ := e.blockchain.GetLatestHeight()
		if newHeight > currentHeight {
			logger.Info("[Consensus] Blockchain advanced from ", currentHeight, " to ", newHeight, ", updating consensus")
			e.consensus.mu.Lock()
			e.consensus.CurrentHeight = newHeight + 1
			e.consensus.CurrentRound = 0
			e.consensus.mu.Unlock()
			e.proposedBlock = nil
			e.prevotes = nil
			e.precommits = nil
			return
		}
	}

	// 라운드 증가 → 다음 제안자의 차례
	e.consensus.IncrementRound()
	e.proposedBlock = nil
	e.prevotes = nil
	e.precommits = nil

	// 다음 라운드의 제안자가 로컬 노드인지 확인
	nextRound := e.consensus.CurrentRound
	proposer := e.consensus.Selector.SelectProposer(height, nextRound)
	if proposer != nil && e.consensus.LocalValidator != nil {
		if proposer.Address == e.consensus.LocalValidator.Address {
			logger.Info("[Consensus] Local node is proposer for round ", nextRound, ", proposing block")
			e.proposeBlock()
			if e.proposedBlock != nil {
				e.broadcastProposalInternal()
			}
		}
	}

	// 다음 라운드 타이머 시작
	e.startRoundTimer()
}

// broadcastProposalInternal 내부용 Proposal 브로드캐스트 (뮤텍스 없이)
func (e *ConsensusEngine) broadcastProposalInternal() {
	if e.p2p == nil || e.proposedBlock == nil || e.consensus.LocalValidator == nil {
		return
	}

	height := e.consensus.CurrentHeight
	round := e.consensus.CurrentRound
	blockHash := e.proposedBlock.Header.Hash
	proposerID := utils.AddressToString(e.consensus.LocalValidator.Address)

	e.prevotes = NewVoteSet(height, round, VoteTypePrevote)
	e.precommits = NewVoteSet(height, round, VoteTypePrecommit)

	if err := e.p2p.BroadcastProposal(height, round, blockHash, e.proposedBlock, proposerID, e.proposedBlock.Signature); err != nil {
		logger.Error("[Consensus] Failed to broadcast proposal: ", err)
		return
	}

	logger.Info("[Consensus] Broadcast proposal at height ", height, " round ", round)
	e.castVote(VoteTypePrevote, blockHash)
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
