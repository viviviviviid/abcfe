# ABCFe Node - AI Assistant & Developer Guide

> **📅 마지막 업데이트: 2025-12-28**

이 문서는 ABCFe 블록체인 노드 프로젝트에 대한 AI 어시스턴트 및 개발자용 가이드입니다.

## 🚀 빠른 시작

### 빌드 & 실행
```bash
# 빌드
make build

# 단일 노드 실행
./abcfed

# 멀티 노드 실행 (자동 셋업)
./setup_multi_nodes.sh 3
```

### 주요 명령어
```bash
# 지갑 생성
./abcfed wallet create --wallet-dir=./resource/wallet

# 노드 상태 확인
curl http://localhost:8000/api/v1/status

# 블록 조회
curl http://localhost:8000/api/v1/blocks
```

## 📁 프로젝트 구조

```
abcfe-node/
├── cmd/node/          # 메인 애플리케이션 (main.go)
├── app/               # 앱 초기화 및 통합 (App 구조체)
├── core/              # 블록체인 코어
│   ├── blockchain.go  # 체인 상태 관리 (BlockChain)
│   ├── block.go       # 블록 구조 및 생성
│   ├── transaction.go # 트랜잭션 처리
│   ├── utxo.go        # UTXO 모델
│   ├── mempool.go     # 트랜잭션 풀
│   └── validate.go    # 검증 로직
├── consensus/         # PoA/BFT 컨센서스
│   ├── consensus.go   # 컨센서스 상태 (5단계) + ProposerValidator 구현
│   ├── engine.go      # 컨센서스 엔진 (블록 생성 + BFT 투표)
│   ├── proposer.go    # 제안자 서명 생성
│   ├── selection.go   # 제안자 선출 (RoundRobin/VRF/Hybrid)
│   ├── validator.go   # 검증자 관리 + 서명 검증
│   ├── staking.go     # 스테이킹 관리
│   └── type.go        # 타입 정의
├── p2p/               # P2P 네트워크
│   ├── p2p.go         # P2P 서비스
│   ├── node.go        # 노드 및 피어 관리
│   ├── message.go     # 메시지 프로토콜
│   └── ratelimit.go   # P2P 레이트 제한
├── api/               # REST API & WebSocket
│   ├── rest/          # REST API
│   │   ├── server.go      # HTTP 서버
│   │   ├── handler.go     # API 핸들러 (940+ lines)
│   │   ├── routes.go      # 라우팅
│   │   ├── types.go       # 응답 타입 정의
│   │   └── middleware.go  # CORS, 로깅
│   └── websocket.go   # WebSocket 핸들러 (즉시 응답 지원)
├── wallet/            # HD 지갑 (BIP-39, BIP-44)
│   ├── wallet.go      # 지갑 관리
│   ├── keystore.go    # 키스토어
│   └── crypto.go      # 암호화
├── storage/           # LevelDB 저장소 래퍼
├── common/            # 공통 유틸리티
│   ├── logger/        # 로깅 (zap)
│   ├── crypto/        # 암호화 유틸
│   └── utils/         # 범용 유틸
├── config/            # 설정 파일 (TOML)
└── protocol/          # 프로토콜 상수 및 타입
```

## 🏗️ 아키텍처

### 핵심 컴포넌트

#### 1. App (app/app.go)
- 모든 컴포넌트의 조정자
- DB, BlockChain, Wallet, Consensus, P2P, REST API 통합
- `New()` 함수에서 초기화
- `StartAll()` 함수에서 서비스 시작

#### 2. BlockChain (core/blockchain.go)
```go
type BlockChain struct {
    LatestHeight    uint64
    LatestBlockHash string
    db              *leveldb.DB
    Mempool         *Mempool
    mu              sync.RWMutex  // 동시성 제어
}
```
- LevelDB 기반 영구 저장
- UTXO 모델
- RWMutex로 동시 읽기/쓰기 보호

