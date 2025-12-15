package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/abcfe/abcfe-node/api/rest"
	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/common/utils"
	conf "github.com/abcfe/abcfe-node/config"
	"github.com/abcfe/abcfe-node/consensus"
	"github.com/abcfe/abcfe-node/core"
	"github.com/abcfe/abcfe-node/p2p"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/abcfe/abcfe-node/storage"
	"github.com/abcfe/abcfe-node/wallet"
	"github.com/syndtr/goleveldb/leveldb"
)

type App struct {
	stop       chan struct{}
	Conf       conf.Config
	DB         *leveldb.DB // Mutex within db should not be copied
	BlockChain *core.BlockChain
	restServer *rest.Server // Added: REST API server field
	Wallet     *wallet.WalletManager

	// Consensus & P2P
	Consensus       *consensus.Consensus
	ConsensusEngine *consensus.ConsensusEngine
	P2PService      *p2p.P2PService
}

func New(configPath string) (*App, error) {
	cfg, err := conf.NewConfig(configPath)
	if err != nil {
		fmt.Println("Failed to initialized application: ", err)
		return nil, err
	}

	if err := logger.InitLogger(cfg); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	db, err := storage.InitDB(cfg)
	if err != nil {
		logger.Error("Failed to load db: ", err)
		return nil, err
	}

	wallet, err := wallet.InitWallet(cfg)
	if err != nil {
		logger.Error("Failed to load wallet: ", err)
		return nil, err
	}
	logger.Info("wallet imported: ", crypto.AddressTo0xPrefixString(wallet.Wallet.Accounts[0].Address))

	bc, err := core.NewChainState(db, cfg)
	if err != nil {
		logger.Error("failed to initailze chain state: ", err)
		return nil, err
	}

	// Initialize Consensus
	cons, err := consensus.NewConsensus(cfg, db)
	if err != nil {
		logger.Error("Failed to initialize consensus: ", err)
		return nil, err
	}

	// If BlockProducer, register as validator
	if cfg.Common.BlockProducer {
		account := wallet.Wallet.Accounts[0]
		if err := cons.RegisterValidator(account.Address, account.PublicKey, account.PrivateKey); err != nil {
			logger.Error("Failed to register validator: ", err)
			return nil, err
		}
		logger.Info("Registered as validator: ", crypto.AddressTo0xPrefixString(account.Address))
	}

	// Initialize Consensus Engine
	consEngine := consensus.NewConsensusEngine(cons, bc)

	// Set ProposerValidator in BlockChain (for PoA verification)
	bc.SetProposerValidator(cons)

	// Initialize P2P (Create first to connect to ConsensusEngine)
	p2pService, err := p2p.NewP2PService(
		cfg.P2P.Address,
		cfg.P2P.Port,
		cfg.Common.NetworkID,
		bc,
	)
	if err != nil {
		logger.Error("Failed to initialize P2P: ", err)
		return nil, err
	}

	app := &App{
		stop:            make(chan struct{}),
		Conf:            *cfg,
		DB:              db,
		BlockChain:      bc,
		Wallet:          wallet,
		Consensus:       cons,
		ConsensusEngine: consEngine,
		P2PService:      p2pService,
	}

	// Initialize REST API server
	app.restServer = rest.NewServer(app.Conf.Server.RestPort, app.BlockChain, app.Wallet, app.Consensus)

	// Set P2P service in REST server
	app.restServer.SetP2P(app.P2PService)

	// Set P2P broadcast and WebSocket notification callback on block commit
	app.ConsensusEngine.SetBlockCommitCallback(func(block *core.Block) {
		// P2P Broadcast
		if app.P2PService != nil && app.P2PService.IsRunning() {
			if err := app.P2PService.BroadcastBlock(block); err != nil {
				logger.Error("Failed to broadcast block: ", err)
			}
		}
		// WebSocket Broadcast
		if wsHub := app.restServer.GetWSHub(); wsHub != nil {
			wsHub.BroadcastNewBlock(block)
		}
	})

	// Set P2P Broadcaster in ConsensusEngine
	app.ConsensusEngine.SetP2PBroadcaster(app.P2PService)

	// Set BlockSyncer in ConsensusEngine (for sync on timeout)
	app.ConsensusEngine.SetBlockSyncer(app.P2PService)

	// Deliver Proposal to ConsensusEngine when received from P2P
	app.P2PService.SetProposalHandler(func(height uint64, round uint32, blockHash prt.Hash, block *core.Block) {
		app.ConsensusEngine.HandleProposal(height, round, blockHash, block)
	})

	// Deliver Vote to ConsensusEngine when received from P2P
	app.P2PService.SetVoteHandler(func(height uint64, round uint32, voteType uint8, blockHash prt.Hash, voterID prt.Address, signature prt.Signature) {
		vote := &consensus.Vote{
			Height:    height,
			Round:     round,
			Type:      consensus.VoteType(voteType),
			BlockHash: blockHash,
			VoterID:   voterID,
			Signature: signature,
		}
		app.ConsensusEngine.HandleVote(vote)
	})

	// Add to Mempool and propagate when transaction received from P2P
	app.P2PService.SetTxHandler(func(tx *core.Transaction) {
		// Transaction ID string
		txID := utils.HashToString(tx.ID)

		// 1. Validate
		if err := app.BlockChain.ValidateTransaction(tx); err != nil {
			logger.Debug("[TxHandler] Invalid tx received: ", txID[:16], " error: ", err)
			return
		}

		// 2. Add to Mempool
		// Returns error if already exists, preventing duplicate propagation
		if err := app.BlockChain.Mempool.NewTranaction(tx); err != nil {
			// Do not propagate if already exists or error occurs
			return
		}

		logger.Debug("[TxHandler] Added tx to mempool: ", txID[:16])

		// 3. Rebroadcast to other peers (Gossip)
		// If I received valid transaction for the first time, tell other peers
		if app.P2PService != nil {
			if err := app.P2PService.BroadcastTx(tx); err != nil {
				logger.Debug("[TxHandler] Failed to rebroadcast tx: ", err)
			}
		}
	})

	// Map for storing pending blocks (height -> block)
	pendingBlocks := make(map[uint64]*core.Block)
	var pendingMu sync.Mutex

	// Set block handling callback when received from P2P
	app.P2PService.SetBlockHandler(func(block *core.Block) {
		// Check if block already exists
		currentHeight, _ := app.BlockChain.GetLatestHeight()
		logger.Debug("[BlockHandler] Received block height=", block.Header.Height, " current=", currentHeight)

		// Special handling if empty chain (currentHeight == 0) and received genesis block (height == 0)
		if currentHeight == 0 && block.Header.Height == 0 {
			logger.Info("[BlockHandler] Received genesis block from peer, importing...")
			if err := app.BlockChain.ValidateBlock(*block); err != nil {
				logger.Error("[BlockHandler] Invalid genesis block: ", err)
				return
			}
			if success, err := app.BlockChain.AddBlock(*block); !success || err != nil {
				logger.Error("[BlockHandler] Failed to add genesis block: ", err)
				return
			}
			logger.Info("[BlockHandler] Genesis block imported successfully")
			return
		}

		if block.Header.Height <= currentHeight {
			logger.Debug("[BlockHandler] Block already exists, skipping")
			return
		}

		// Check if consecutive block
		if block.Header.Height > currentHeight+1 {
			logger.Debug("[BlockHandler] Missing blocks! Need height ", currentHeight+1, " got ", block.Header.Height, ". Storing pending block.")
			// Store for later processing
			pendingMu.Lock()
			pendingBlocks[block.Header.Height] = block
			pendingMu.Unlock()

			// Request missing blocks
			peers := app.P2PService.GetPeers()
			if len(peers) > 0 {
				if err := app.P2PService.RequestBlocks(peers[0], currentHeight+1, block.Header.Height-1); err != nil {
					logger.Error("[BlockHandler] Failed to request missing blocks: ", err)
				} else {
					logger.Debug("[BlockHandler] Requested blocks ", currentHeight+1, " to ", block.Header.Height-1)
				}
			}
			return
		}

		// Validate and add block
		logger.Debug("[BlockHandler] Validating block height=", block.Header.Height)
		if err := app.BlockChain.ValidateBlock(*block); err != nil {
			logger.Error("[BlockHandler] Invalid received block height=", block.Header.Height, ": ", err)
			return
		}
		logger.Debug("[BlockHandler] Block ", block.Header.Height, " validation passed")

		if success, err := app.BlockChain.AddBlock(*block); !success || err != nil {
			logger.Error("[BlockHandler] Failed to add received block height=", block.Header.Height, ": ", err)
			return
		}
		logger.Debug("[BlockHandler] Block ", block.Header.Height, " added successfully")

		logger.Info("[BlockHandler] Block synced from peer: height=", block.Header.Height)
		// BFT: Sync consensus height - Update to next height on block receipt
		if app.Consensus != nil {
			app.Consensus.UpdateHeight(block.Header.Height + 1)
		}

		// Rebroadcast to other peers (gossip)
		if app.P2PService != nil {
			if err := app.P2PService.BroadcastBlock(block); err != nil {
				logger.Debug("[BlockHandler] Failed to rebroadcast block: ", err)
			}
		}

		// Process pending next block
		for {
			pendingMu.Lock()
			nextHeight := block.Header.Height + 1
			nextBlock, exists := pendingBlocks[nextHeight]
			if exists {
				delete(pendingBlocks, nextHeight)
			}
			pendingMu.Unlock()

			if !exists {
				break
			}

			if err := app.BlockChain.ValidateBlock(*nextBlock); err != nil {
				logger.Error("[BlockHandler] Invalid pending block: ", err)
				break
			}

			if success, err := app.BlockChain.AddBlock(*nextBlock); !success || err != nil {
				logger.Error("[BlockHandler] Failed to add pending block: ", err)
				break
			}

			logger.Debug("[BlockHandler] Pending block added: height=", nextBlock.Header.Height)
			// BFT: Sync consensus height
			if app.Consensus != nil {
				app.Consensus.UpdateHeight(nextBlock.Header.Height + 1)
			}

			block = nextBlock
		}
	})

	return app, nil
}

