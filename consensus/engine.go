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

// P2PBroadcaster P2P broadcast interface
type P2PBroadcaster interface {
	BroadcastProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block, proposerID string, signature prt.Signature) error
	BroadcastVote(height uint64, round uint32, blockHash prt.Hash, voteType uint8, voterID string, signature prt.Signature) error
}

// BlockSyncer block sync interface
type BlockSyncer interface {
	SyncBlocks() error
	GetPeerCount() int
}

// ConsensusEngine consensus engine (execution logic)
type ConsensusEngine struct {
	mu sync.RWMutex

	consensus  *Consensus
	blockchain *core.BlockChain

	// P2P broadcaster
	p2p P2PBroadcaster

	// Block sync (P2PService)
	syncer BlockSyncer

	// Vote management
	prevotes   *VoteSet
	precommits *VoteSet

	// Currently proposed block
	proposedBlock *core.Block

	// Callback
	onBlockCommit func(*core.Block) // Called on block commit (for P2P broadcast)

	// Execution control
	running bool
	stopCh  chan struct{}

	// Round timeout
	roundTimer          *time.Timer
	roundTimerMu        sync.Mutex
	consecutiveTimeouts int // Consecutive timeout counter

	// Block interval control
	lastBlockTime int64 // Last block commit timestamp (Unix milliseconds)
}

// NewConsensusEngine creates a new consensus engine
func NewConsensusEngine(consensus *Consensus, blockchain *core.BlockChain) *ConsensusEngine {
	return &ConsensusEngine{
		consensus:  consensus,
		blockchain: blockchain,
		stopCh:     make(chan struct{}),
	}
}

// SetBlockCommitCallback sets block commit callback
func (e *ConsensusEngine) SetBlockCommitCallback(callback func(*core.Block)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onBlockCommit = callback
}

// SetP2PBroadcaster sets P2P broadcaster
func (e *ConsensusEngine) SetP2PBroadcaster(p2p P2PBroadcaster) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.p2p = p2p
}

// SetBlockSyncer sets block syncer
func (e *ConsensusEngine) SetBlockSyncer(syncer BlockSyncer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.syncer = syncer
}

// Start starts the consensus engine
func (e *ConsensusEngine) Start() error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("consensus engine already running")
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	// Initialize to current blockchain height
	height, _ := e.blockchain.GetLatestHeight()
	e.consensus.UpdateHeight(height + 1)

	go e.runConsensusLoop()

	logger.Info("[Consensus] Engine started at height ", height+1)
	return nil
}

// Stop stops the consensus engine
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

// runConsensusLoop consensus main loop
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

// runRound runs a single round
func (e *ConsensusEngine) runRound() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check minimum block interval
	now := time.Now().UnixMilli()
	if e.lastBlockTime > 0 {
		elapsed := now - e.lastBlockTime
		if elapsed < BlockIntervalMs {
			// Not enough time has passed since last block
			return
		}
	}

	// Create block in solo mode if no validators
	validators := e.consensus.ValidatorSet.GetActiveValidators()

	if len(validators) == 0 {
		// Solo node mode - create block immediately
		e.produceBlockSolo()
		return
	}

	// PoA: Determine next block proposer based on blockchain height
	// (Use actual blockchain height, not consensus height)
	currentHeight, err := e.blockchain.GetLatestHeight()
	if err != nil {
		currentHeight = 0
	}
	nextBlockHeight := currentHeight + 1

	// Sync consensus height (based on blockchain height)
	if e.consensus.CurrentHeight != nextBlockHeight {
		e.consensus.mu.Lock()
		e.consensus.CurrentHeight = nextBlockHeight
		e.consensus.CurrentRound = 0
		e.consensus.mu.Unlock()
	}

	// Check proposer for next block (blockchain height + 1)
	proposer := e.consensus.Selector.SelectProposer(nextBlockHeight, e.consensus.CurrentRound)
	if proposer == nil {
		logger.Warn("[Consensus] No proposer selected")
		return
	}

	// Debug log: Current proposer info
	proposerAddr := utils.AddressToString(proposer.Address)
	localAddr := ""
	isLocalProposer := false
	if e.consensus.LocalValidator != nil {
		localAddr = utils.AddressToString(e.consensus.LocalValidator.Address)
		isLocalProposer = (proposer.Address == e.consensus.LocalValidator.Address)
	}
	logger.Info("[Consensus] BlockHeight ", currentHeight, " NextBlock ", nextBlockHeight, " Proposer: ", proposerAddr, " Local: ", localAddr, " IsLocal: ", isLocalProposer)

	// Check if I am the proposer
	if isLocalProposer {
		e.proposeBlock()

		// BFT mode: Broadcast proposal and collect votes
		if e.proposedBlock != nil {
			e.broadcastProposal()
		}
	} else {
		// Non-proposer: Wait for proposal
		// Keep existing VoteSet if exists (might be collecting votes)
		if e.prevotes == nil || e.prevotes.Height != nextBlockHeight {
			e.prevotes = NewVoteSet(nextBlockHeight, 0, VoteTypePrevote)
			e.precommits = NewVoteSet(nextBlockHeight, 0, VoteTypePrecommit)
			e.startRoundTimer()
		}
		// Keep timeout timer if already running
	}
}