#### 3. Consensus (consensus/)
- **PoA/BFT (Proof of Authority with BFT voting)** 기반
- **1초마다** 새 블록 생성 (BlockIntervalMs = 1000)
- 제안자 선출: **RoundRobin / VRF / Hybrid** (설정 가능)
- 검증자: config 파일에서 고정 로드
- 블록에 **제안자 주소 + 서명** 포함
- **5단계 컨센서스 상태 머신**

```go
// 컨센서스 상태 (5단계)
const (
    StateIdle         = "IDLE"         // 대기
    StateProposing    = "PROPOSING"    // 블록 제안
    StatePrevoting    = "PREVOTING"    // 1차 투표 (Prevote)
    StatePrecommitting= "PRECOMMITTING"// 2차 투표 (Precommit)
    StateCommitting   = "COMMITTING"   // 블록 확정
)

// 블록 구조 (PoA 정보 포함)
type Block struct {
    Header       BlockHeader
    Transactions []*Transaction
    Proposer     Address    // 블록 제안자 주소
    Signature    Signature  // 제안자의 블록 해시 서명
}

// 타이밍 상수
BlockProduceTimeMs = 1000   // 블록 생성 체크 간격
BlockIntervalMs    = 1000   // 블록 간 최소 간격
RoundTimeoutMs     = 20000  // 라운드 타임아웃
```

**제안자 선택 알고리즘:**
```go
// 1. Round-Robin (기본)
SelectProposer(height, round) // (height + round) % len(validators)

// 2. VRF 기반 (예측 불가능)
SelectProposerVRF(height, round, prevBlockHash) // hash(prevBlockHash + height + round)

// 3. Hybrid (VRF + Round-Robin)
SelectProposerHybrid(height, round, prevBlockHash)
// - Round 0: VRF 기반 선택
// - Round 1+: Round-Robin fallback (liveness 보장)
```

**BFT 컨센서스 흐름:**
```
1. Proposing: 제안자가 블록 생성 및 브로드캐스트
2. Prevoting: 검증자들이 1차 투표 (2/3 이상 필요)
3. Precommitting: 검증자들이 2차 투표 (2/3 이상 필요)
4. Committing: 블록 확정 및 체인에 추가
5. Idle: 다음 라운드 대기
```

#### 4. P2P Network (p2p/)
- TCP 기반 메시징
- 핸드셰이크 프로토콜 (높이 정보 교환)
- 블록/트랜잭션 브로드캐스트
- **자동 블록 동기화** (높이 기반)
- 10MB 버퍼 (대용량 메시지 지원)
- **레이트 제한** (DoS 방지)

**메시지 타입:**
```go
// 기본 메시지
MsgTypeHandshake     // 핸드셰이크
MsgTypeHandshakeAck  // ACK
MsgTypeNewBlock      // 새 블록 알림
MsgTypeGetBlocks     // 블록 범위 요청
MsgTypeBlocks        // 블록 응답 (최대 100개)
MsgTypeNewTx         // 새 트랜잭션

// BFT 컨센서스 메시지
MsgTypeProposal      // 블록 제안
MsgTypeVote          // 투표 (Prevote/Precommit)
```

**레이트 제한 설정 (ratelimit.go):**
```go
MaxMessagesPerSecond: 100
BurstSize: 200
MaxBlocksPerSecond: 5
MaxTxPerSecond: 50
MaxProposalsPerSecond: 10
MaxVotesPerSecond: 50
BanDuration: 60 seconds
```

#### 5. REST API (api/rest/)

**포트 분리 (보안):**
```toml
[server]
RestPort = 8000           # 공개 API (0.0.0.0 - 외부 접근 가능)
InternalRestPort = 8800   # 내부 API (127.0.0.1 - localhost만 접근)
```

**공개 API (포트 8000) - 조회 전용:**
```
GET  /api/v1/status                      # 노드 상태
GET  /api/v1/blocks                      # 블록 목록 (페이징)
GET  /api/v1/block/height/{height}       # 높이로 블록 조회
GET  /api/v1/block/hash/{hash}           # 해시로 블록 조회
GET  /api/v1/address/{address}/balance   # 주소 잔액
GET  /api/v1/address/{address}/utxo      # 주소 UTXO
POST /api/v1/tx/signed                   # 클라이언트 서명 TX 제출
GET  /api/v1/mempool/list                # 멤풀 상태
GET  /api/v1/consensus/status            # 컨센서스 상태
```

