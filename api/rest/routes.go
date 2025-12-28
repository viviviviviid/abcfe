package rest

import (
	"net/http"

	"github.com/abcfe/abcfe-node/api"
	"github.com/abcfe/abcfe-node/consensus"
	"github.com/abcfe/abcfe-node/core"
	"github.com/abcfe/abcfe-node/p2p"
	"github.com/abcfe/abcfe-node/wallet"
	"github.com/gorilla/mux"
)

// setupPublicRouter 공개 API 라우터 (외부 접근 가능)
// 조회 전용 API만 포함
func setupPublicRouter(blockchain *core.BlockChain, wsHub *api.WSHub, cons *consensus.Consensus, consEngine *consensus.ConsensusEngine, p2pService *p2p.P2PService) http.Handler {
	r := mux.NewRouter()

	// Middleware setup
	r.Use(LoggingMiddleware)
	r.Use(RecoveryMiddleware)

	// Base route
	r.HandleFunc("/", HomeHandler).Methods("GET")

	// WebSocket endpoint
	r.HandleFunc("/ws", api.HandleWebSocket(wsHub))

	// Blockchain API route
	apiRouter := r.PathPrefix("/api/v1").Subrouter()

	// Blockchain status and statistics (조회)
	apiRouter.HandleFunc("/status", GetStatus(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/stats", GetNetworkStats(blockchain, wsHub)).Methods("GET")

	// Consensus status API (조회)
	apiRouter.HandleFunc("/consensus/status", GetConsensusStatus(cons, consEngine)).Methods("GET")

	// Block related API (조회)
	apiRouter.HandleFunc("/blocks", GetBlocks(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/latest", GetLatestBlock(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/height/{height}", GetBlockByHeight(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/{height}", GetBlockByHeight(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/hash/{hash}", GetBlockByHash(blockchain)).Methods("GET")

	// Transaction API (조회 + 클라이언트 서명 TX 제출)
	apiRouter.HandleFunc("/tx/signed", SubmitSignedTx(blockchain, p2pService)).Methods("POST") // 클라이언트가 서명한 TX는 공개
	apiRouter.HandleFunc("/tx/{txid}", GetTx(blockchain)).Methods("GET")

	// Mempool related API (조회)
	apiRouter.HandleFunc("/mempool/list", GetMempoolList(blockchain)).Methods("GET")

	// UTXO related API (조회)
	apiRouter.HandleFunc("/address/{address}/utxo", GetAddressUtxo(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/address/{address}/balance", GetBalanceByUtxo(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/address/{address}/txs", GetAddressTransactions(blockchain)).Methods("GET")

	// WebSocket status API (조회)
	apiRouter.HandleFunc("/ws/status", GetWSStatus(wsHub)).Methods("GET")

	// P2P status API (조회)
	apiRouter.HandleFunc("/p2p/peers", GetP2PPeers(p2pService)).Methods("GET")
	apiRouter.HandleFunc("/p2p/status", GetP2PStatus(p2pService)).Methods("GET")

	return r
}

// setupInternalRouter 내부 API 라우터 (127.0.0.1만 접근 가능)
// 지갑 사용, 서버 서명 등 민감한 API 포함
func setupInternalRouter(blockchain *core.BlockChain, walletMgr *wallet.WalletManager, wsHub *api.WSHub, cons *consensus.Consensus, consEngine *consensus.ConsensusEngine, p2pService *p2p.P2PService) http.Handler {
	r := mux.NewRouter()

	// Middleware setup
	r.Use(LoggingMiddleware)
	r.Use(RecoveryMiddleware)

	// Base route
	r.HandleFunc("/", HomeHandler).Methods("GET")

	// Blockchain API route
	apiRouter := r.PathPrefix("/api/v1").Subrouter()

	// === 공개 API도 내부에서 사용 가능 (편의성) ===

	// Blockchain status and statistics
	apiRouter.HandleFunc("/status", GetStatus(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/stats", GetNetworkStats(blockchain, wsHub)).Methods("GET")

	// Consensus status API
	apiRouter.HandleFunc("/consensus/status", GetConsensusStatus(cons, consEngine)).Methods("GET")

	// Block related API
	apiRouter.HandleFunc("/blocks", GetBlocks(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block", ComposeAndAddBlock(blockchain)).Methods("POST") // 테스트용 블록 생성 (내부 전용)
	apiRouter.HandleFunc("/block/latest", GetLatestBlock(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/height/{height}", GetBlockByHeight(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/{height}", GetBlockByHeight(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/hash/{hash}", GetBlockByHash(blockchain)).Methods("GET")

	// Transaction API
	apiRouter.HandleFunc("/tx/signed", SubmitSignedTx(blockchain, p2pService)).Methods("POST")
	apiRouter.HandleFunc("/tx/{txid}", GetTx(blockchain)).Methods("GET")

	// Mempool related API
	apiRouter.HandleFunc("/mempool/list", GetMempoolList(blockchain)).Methods("GET")

	// UTXO related API
	apiRouter.HandleFunc("/address/{address}/utxo", GetAddressUtxo(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/address/{address}/balance", GetBalanceByUtxo(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/address/{address}/txs", GetAddressTransactions(blockchain)).Methods("GET")

	// WebSocket status API
	apiRouter.HandleFunc("/ws/status", GetWSStatus(wsHub)).Methods("GET")

	// P2P status API
	apiRouter.HandleFunc("/p2p/peers", GetP2PPeers(p2pService)).Methods("GET")
	apiRouter.HandleFunc("/p2p/status", GetP2PStatus(p2pService)).Methods("GET")

	// === 내부 전용 API (민감한 작업) ===

	// 서버 지갑으로 TX 서명 및 전송 (내부 전용)
	apiRouter.HandleFunc("/tx/send", SendTxWithWallet(blockchain, walletMgr, p2pService)).Methods("POST")

	// Wallet 관리 API (내부 전용)
	apiRouter.HandleFunc("/wallet/accounts", GetWalletAccounts(walletMgr)).Methods("GET")
	apiRouter.HandleFunc("/wallet/account/new", CreateNewAccount(walletMgr)).Methods("POST")

	return r
}

// setupRouter 기존 호환용 (deprecated, 내부 라우터와 동일)
func setupRouter(blockchain *core.BlockChain, walletMgr *wallet.WalletManager, wsHub *api.WSHub, cons *consensus.Consensus, consEngine *consensus.ConsensusEngine, p2pService *p2p.P2PService) http.Handler {
	return setupInternalRouter(blockchain, walletMgr, wsHub, cons, consEngine, p2pService)
}