// produceBlockSolo creates block in solo node mode
func (e *ConsensusEngine) produceBlockSolo() {
	// Check current height
	currentHeight, err := e.blockchain.GetLatestHeight()
	if err != nil {
		currentHeight = 0
	}

	// Get previous block hash (includes genesis block hash)
	var prevHash prt.Hash
	prevBlock, err := e.blockchain.GetBlockByHeight(currentHeight)
	if err == nil {
		prevHash = prevBlock.Header.Hash
	}

	// Proposer address (local validator or empty address)
	var proposerAddr prt.Address
	if e.consensus.LocalValidator != nil {
		proposerAddr = e.consensus.LocalValidator.Address
	}

	// Block timestamp (used consistently across all nodes)
	blockTimestamp := time.Now().Unix()

	// Create new block
	newBlock := e.blockchain.SetBlock(prevHash, currentHeight+1, proposerAddr, blockTimestamp)
	if newBlock == nil {
		logger.Error("[Consensus] Failed to create block")
		return
	}

	// Sign block (if local proposer exists)
	if e.consensus.LocalProposer != nil {
		sig, err := e.consensus.LocalProposer.signBlockHash(newBlock.Header.Hash)
		if err != nil {
			logger.Error("[Consensus] Failed to sign block: ", err)
		} else {
			newBlock.SignBlock(sig)
		}
	}

	// Add block
	if success, err := e.blockchain.AddBlock(*newBlock); !success || err != nil {
		logger.Error("[Consensus] Failed to add block: ", err)
		return
	}

	logger.Info("[Consensus] Block ", newBlock.Header.Height, " created (hash: ", utils.HashToString(newBlock.Header.Hash)[:16], ", txs: ", len(newBlock.Transactions), ")")

	// Update last block time for interval control
	e.lastBlockTime = time.Now().UnixMilli()

	// Call callback
	if e.onBlockCommit != nil {
		e.onBlockCommit(newBlock)
	}

	// To next height
	e.consensus.UpdateHeight(newBlock.Header.Height + 1)
}

// proposeBlock proposes a block (PoA mode: commit immediately without vote)
func (e *ConsensusEngine) proposeBlock() {
	e.consensus.mu.Lock()
	e.consensus.State = StateProposing
	e.consensus.mu.Unlock()

	// Check current height
	currentHeight, err := e.blockchain.GetLatestHeight()
	if err != nil {
		currentHeight = 0
	}

	// Previous block hash (includes genesis block hash)
	var prevHash prt.Hash
	prevBlock, err := e.blockchain.GetBlockByHeight(currentHeight)
	if err == nil {
		prevHash = prevBlock.Header.Hash
	}

	// Proposer address
	var proposerAddr prt.Address
	if e.consensus.LocalValidator != nil {
		proposerAddr = e.consensus.LocalValidator.Address
	}

	// Block timestamp (used consistently across all nodes)
	blockTimestamp := time.Now().Unix()

	// Create new block
	newBlock := e.blockchain.SetBlock(prevHash, currentHeight+1, proposerAddr, blockTimestamp)
	if newBlock == nil {
		logger.Error("[Consensus] Failed to create proposed block")
		return
	}

	// Block signature
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

// broadcastProposal starts proposal broadcast and voting (BFT mode)
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

	// Initialize VoteSet
	e.prevotes = NewVoteSet(height, round, VoteTypePrevote)
	e.precommits = NewVoteSet(height, round, VoteTypePrecommit)

	// Broadcast Proposal via P2P
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

	// Start round timeout timer
	e.startRoundTimer()

	// Local prevote (proposer also participates in voting)
	e.castVote(VoteTypePrevote, blockHash)
}