**내부 API (포트 8800) - localhost 전용:**
```
# 공개 API 모두 포함 +
POST /api/v1/tx/send                     # 서버 지갑으로 TX 전송 (내부 전용)
GET  /api/v1/wallet/accounts             # 지갑 계정 목록 (내부 전용)
POST /api/v1/wallet/account/new          # 새 계정 생성 (내부 전용)
POST /api/v1/block                       # 테스트용 블록 생성 (내부 전용)
```

**WebSocket:**
```
ws://localhost:8000/ws
이벤트: new_block, new_transaction, block_confirmed, consensus_state_change
```

**WebSocket 즉시 응답:** 연결 시 현재 컨센서스 상태와 최신 블록을 즉시 전송

**⚠️ 클라이언트 TX 서명 (POST /api/v1/tx/signed):**
```
1. SubmitSignedTxReq는 networkId 필드를 받지 않음!
2. 노드가 내부적으로 networkId: "" 로 TX ID 계산
3. 클라이언트도 동일하게 networkId: "" 사용해야 함

JSON 인코딩 규칙:
- []byte (publicKey, data): Base64 문자열
- [32]byte (id, txId): 숫자 배열 [0,0,0,...]
- [72]byte (signature): 숫자 배열 [0,0,0,...]
- [20]byte (address): 숫자 배열 [152,118,...]

TX ID 계산용 JSON 필드 순서 (Go 구조체 순서):
version → networkId → id → timestamp → inputs → outputs → memo → data

Input 필드 순서:
txId → outputIndex → signature → publicKey

Output 필드 순서:
address → amount → txType
```

#### 6. Wallet (wallet/)
- **HD 지갑** (BIP-39, BIP-44)
- 니모닉 기반 키 파생
- AES 암호화 키스토어
- JSON 형식 저장

```json
{
  "mnemonic": "encrypted...",
  "accounts": [
    {
      "address": "0x...",
      "publicKey": "...",
      "privateKey": "encrypted...",
      "index": 0
    }
  ]
}
```

### 데이터 레이어

#### LevelDB 구조
- **Prefix 기반 키 설계** (protocol/prefix.go)
```
meta:height               -> 최신 높이
meta:blockhash            -> 최신 블록 해시
block:hash:<hash>         -> 블록 데이터
block:height:<height>     -> 블록 해시
utxo:<txid>:<index>       -> UTXO
tx:<txid>                 -> 트랜잭션
```

#### 직렬화
- **DB 저장**: GOB 포맷 (`utils.SerializationFormatGob`)
- **API 응답**: JSON 포맷 (`utils.SerializationFormatJSON`)

## 🔄 핵심 플로우

### 블록 생성 플로우 (PoA/BFT)
```
1. Consensus Engine (1초 타이머)
   ↓
2. 제안자 선택 (RoundRobin/VRF/Hybrid)
   ↓
3. [PROPOSING] 내가 제안자인 경우 블록 생성
   ↓
4. Mempool에서 트랜잭션 선택
   ↓
5. 머클 루트 계산
   ↓
6. 블록 헤더 구성 (해시, 높이, 타임스탬프)
   ↓
7. 블록 해시 계산 (Header만으로)
   ↓
8. 제안자 주소 설정 + 블록 해시에 서명
   ↓
9. P2P로 Proposal 브로드캐스트
   ↓
10. [PREVOTING] 검증자들이 블록 검증 후 1차 투표
   ↓
11. [PRECOMMITTING] 2/3 이상 Prevote 시 2차 투표
   ↓
12. [COMMITTING] 2/3 이상 Precommit 시 블록 확정
   ↓
13. BlockChain에 추가 (DB 저장)
   ↓
14. WebSocket으로 new_block 이벤트 알림
   ↓
15. [IDLE] 다음 블록 대기
```

