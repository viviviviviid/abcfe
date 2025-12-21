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
	port            int
	httpServer      *http.Server
	blockchain      *core.BlockChain
	wallet          *wallet.WalletManager
	wsHub           *api.WSHub
	consensus       *consensus.Consensus
	consensusEngine *consensus.ConsensusEngine
	p2p             *p2p.P2PService
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

// Start starts API server
func (s *Server) Start() error {
	// Start WebSocket Hub
	go s.wsHub.Run()

	router := setupRouter(s.blockchain, s.wallet, s.wsHub, s.consensus, s.consensusEngine, s.p2p)

	addr := fmt.Sprintf(":%d", s.port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("REST API Server starting on port", s.port)
	logger.Info("WebSocket available at ws://localhost:", s.port, "/ws")
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("REST API Server error:", err)
		}
	}()

	return nil
}

// Stop stops API server
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("Shutting down REST API Server...")
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
