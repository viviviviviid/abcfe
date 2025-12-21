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

func setupRouter(blockchain *core.BlockChain, walletMgr *wallet.WalletManager, wsHub *api.WSHub, cons *consensus.Consensus, consEngine *consensus.ConsensusEngine, p2pService *p2p.P2PService) http.Handler {
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

	// Blockchain status and statistics
	apiRouter.HandleFunc("/status", GetStatus(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/stats", GetNetworkStats(blockchain, wsHub)).Methods("GET")

	// Consensus status API
	apiRouter.HandleFunc("/consensus/status", GetConsensusStatus(cons, consEngine)).Methods("GET")

	// Block related API
	apiRouter.HandleFunc("/blocks", GetBlocks(blockchain)).Methods("GET")          // Block list (Pagination)
	apiRouter.HandleFunc("/block", ComposeAndAddBlock(blockchain)).Methods("POST") // Test-only block composition and addition (no validation)
	apiRouter.HandleFunc("/block/latest", GetLatestBlock(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/height/{height}", GetBlockByHeight(blockchain)).Methods("GET") // Match documentation
	apiRouter.HandleFunc("/block/{height}", GetBlockByHeight(blockchain)).Methods("GET")        // Backward compatibility
	apiRouter.HandleFunc("/block/hash/{hash}", GetBlockByHash(blockchain)).Methods("GET")

	// Transaction related API
	// Note: /tx (send without signature) removed due to security vulnerability
	// Use /tx/signed (client signature) or /tx/send (server wallet signature) instead
	apiRouter.HandleFunc("/tx/signed", SubmitSignedTx(blockchain, p2pService)).Methods("POST")            // Submit raw TX signed by client
	apiRouter.HandleFunc("/tx/send", SendTxWithWallet(blockchain, walletMgr, p2pService)).Methods("POST") // Sign with server wallet and send
	apiRouter.HandleFunc("/tx/{txid}", GetTx(blockchain)).Methods("GET")

	// Mempool related API
	apiRouter.HandleFunc("/mempool/list", GetMempoolList(blockchain)).Methods("GET")

	// UTXO related API
	apiRouter.HandleFunc("/address/{address}/utxo", GetAddressUtxo(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/address/{address}/balance", GetBalanceByUtxo(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/address/{address}/txs", GetAddressTransactions(blockchain)).Methods("GET")

	// Wallet related API
	apiRouter.HandleFunc("/wallet/accounts", GetWalletAccounts(walletMgr)).Methods("GET")
	apiRouter.HandleFunc("/wallet/account/new", CreateNewAccount(walletMgr)).Methods("POST")

	// WebSocket status API
	apiRouter.HandleFunc("/ws/status", GetWSStatus(wsHub)).Methods("GET")

	// P2P status API
	apiRouter.HandleFunc("/p2p/peers", GetP2PPeers(p2pService)).Methods("GET")
	apiRouter.HandleFunc("/p2p/status", GetP2PStatus(p2pService)).Methods("GET")

	return r
}