### P2P 동기화 플로우
```
1. 피어 연결 (TCP)
   ↓
2. 핸드셰이크 (버전, NodeID, NetworkID, 높이)
   ↓
3. 높이 비교
   ↓
4. 낮은 노드가 GetBlocks 메시지 전송
   ↓
5. 높은 노드가 Blocks 메시지 응답 (최대 100개)
   ↓
6. 수신 노드가 블록 검증 및 추가
   ↓
7. 실시간 NewBlock 메시지로 동기화 유지
```

### 트랜잭션 플로우
```
1. 클라이언트가 POST /api/v1/transaction
   ↓
2. 트랜잭션 검증 (UTXO, 서명, 잔액)
   ↓
3. Mempool에 추가
   ↓
4. P2P 브로드캐스트 (NewTx)
   ↓
5. Consensus가 다음 블록에 포함
   ↓
6. 블록 생성 시 UTXO 업데이트
```

## 🛠️ 개발 가이드

### 테스트
```bash
# 전체 테스트
go test ./...

# 특정 패키지
go test ./core -v
go test ./wallet -v

# 커버리지
go test -cover ./...

# 단일 테스트
go test -v -run TestCreateWallet ./wallet/...
```

### 로깅
```go
// 로그 레벨
logger.Debug("디버그 메시지: ", value)
logger.Info("정보 메시지")
logger.Warn("경고: ", warning)
logger.Error("에러: ", err)
```

- **로그 위치**: `./log/syslogs/`
- **로그 레벨**: config에서 설정 (`level = "debug"`)

### 코드 스타일
- Go 표준 스타일 가이드 준수
- `gofmt` 사용
- 주석은 **한글**로 작성
- 에러 처리 필수

## 📦 멀티 노드 환경

### 스크립트
```bash
# 전체 자동 셋업 (지갑 + 제네시스 + 시작)
./setup_multi_nodes.sh 3

# 개별 단계
./create_wallets.sh 3      # 지갑 생성
./setup_genesis.sh 3       # 제네시스 블록 복사
./start_multi_nodes.sh 3   # 노드 시작
./check_nodes.sh           # 상태 확인
./stop_all_nodes.sh        # 노드 중지
./clean_all.sh             # 데이터 정리
```

### 노드 역할
- **Node 1** (Boot/Producer)
  - Mode: `boot`
  - BlockProducer: `true`
  - P2P Port: `30303`
  - Public REST Port: `8000` (외부 접근 가능)
  - Internal REST Port: `8800` (localhost만)
  - 역할: 제네시스 블록 생성, 블록 생성, 부트스트랩

- **Node 2-N** (Validator/Sync-only)
  - Mode: `validator`
  - BlockProducer: `false`
  - P2P Port: `30304`, `30305`, ...
  - Public REST Port: `8001`, `8002`, ...
  - Internal REST Port: `8801`, `8802`, ...
  - BootNodes: `["127.0.0.1:30303"]`
  - 역할: 블록 동기화, 검증

### 설정 파일
- `config/config.toml`: Node 1
- `config/config_node2.toml`: Node 2
- `config/config_node3.toml`: Node 3
- (자동 생성: `start_multi_nodes.sh`)

### 제네시스 블록 동기화
⚠️ **중요**: 모든 노드는 동일한 제네시스 블록을 가져야 합니다.

```bash
# setup_genesis.sh가 자동으로 처리:
# 1. Boot 노드에서 제네시스 블록 생성
# 2. 다른 노드 DB에 제네시스 블록 복사
# 3. 모든 노드가 동일한 체인에서 시작
```

## 🔍 핵심 개념

### UTXO 모델
```
Transaction Input (TxInput):
- TxID: 이전 트랜잭션 ID
- OutIndex: 출력 인덱스
- Signature: 서명
- PublicKey: 공개키

Transaction Output (TxOutput):
- Address: 수신 주소
- Amount: 금액
- TxType: 트랜잭션 타입

잔액 계산:
Balance = Σ(해당 주소의 모든 UTXO)
```

