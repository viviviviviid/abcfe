package rest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/abcfe/abcfe-node/api"
	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/consensus"
	"github.com/abcfe/abcfe-node/core"
	"github.com/abcfe/abcfe-node/p2p"
	"github.com/abcfe/abcfe-node/wallet"
)

// Server REST API server structure
type Server struct {
	port             int // 공개 API 포트 (0.0.0.0)
	internalPort     int // 내부 API 포트 (127.0.0.1), 0이면 비활성화
	httpServer       *http.Server
	internalServer   *http.Server // 내부 전용 서버
	blockchain       *core.BlockChain
	wallet           *wallet.WalletManager
	wsHub            *api.WSHub
	consensus        *consensus.Consensus
	consensusEngine  *consensus.ConsensusEngine
	p2p              *p2p.P2PService
}

// NewServer creates API server instance
func NewServer(port int, blockchain *core.BlockChain, walletMgr *wallet.WalletManager, cons *consensus.Consensus) *Server {
	wsHub := api.NewWSHub()
	return &Server{
		port:       port,
		blockchain: blockchain,
		wallet:     walletMgr,
		wsHub:      wsHub,
		consensus:  cons,
	}
}

// NewServerWithInternalPort creates API server with separate internal port
func NewServerWithInternalPort(port int, internalPort int, blockchain *core.BlockChain, walletMgr *wallet.WalletManager, cons *consensus.Consensus) *Server {
	wsHub := api.NewWSHub()
	return &Server{
		port:         port,
		internalPort: internalPort,
		blockchain:   blockchain,
		wallet:       walletMgr,
		wsHub:        wsHub,
		consensus:    cons,
	}
}

// Start starts API server
func (s *Server) Start() error {
	// Start WebSocket Hub
	go s.wsHub.Run()

	// 내부 포트가 설정된 경우: 공개/내부 서버 분리
	if s.internalPort > 0 {
		return s.startDualServers()
	}

	// 기존 방식: 단일 서버 (모든 API 포함)
	return s.startSingleServer()
}

// startSingleServer 단일 서버 모드 (기존 호환)
func (s *Server) startSingleServer() error {
	router := setupRouter(s.blockchain, s.wallet, s.wsHub, s.consensus, s.consensusEngine, s.p2p)

	addr := fmt.Sprintf(":%d", s.port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("REST API Server starting on port", s.port, "(single mode - all APIs)")
	logger.Info("WebSocket available at ws://localhost:", s.port, "/ws")
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("REST API Server error:", err)
		}
	}()

	return nil
}

// startDualServers 공개/내부 서버 분리 모드
func (s *Server) startDualServers() error {
	// 1. 공개 서버 (0.0.0.0 - 외부 접근 가능, 조회 API만)
	publicRouter := setupPublicRouter(s.blockchain, s.wsHub, s.consensus, s.consensusEngine, s.p2p)

	publicAddr := fmt.Sprintf("0.0.0.0:%d", s.port)
	s.httpServer = &http.Server{
		Addr:         publicAddr,
		Handler:      publicRouter,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("Public REST API Server starting on", publicAddr, "(read-only APIs)")
	logger.Info("WebSocket available at ws://0.0.0.0:", s.port, "/ws")
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Public REST API Server error:", err)
		}
	}()

	// 2. 내부 서버 (127.0.0.1 - localhost만 접근 가능, 모든 API)
	internalRouter := setupInternalRouter(s.blockchain, s.wallet, s.wsHub, s.consensus, s.consensusEngine, s.p2p)

	internalAddr := fmt.Sprintf("127.0.0.1:%d", s.internalPort)
	s.internalServer = &http.Server{
		Addr:         internalAddr,
		Handler:      internalRouter,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("Internal REST API Server starting on", internalAddr, "(full APIs - localhost only)")
	go func() {
		if err := s.internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Internal REST API Server error:", err)
		}
	}()

	return nil
}

// Stop stops API server
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("Shutting down REST API Server...")

	// 내부 서버도 종료
	if s.internalServer != nil {
		if err := s.internalServer.Shutdown(ctx); err != nil {
			logger.Error("Internal REST API Server shutdown error:", err)
		}
	}

	return s.httpServer.Shutdown(ctx)
}

// GetWSHub returns WebSocket Hub
func (s *Server) GetWSHub() *api.WSHub {
	return s.wsHub
}

// SetP2P sets P2P service
func (s *Server) SetP2P(p2pService *p2p.P2PService) {
	s.p2p = p2pService
}

// GetP2P returns P2P service
func (s *Server) GetP2P() *p2p.P2PService {
	return s.p2p
}

// SetConsensusEngine sets ConsensusEngine
func (s *Server) SetConsensusEngine(engine *consensus.ConsensusEngine) {
	s.consensusEngine = engine
}

// GetConsensusEngine returns ConsensusEngine
func (s *Server) GetConsensusEngine() *consensus.ConsensusEngine {
	return s.consensusEngine
}