func (p *App) NewRest() error {
	// Start REST API server
	if err := p.restServer.Start(); err != nil {
		return fmt.Errorf("failed to start REST API server: %w", err)
	}

	logger.Info("All services started")
	return nil
}

// StartConsensus 컨센서스 엔진 시작
func (p *App) StartConsensus() error {
	if p.ConsensusEngine == nil {
		return fmt.Errorf("consensus engine not initialized")
	}

	if err := p.ConsensusEngine.Start(); err != nil {
		return fmt.Errorf("failed to start consensus engine: %w", err)
	}

	logger.Info("Consensus engine started")
	return nil
}

// StartP2P P2P 서비스 시작
func (p *App) StartP2P() error {
	if p.P2PService == nil {
		return fmt.Errorf("p2p service not initialized")
	}

	if err := p.P2PService.Start(); err != nil {
		return fmt.Errorf("failed to start P2P service: %w", err)
	}

	logger.Info("P2P service started on port ", p.Conf.P2P.Port)
	return nil
}

// ConnectPeer 피어에 연결
func (p *App) ConnectPeer(address string) error {
	if p.P2PService == nil {
		return fmt.Errorf("p2p service not initialized")
	}

	return p.P2PService.Connect(address)
}

// StartAll 모든 서비스 시작 (REST, P2P, Consensus)
func (p *App) StartAll() error {
	// REST API 시작
	if err := p.NewRest(); err != nil {
		return err
	}

	// P2P 시작
	if err := p.StartP2P(); err != nil {
		return err
	}

	// Boot 노드들에 연결
	connectedPeers := 0
	for _, bootNode := range p.Conf.P2P.BootNodes {
		if err := p.ConnectPeer(bootNode); err != nil {
			logger.Error("Failed to connect to boot node: ", bootNode, " error: ", err)
		} else {
			connectedPeers++
		}
	}

	// 피어 연결 후 초기 동기화 시작
	if connectedPeers > 0 {
		// 피어가 완전히 연결될 때까지 잠시 대기
		time.Sleep(1 * time.Second)

		// 초기 블록 동기화 시도
		go func() {
			logger.Info("[Sync] Starting initial block synchronization...")
			if err := p.P2PService.SyncBlocks(); err != nil {
				logger.Debug("[Sync] Initial sync completed or no sync needed: ", err)
			} else {
				logger.Info("[Sync] Initial synchronization completed")
			}
		}()

		// 주기적 동기화 시작 (모든 노드에서 - BFT 참여를 위해 동기화 필요)
		go p.startPeriodicSync()
	}

	// BFT 모드: 모든 검증자 노드에서 컨센서스 시작 (투표 참여를 위해)
	// BlockProducer 여부와 관계없이 컨센서스 엔진은 모든 노드에서 실행되어야 함
	if err := p.StartConsensus(); err != nil {
		return err
	}
	if !p.Conf.Common.BlockProducer {
		logger.Info("Running as validator node (participating in BFT consensus)")
	}

	logger.Info("All services started successfully")
	return nil
}