### 블록 검증 (11단계)
1. **이전 해시 검증**: `PrevHash == 이전 블록 Hash`
2. **머클 루트 검증**: `MerkleRoot == calculateMerkleRoot(Txs)`
3. **블록 해시 검증**: `Hash == utils.Hash(Header)` (Header만으로 계산)
4. **높이 연속성**: `Height == 이전 Height + 1`
5. **타임스탬프**: `Timestamp >= 이전 Timestamp`
6. **제안자 주소 검증**: `Proposer != empty` (PoA)
7. **제안자 서명 검증**: 유효한 검증자 + 유효한 서명 (PoA)
8. **트랜잭션 개수**: `len(Txs) <= MaxTxsPerBlock`
9. **중복 트랜잭션**: 같은 TX ID 중복 방지
10. **중복 UTXO**: 같은 블록 내 동일 UTXO 사용 방지
11. **각 트랜잭션 검증**: UTXO 존재, 서명, 잔액

### 트랜잭션 검증
1. **UTXO 존재**: Input이 참조하는 UTXO가 존재하는가?
2. **서명 검증**: 공개키로 서명 검증
3. **잔액 검증**: `Σ(Inputs) >= Σ(Outputs)`
4. **이중 지불 방지**: 동일 UTXO 재사용 금지

## 🐛 디버깅 & 트러블슈팅

### 로그 확인
```bash
# 메인 노드
tail -f ./log/syslogs/_2025-12-09.log

# 노드 2
tail -f ./log/syslogs2/_2025-12-09.log

# 임시 로그
tail -f /tmp/abcfed_node1.log
```

### 일반적인 문제

#### 1. 노드 시작 실패
```bash
# 포트 충돌 확인
lsof -i :8000
lsof -i :30303

# 프로세스 정리
./stop_all_nodes.sh
pkill -9 abcfed
```

#### 2. 동기화 안됨
```bash
# 제네시스 블록 재설정
./setup_genesis.sh 3
./stop_all_nodes.sh
./start_multi_nodes.sh 3
```

#### 3. DB 손상
```bash
./clean_all.sh
./setup_multi_nodes.sh 3
```

#### 4. 지갑 없음
```bash
./create_wallets.sh 3
```

### 상태 확인
```bash
# 스크립트로 확인
./check_nodes.sh

# API로 확인
curl http://localhost:8000/api/v1/status
curl http://localhost:8001/api/v1/status
```

## 📚 참고 문서

- **[README.md](README.md)** - 프로젝트 개요
- **[QUICK_START.md](QUICK_START.md)** - 1분 빠른 시작
- **[README_SCRIPTS.md](README_SCRIPTS.md)** - 스크립트 상세 가이드
- **[USER_GUIDE.md](USER_GUIDE.md)** - 전체 사용자 가이드

## 🔑 중요 파일

### 핵심 소스
- `app/app.go` (282 lines) - 앱 통합
- `core/blockchain.go` (158 lines) - 체인 상태
- `core/validate.go` (260 lines) - 검증 로직
- `api/rest/handler.go` (738 lines) - API 핸들러
- `consensus/engine.go` - 블록 생성 엔진
- `p2p/p2p.go` - P2P 서비스
- `p2p/node.go` - 노드 관리

### 설정
- `config/config.toml` - 메인 설정
- `config/dev.config.toml` - 개발 환경
- `config/prod.config.toml` - 프로덕션

### 스크립트
- `setup_multi_nodes.sh` - 자동 셋업 (최고 우선순위)
- `setup_genesis.sh` - 제네시스 블록 셋업 (중요!)
- `start_multi_nodes.sh` - 노드 시작
- `check_nodes.sh` - 상태 확인

## 🔐 PoA/BFT 컨센서스 구현 현황

