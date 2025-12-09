package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/abcfe/abcfe-node/api/rest"
	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/logger"
	conf "github.com/abcfe/abcfe-node/config"
	"github.com/abcfe/abcfe-node/consensus"
	"github.com/abcfe/abcfe-node/core"
	"github.com/abcfe/abcfe-node/p2p"
	"github.com/abcfe/abcfe-node/storage"
	"github.com/abcfe/abcfe-node/wallet"
	"github.com/syndtr/goleveldb/leveldb"
)

type App struct {
	stop       chan struct{}
	Conf       conf.Config
	DB         *leveldb.DB // db내의 mutex는 복사되면 안됨
	BlockChain *core.BlockChain
	restServer *rest.Server // 추가: REST API 서버 필드
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
		log.Fatalf("Failed to initialize logger: %v", err)
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

	// Consensus 초기화
	cons, err := consensus.NewConsensus(cfg, db)
	if err != nil {
		logger.Error("Failed to initialize consensus: ", err)
		return nil, err
	}

	// Consensus Engine 초기화
	consEngine := consensus.NewConsensusEngine(cons, bc)

	// P2P 초기화
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

	// REST API 서버 초기화
	app.restServer = rest.NewServer(app.Conf.Server.RestPort, app.BlockChain)

	// 블록 커밋 시 P2P 브로드캐스트 콜백 설정
	app.ConsensusEngine.SetBlockCommitCallback(func(block *core.Block) {
		if app.P2PService != nil && app.P2PService.IsRunning() {
			if err := app.P2PService.BroadcastBlock(block); err != nil {
				logger.Error("Failed to broadcast block: ", err)
			}
		}
	})

	// 누락된 블록 저장용 맵 (높이 -> 블록)
	pendingBlocks := make(map[uint64]*core.Block)
	var pendingMu sync.Mutex

	// P2P에서 블록 수신 시 처리 콜백 설정
	app.P2PService.SetBlockHandler(func(block *core.Block) {
		// 이미 있는 블록인지 확인
		currentHeight, _ := app.BlockChain.GetLatestHeight()
		log.Printf("[BlockHandler] Received block height=%d, current=%d", block.Header.Height, currentHeight)

		if block.Header.Height <= currentHeight {
			log.Println("[BlockHandler] Block already exists, skipping")
			return
		}

		// 연속된 블록인지 확인
		if block.Header.Height > currentHeight+1 {
			log.Printf("[BlockHandler] Missing blocks! Need height %d, got %d. Storing pending block.", currentHeight+1, block.Header.Height)
			// 나중에 처리할 수 있도록 저장
			pendingMu.Lock()
			pendingBlocks[block.Header.Height] = block
			pendingMu.Unlock()

			// 누락된 블록 요청
			peers := app.P2PService.GetPeers()
			if len(peers) > 0 {
				if err := app.P2PService.RequestBlocks(peers[0], currentHeight+1, block.Header.Height-1); err != nil {
					log.Println("[BlockHandler] Failed to request missing blocks:", err)
				} else {
					log.Printf("[BlockHandler] Requested blocks %d to %d", currentHeight+1, block.Header.Height-1)
				}
			}
			return
		}

		// 블록 검증 및 추가
		log.Printf("[BlockHandler] Validating block height=%d", block.Header.Height)
		if err := app.BlockChain.ValidateBlock(*block); err != nil {
			log.Printf("[BlockHandler] Invalid received block height=%d: %v", block.Header.Height, err)
			logger.Error("Invalid received block: ", err)
			return
		}
		log.Printf("[BlockHandler] Block %d validation passed", block.Header.Height)

		if success, err := app.BlockChain.AddBlock(*block); !success || err != nil {
			log.Printf("[BlockHandler] Failed to add received block height=%d: %v", block.Header.Height, err)
			logger.Error("Failed to add received block: ", err)
			return
		}
		log.Printf("[BlockHandler] Block %d added successfully", block.Header.Height)

		log.Printf("[BlockHandler] Block synced from peer: height=%d", block.Header.Height)
		logger.Info("Block synced from peer: height=", block.Header.Height)

		// 대기 중인 다음 블록 처리
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
				log.Println("[BlockHandler] Invalid pending block:", err)
				break
			}

			if success, err := app.BlockChain.AddBlock(*nextBlock); !success || err != nil {
				log.Println("[BlockHandler] Failed to add pending block:", err)
				break
			}

			log.Printf("[BlockHandler] Pending block added: height=%d", nextBlock.Header.Height)
			block = nextBlock
		}
	})

	return app, nil
}

func (p *App) NewRest() error {
	// REST API 서버 시작
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
	for _, bootNode := range p.Conf.P2P.BootNodes {
		if err := p.ConnectPeer(bootNode); err != nil {
			logger.Error("Failed to connect to boot node: ", bootNode, " error: ", err)
		}
	}

	// BlockProducer인 경우에만 Consensus 시작
	if p.Conf.Common.BlockProducer {
		if err := p.StartConsensus(); err != nil {
			return err
		}
	} else {
		logger.Info("Running as sync-only node (not a block producer)")
	}

	logger.Info("All services started successfully")
	return nil
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
