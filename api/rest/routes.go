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

func setupRouter(blockchain *core.BlockChain, walletMgr *wallet.WalletManager, wsHub *api.WSHub, cons *consensus.Consensus, p2pService *p2p.P2PService) http.Handler {
	r := mux.NewRouter()

	// 미들웨어 설정
	r.Use(LoggingMiddleware)
	r.Use(RecoveryMiddleware)

	// 기본 경로
	r.HandleFunc("/", HomeHandler).Methods("GET")

	// WebSocket 엔드포인트
	r.HandleFunc("/ws", api.HandleWebSocket(wsHub))

	// 블록체인 API 경로
	apiRouter := r.PathPrefix("/api/v1").Subrouter()

	// 블록체인 상태 및 통계
	apiRouter.HandleFunc("/status", GetStatus(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/stats", GetNetworkStats(blockchain, wsHub)).Methods("GET")

	// 컨센서스 상태 API
	apiRouter.HandleFunc("/consensus/status", GetConsensusStatus(cons)).Methods("GET")

	// 블록 관련 API
	apiRouter.HandleFunc("/blocks", GetBlocks(blockchain)).Methods("GET")          // 블록 목록 (페이지네이션)
	apiRouter.HandleFunc("/block", ComposeAndAddBlock(blockchain)).Methods("POST") // 테스트 전용 블록 구성 및 블록 추가 (검증은 없음)
	apiRouter.HandleFunc("/block/latest", GetLatestBlock(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/block/height/{height}", GetBlockByHeight(blockchain)).Methods("GET") // 문서와 일치
	apiRouter.HandleFunc("/block/{height}", GetBlockByHeight(blockchain)).Methods("GET")        // 하위 호환성
	apiRouter.HandleFunc("/block/hash/{hash}", GetBlockByHash(blockchain)).Methods("GET")

	// 트랜잭션 관련 API
	// 주의: /tx (서명 없이 전송)는 보안 취약점으로 제거됨
	// 대신 /tx/signed (클라이언트 서명) 또는 /tx/send (서버 지갑 서명)를 사용
	apiRouter.HandleFunc("/tx/signed", SubmitSignedTx(blockchain, p2pService)).Methods("POST")            // 클라이언트가 서명한 raw TX 제출
	apiRouter.HandleFunc("/tx/send", SendTxWithWallet(blockchain, walletMgr, p2pService)).Methods("POST") // 서버 지갑으로 서명하여 전송
	apiRouter.HandleFunc("/tx/{txid}", GetTx(blockchain)).Methods("GET")

	// 멤풀 관련 API
	apiRouter.HandleFunc("/mempool/list", GetMempoolList(blockchain)).Methods("GET")

	// UTXO 관련 API
	apiRouter.HandleFunc("/address/{address}/utxo", GetAddressUtxo(blockchain)).Methods("GET")
	apiRouter.HandleFunc("/address/{address}/balance", GetBalanceByUtxo(blockchain)).Methods("GET")

	// 지갑 관련 API
	apiRouter.HandleFunc("/wallet/accounts", GetWalletAccounts(walletMgr)).Methods("GET")
	apiRouter.HandleFunc("/wallet/account/new", CreateNewAccount(walletMgr)).Methods("POST")

	// WebSocket 상태 API
	apiRouter.HandleFunc("/ws/status", GetWSStatus(wsHub)).Methods("GET")

	// P2P 상태 API
	apiRouter.HandleFunc("/p2p/peers", GetP2PPeers(p2pService)).Methods("GET")
	apiRouter.HandleFunc("/p2p/status", GetP2PStatus(p2pService)).Methods("GET")

	return r
}