### ✅ 완료된 항목
| 항목 | 파일 | 설명 |
|------|------|------|
| Block에 Proposer/Signature | `core/block.go` | 제안자 주소 + 서명 필드 |
| 블록 생성 시 서명 | `consensus/engine.go` | `proposeBlock()`, `produceBlockSolo()` |
| 제안자/서명 검증 | `core/validate.go` | `ValidateProposer()`, `ValidateProposerSignature()` |
| ProposerValidator 인터페이스 | `core/blockchain.go` | 순환 참조 없이 검증 분리 |
| Consensus 인터페이스 구현 | `consensus/consensus.go` | `ValidateProposerSignature()`, `IsValidProposer()` |
| API 응답 컨센서스 정보 | `api/rest/handler.go` | `proposer`, `signature` 필드 |
| 제안자 선택 알고리즘 | `consensus/selection.go` | RoundRobin, VRF, Hybrid 지원 |
| P2P 블록 동기화 + 검증 | `app/app.go` | 블록 수신 시 PoA 검증 |
| **5단계 컨센서스 상태** | `consensus/consensus.go` | Idle→Proposing→Prevoting→Precommitting→Committing |
| **BFT 투표 메시지** | `p2p/message.go` | MsgTypeProposal, MsgTypeVote |
| **P2P 레이트 제한** | `p2p/ratelimit.go` | DoS 방지 |
| **WebSocket 즉시 응답** | `api/websocket.go` | 연결 시 현재 상태 전송 |
| **투표 서명 검증** | `consensus/validator.go` | Prevote/Precommit 서명 검증 |

### ❌ 미구현 항목
| 항목 | 설명 | 우선순위 |
|------|------|----------|
| 검증자 동적 추가/제거 | 현재 config 고정 | 중간 |
| 슬래싱 메커니즘 | 제안자 미이행 패널티 | 중간 |
| 에포크 기반 검증자 교체 | 주기적 업데이트 | 낮음 |

### 주요 코드 위치
```go
// 컨센서스 상태 (consensus/consensus.go)
const (
    StateIdle         = "IDLE"
    StateProposing    = "PROPOSING"
    StatePrevoting    = "PREVOTING"
    StatePrecommitting= "PRECOMMITTING"
    StateCommitting   = "COMMITTING"
)

// 제안자 선택 (consensus/selection.go)
SelectProposer(height, round)        // Round-Robin
SelectProposerVRF(height, round, prevBlockHash)   // VRF 기반
SelectProposerHybrid(height, round, prevBlockHash) // Hybrid

// 블록 생성 + 서명 (consensus/engine.go)
func (e *ConsensusEngine) proposeBlock() {
    newBlock := e.blockchain.SetBlock(prevHash, height, proposerAddr)
    sig, _ := e.consensus.LocalProposer.signBlockHash(newBlock.Header.Hash)
    newBlock.SignBlock(sig)
}

// 서명 검증 (consensus/validator.go)
func (v *Validator) ValidateBlockSignature(blockHash, sig) bool {
    return crypto.VerifySignature(publicKey, hashBytes, sig)
}
```

## 💡 개발 팁

1. **동시성**: BlockChain은 `sync.RWMutex`로 보호됨
2. **배치 쓰기**: DB 작업은 `leveldb.Batch` 사용
3. **제네시스 블록**: Boot 노드만 자동 생성, sync-only 노드는 P2P로 수신
4. **버퍼 크기**: P2P 메시지 버퍼 10MB (대용량 블록 전송)
5. **블록 동기화**: 최대 100개 블록씩 전송
6. **에러 처리**: 모든 에러는 로그 출력 후 처리
7. **블록 해시**: Header만으로 계산 (JSON 직렬화)
8. **PoA 검증**: `ProposerValidator` 인터페이스로 순환 참조 방지
9. **블록 생성 간격**: 1초 (BlockIntervalMs = 1000)
10. **제안자 선택**: Hybrid 모드 권장 (VRF + Round-Robin)

---

## 📚 추가 문서

프로젝트 내 `docs/` 폴더에 상세 문서가 있습니다:

- `docs/consensus/bft-consensus.md` - BFT 컨센서스 상세
- `docs/consensus/proposer-selection.md` - 제안자 선택 알고리즘
- `docs/consensus/state-machine.md` - 상태 머신
- `docs/consensus/timeout-recovery.md` - 타임아웃 복구
- `docs/api/websocket-api.md` - WebSocket API
- `docs/frontend/node-visualization.md` - 노드 시각화 가이드

---

**질문이나 이슈가 있으면 GitHub Issues를 활용하세요!**
