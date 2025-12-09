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
	"github.com/abcfe/abcfe-node/wallet"
)

// Server REST API 서버 구조체
type Server struct {
	port       int
	httpServer *http.Server
	blockchain *core.BlockChain
	wallet     *wallet.WalletManager
	wsHub      *api.WSHub
	consensus  *consensus.Consensus
}

// NewServer API 서버 인스턴스 생성
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

// Start API 서버 시작
func (s *Server) Start() error {
	// WebSocket Hub 시작
	go s.wsHub.Run()

	router := setupRouter(s.blockchain, s.wallet, s.wsHub, s.consensus)

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

// Stop API 서버 종료
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("Shutting down REST API Server...")
	return s.httpServer.Shutdown(ctx)
}

// GetWSHub WebSocket Hub 반환
func (s *Server) GetWSHub() *api.WSHub {
	return s.wsHub
}