// HandleProposal handles proposal message (from P2P)
func (e *ConsensusEngine) HandleProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if block at this height is already committed
	currentHeight, _ := e.blockchain.GetLatestHeight()
	if height <= currentHeight {
		logger.Debug("[Consensus] Proposal for already committed height ", height, ", ignoring")
		return
	}

	// Expected next block height
	expectedNextHeight := currentHeight + 1

	// Ignore if height mismatch
	if height != expectedNextHeight {
		logger.Debug("[Consensus] Proposal height mismatch: expected ", expectedNextHeight, ", got ", height)
		return
	}

	// First check if proposer for this round is valid (before round sync)
	expectedProposer := e.consensus.Selector.SelectProposer(height, round)
	if expectedProposer == nil {
		logger.Error("[Consensus] No expected proposer for height ", height, " round ", round)
		return
	}
	if block.Proposer != expectedProposer.Address {
		logger.Debug("[Consensus] Invalid proposer for round ", round, ": expected ", utils.AddressToString(expectedProposer.Address)[:16], ", got ", utils.AddressToString(block.Proposer)[:16])
		return
	}

	// Sync to height/round if valid proposal
	if e.consensus.CurrentHeight != height || e.consensus.CurrentRound != round {
		logger.Info("[Consensus] Syncing to height ", height, " round ", round, " (was ", e.consensus.CurrentHeight, "/", e.consensus.CurrentRound, ")")
		e.consensus.mu.Lock()
		e.consensus.CurrentHeight = height
		e.consensus.CurrentRound = round
		e.consensus.mu.Unlock()
	}

	// Validate block (including signature)
	if err := e.blockchain.ValidateBlock(*block, false); err != nil {
		logger.Error("[Consensus] Invalid proposed block: ", err)
		return
	}

	logger.Info("[Consensus] Received valid proposal at height ", height, " round ", round, " from ", utils.AddressToString(block.Proposer)[:16])

	e.proposedBlock = block
	e.prevotes = NewVoteSet(height, round, VoteTypePrevote)
	e.precommits = NewVoteSet(height, round, VoteTypePrecommit)

	// Start round timeout timer
	e.startRoundTimer()

	// Send Prevote (if local validator)
	if e.consensus.LocalValidator != nil {
		e.castVote(VoteTypePrevote, blockHash)
	}
}

// HandleVote handles vote message (from P2P)
func (e *ConsensusEngine) HandleVote(vote *Vote) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Ignore votes for already committed height
	currentHeight, _ := e.blockchain.GetLatestHeight()
	if vote.Height <= currentHeight {
		return
	}

	// Ignore if not next block height
	expectedNextHeight := currentHeight + 1
	if vote.Height != expectedNextHeight {
		return
	}

	// Ignore if different from current round (votes synced with proposal)
	// However, process if VoteSet exists and round matches
	if vote.Round != e.consensus.CurrentRound {
		logger.Debug("[Consensus] Vote round mismatch: current ", e.consensus.CurrentRound, ", got ", vote.Round)
		return
	}

	// Get voter's voting power
	validator := e.consensus.ValidatorSet.GetValidator(vote.VoterID)
	if validator == nil {
		logger.Debug("[Consensus] Unknown voter: ", utils.AddressToString(vote.VoterID)[:16])
		return
	}

	// Verify vote signature
	if !validator.ValidateBlockSignature(vote.BlockHash, vote.Signature) {
		logger.Warn("[Consensus] Invalid vote signature from: ", utils.AddressToString(vote.VoterID)[:16])
		return
	}

	totalPower := e.consensus.ValidatorSet.TotalVotingPower
	voterAddr := utils.AddressToString(vote.VoterID)[:16]

	switch vote.Type {
	case VoteTypePrevote:
		if e.prevotes != nil {
			added := e.prevotes.AddVote(vote, validator.VotingPower)
			if added {
				logger.Debug("[Consensus] Prevote received from ", voterAddr, " (", e.prevotes.VotedPower, "/", totalPower, " = ", len(e.prevotes.Votes), " votes)")
			}

			// Proceed to precommit if 2/3+
			if e.prevotes.HasTwoThirdsMajority(totalPower) {
				logger.Debug("[Consensus] Prevote 2/3+ reached at height ", vote.Height, " (", e.prevotes.VotedPower, "/", totalPower, ")")
				e.castVote(VoteTypePrecommit, vote.BlockHash)
			}
		}

	case VoteTypePrecommit:
		if e.precommits != nil {
			added := e.precommits.AddVote(vote, validator.VotingPower)
			if added {
				logger.Debug("[Consensus] Precommit received from ", voterAddr, " (", e.precommits.VotedPower, "/", totalPower, " = ", len(e.precommits.Votes), " votes)")
			}

			// Commit if 2/3+
			if e.precommits.HasTwoThirdsMajority(totalPower) && e.proposedBlock != nil {
				logger.Debug("[Consensus] Precommit 2/3+ reached at height ", vote.Height, " (", e.precommits.VotedPower, "/", totalPower, "), committing block")
				e.commitBlockWithSignatures(e.proposedBlock)
			}
		}
	}
}