// startPeriodicSync 주기적 블록 동기화 (모든 노드에서 실행)
func (p *App) startPeriodicSync() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			if p.P2PService.GetPeerCount() > 0 {
				currentHeight, _ := p.BlockChain.GetLatestHeight()

				if err := p.P2PService.SyncBlocks(); err != nil {
					logger.Debug("[Sync] Sync check at height ", currentHeight, ": ", err)
				}
			}
		}
	}
}

// Cleanup 애플리케이션 정리
func (p *App) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Consensus 엔진 종료
	if p.ConsensusEngine != nil {
		p.ConsensusEngine.Stop()
		logger.Info("Consensus engine stopped")
	}

	// P2P 서비스 종료
	if p.P2PService != nil {
		if err := p.P2PService.Stop(); err != nil {
			logger.Error("Error stopping P2P service:", err)
		} else {
			logger.Info("P2P service stopped")
		}
	}

	// REST API 서버 종료
	if p.restServer != nil {
		if err := p.restServer.Stop(ctx); err != nil {
			logger.Error("Error stopping REST API server:", err)
		}
	}

	// DB 연결 닫기
	if p.DB != nil {
		if err := p.DB.Close(); err != nil {
			logger.Error("Error closing DB connection:", err)
		}
	}

	logger.Info("All resources cleaned up")
}

func (p *App) Wait() {
	<-p.stop // 채널에서 값 읽으려고 시도
}

func (p *App) Terminate() {
	p.Cleanup() // 자원 정리 후 종료
	close(p.stop)
}

func (p *App) SigHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM) // OS 시그널을 채널로 전달
	go func() {
		sig := <-sigCh
		logger.Info("Arrived terminate signal: ", sig)
		p.Terminate()
	}()
}