// castVote creates and sends vote
func (e *ConsensusEngine) castVote(voteType VoteType, blockHash prt.Hash) {
	if e.consensus.LocalValidator == nil || e.consensus.LocalProposer == nil {
		return
	}

	// Create signature
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

	// Broadcast vote via P2P
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

	// Handle local vote
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

// commitBlock commits block (for solo node - without signature)
func (e *ConsensusEngine) commitBlock(block *core.Block) {
	// Cancel timeout timer
	e.stopRoundTimer()

	e.consensus.mu.Lock()
	e.consensus.State = StateCommitting
	e.consensus.mu.Unlock()

	// Add block
	success, err := e.blockchain.AddBlock(*block)
	if !success || err != nil {
		logger.Error("[Consensus] Failed to commit block: ", err)
		e.consensus.IncrementRound()
		return
	}

	logger.Info("[Consensus] Block ", block.Header.Height, " committed (hash: ", utils.HashToString(block.Header.Hash)[:16], ", txs: ", len(block.Transactions), ")")

	// Update last block time for interval control
	e.lastBlockTime = time.Now().UnixMilli()

	// Call callback (P2P broadcast)
	if e.onBlockCommit != nil {
		e.onBlockCommit(block)
	}

	// To next height
	e.consensus.UpdateHeight(block.Header.Height + 1)
	e.proposedBlock = nil
	e.prevotes = nil
	e.precommits = nil
}

// commitBlockWithSignatures commits block after BFT consensus (with validator signatures)
func (e *ConsensusEngine) commitBlockWithSignatures(block *core.Block) {
	// Cancel timeout timer
	e.stopRoundTimer()

	// Reset consecutive timeout counter
	e.consecutiveTimeouts = 0

	e.consensus.mu.Lock()
	e.consensus.State = StateCommitting
	e.consensus.mu.Unlock()

	// Create CommitSignatures from precommits
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

	// Final BFT validation before commit
	if err := e.blockchain.ValidateBlock(*block, true); err != nil {
		logger.Error("[Consensus] Final block validation failed: ", err)
		e.consensus.IncrementRound()
		return
	}

	// Add block
	success, err := e.blockchain.AddBlock(*block)
	if !success || err != nil {
		logger.Error("[Consensus] Failed to commit block: ", err)
		e.consensus.IncrementRound()
		return
	}

	logger.Info("[Consensus] Block ", block.Header.Height, " committed with BFT consensus (hash: ", utils.HashToString(block.Header.Hash)[:16], ", txs: ", len(block.Transactions), ", validators: ", len(block.CommitSignatures), ")")

	// Update last block time for interval control
	e.lastBlockTime = time.Now().UnixMilli()

	// Call callback (P2P broadcast)
	if e.onBlockCommit != nil {
		e.onBlockCommit(block)
	}

	// To next height
	e.consensus.UpdateHeight(block.Header.Height + 1)
	e.proposedBlock = nil
	e.prevotes = nil
	e.precommits = nil
}

// startRoundTimer starts round timeout timer
func (e *ConsensusEngine) startRoundTimer() {
	e.roundTimerMu.Lock()
	defer e.roundTimerMu.Unlock()

	// Cancel existing timer
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

// stopRoundTimer cancels round timeout timer
func (e *ConsensusEngine) stopRoundTimer() {
	e.roundTimerMu.Lock()
	defer e.roundTimerMu.Unlock()

	if e.roundTimer != nil {
		e.roundTimer.Stop()
		e.roundTimer = nil
	}
}

// handleRoundTimeout handles round timeout
func (e *ConsensusEngine) handleRoundTimeout(height uint64, round uint32) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Ignore if already advanced to next height or round changed
	if e.consensus.CurrentHeight != height || e.consensus.CurrentRound != round {
		e.consecutiveTimeouts = 0 // Reset on normal progress
		return
	}

	e.consecutiveTimeouts++
	logger.Warn("[Consensus] Round timeout at height ", height, " round ", round, " (consecutive: ", e.consecutiveTimeouts, ")")

	// Attempt block sync if 3+ consecutive timeouts
	if e.consecutiveTimeouts >= 3 {
		logger.Warn("[Consensus] Too many consecutive timeouts, attempting block sync...")
		e.consecutiveTimeouts = 0

		// Check current blockchain height
		currentHeight, _ := e.blockchain.GetLatestHeight()

		// Attempt block sync (in separate goroutine)
		if e.syncer != nil && e.syncer.GetPeerCount() > 0 {
			go func() {
				if err := e.syncer.SyncBlocks(); err != nil {
					logger.Debug("[Consensus] Block sync during timeout: ", err)
				} else {
					logger.Info("[Consensus] Block sync completed after timeout")
				}
			}()
		}

		// Update consensus height if blockchain height changed after sync
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

	// Increment round -> next proposer's turn
	e.consensus.IncrementRound()
	e.proposedBlock = nil
	e.prevotes = nil
	e.precommits = nil

	// Check if next round proposer is local node
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

	// Start next round timer
	e.startRoundTimer()
}

// broadcastProposalInternal internal proposal broadcast (without mutex)
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

// GetStatus returns current status
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

// IsRunning checks if running
func (e *ConsensusEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}
