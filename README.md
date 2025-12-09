# ABCFe Blockchain Node

ABCFe는 UTXO 기반의 블록체인 노드 구현입니다. PoS 컨센서스, P2P 네트워크, REST API를 제공하며, 멀티 노드 환경을 지원합니다.

## 🚀 빠른 시작

### 단일 노드
```bash
# 1. 빌드
make build

# 2. 지갑 생성
./abcfed wallet create --wallet-dir=./resource/wallet

# 3. 노드 시작
./abcfed
```

### 멀티 노드 (추천)
```bash
# 1. 빌드
make build

# 2. 전체 자동 셋업 (3개 노드)
./setup_multi_nodes.sh 3

# 완료! 🎉
```

**더 자세한 내용은 [QUICK_START.md](QUICK_START.md)를 참고하세요.**

---

## 📚 문서

### 시작하기
- **[QUICK_START.md](QUICK_START.md)** - 1분 빠른 시작 가이드 ⭐
- **[README_SCRIPTS.md](README_SCRIPTS.md)** - 스크립트 상세 가이드

### 사용자 가이드
- **[USER_GUIDE.md](USER_GUIDE.md)** - 전체 사용자 가이드 (지갑, API, 트랜잭션)

### 개발자 가이드
- **[CLAUDE.md](CLAUDE.md)** - 아키텍처 및 개발 가이드

---

## 🛠️ 주요 기능

### 블록체인 코어
- ✅ **UTXO 모델** - 트랜잭션 입출력 기반
- ✅ **블록 생성** - 3초마다 자동 생성
- ✅ **블록 검증** - 해시, 높이, 타임스탬프, 머클 루트
- ✅ **트랜잭션 풀** - Mempool 관리

### 컨센서스
- ✅ **PoS (Proof of Stake)** - 스테이킹 기반
- ✅ **검증자 관리** - 가중 랜덤 선출
- ✅ **블록 제안** - 제안자 선출 및 블록 생성

### P2P 네트워크
- ✅ **멀티 노드 지원** - 자동 블록 동기화
- ✅ **피어 관리** - 핸드셰이크 프로토콜
- ✅ **실시간 브로드캐스트** - 블록/트랜잭션 전파
- ✅ **높이 기반 동기화** - 최대 100개 블록씩 전송

### API & WebSocket
- ✅ **REST API** - 블록, 트랜잭션, 주소 조회
- ✅ **WebSocket** - 실시간 이벤트 알림
- ✅ **CORS 지원** - 크로스 도메인 요청

### 지갑
- ✅ **HD 지갑** - BIP-39, BIP-44 표준
- ✅ **니모닉** - 12-24단어 복구 문구
- ✅ **계층적 주소** - 무한대 주소 생성
- ✅ **암호화 저장** - AES 암호화 키스토어

### 관리 도구
- ✅ **자동 셋업 스크립트** - 멀티 노드 환경 자동 구성
- ✅ **상태 모니터링** - 실시간 노드 상태 확인
- ✅ **제네시스 블록 관리** - 자동 배포

---

## 🎯 관리 스크립트

### 전체 자동 셋업
```bash
./setup_multi_nodes.sh 3    # 3개 노드 자동 셋업
```

### 개별 스크립트
```bash
./create_wallets.sh 3       # 지갑 생성
./setup_genesis.sh 3        # 제네시스 블록 셋업
./start_multi_nodes.sh 3    # 노드 시작
./check_nodes.sh            # 상태 확인
./stop_all_nodes.sh         # 노드 중지
./clean_all.sh              # 데이터 정리
```

**더 자세한 내용은 [README_SCRIPTS.md](README_SCRIPTS.md)를 참고하세요.**

---

## 🔗 API 엔드포인트

### 노드 & 블록체인
```bash
# 노드 상태
curl http://localhost:8000/api/v1/status

# 블록 목록 (페이징)
curl "http://localhost:8000/api/v1/blocks?page=1&limit=10"

# 특정 블록 조회
curl http://localhost:8000/api/v1/block/height/100
curl http://localhost:8000/api/v1/block/hash/0x...
```

### 주소 & 잔액
```bash
# 주소 잔액
curl http://localhost:8000/api/v1/address/0x.../balance

# 주소 UTXO
curl http://localhost:8000/api/v1/address/0x.../utxo
```

### 트랜잭션
```bash
# 트랜잭션 전송
curl -X POST http://localhost:8000/api/v1/transaction \
  -H "Content-Type: application/json" \
  -d '{...}'

# 멤풀 상태
curl http://localhost:8000/api/v1/mempool
```

### 컨센서스
```bash
# 컨센서스 상태
curl http://localhost:8000/api/v1/consensus/status
```

### WebSocket
```javascript
const ws = new WebSocket('ws://localhost:8000/ws');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data.type); // newBlock, newTransaction, chainSync
};
```

**전체 API 문서는 [USER_GUIDE.md](USER_GUIDE.md#5-rest-api-사용)를 참고하세요.**

---

## 📊 아키텍처

```
┌─────────────────────────────────────────────────────┐
│                    ABCFe Node                       │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌──────────────┐  ┌──────────────┐              │
│  │  REST API    │  │  WebSocket   │              │
│  │  (port 8000) │  │  (port 8000) │              │
│  └──────┬───────┘  └──────┬───────┘              │
│         │                  │                       │
│         └──────────┬───────┘                       │
│                    │                               │
│         ┌──────────▼──────────┐                   │
│         │    BlockChain       │                   │
│         │  (Core, Validate)   │                   │
│         └──────────┬──────────┘                   │
│                    │                               │
│    ┌───────────────┼───────────────┐              │
│    │               │               │              │
│ ┌──▼───┐      ┌───▼────┐     ┌───▼────┐         │
│ │ UTXO │      │Mempool │     │ LevelDB│         │
│ └──────┘      └────────┘     └────────┘         │
│                                                     │
│ ┌─────────────────────────────────────────┐       │
│ │         Consensus Engine                │       │
│ │  (PoS, Proposer, Validator)            │       │
│ └─────────────────────────────────────────┘       │
│                                                     │
│ ┌─────────────────────────────────────────┐       │
│ │         P2P Network                     │       │
│ │  (Sync, Broadcast, Peer Management)    │       │
│ └─────────────────────────────────────────┘       │
│                                                     │
│ ┌─────────────────────────────────────────┐       │
│ │         HD Wallet                       │       │
│ │  (BIP-39, BIP-44, Keystore)            │       │
│ └─────────────────────────────────────────┘       │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 디렉토리 구조
```
abcfe-node/
├── cmd/node/          # 메인 애플리케이션
├── app/               # 앱 통합 레이어
├── core/              # 블록체인 코어
├── consensus/         # PoS 컨센서스
├── p2p/               # P2P 네트워크
├── api/rest/          # REST API & WebSocket
├── wallet/            # HD 지갑
├── common/            # 공통 유틸리티
├── config/            # 설정 파일
├── storage/           # LevelDB 저장소
└── protocol/          # 프로토콜 타입
```

---

## 🔧 개발

### 빌드 & 테스트
```bash
# 빌드
make build

# 테스트
make test
go test ./...
go test ./core -v

# 정리
make clean
```

### 설정 파일
- `config/config.toml` - 메인 설정 (Node 1)
- `config/dev.config.toml` - 개발 환경
- `config/prod.config.toml` - 프로덕션 환경
- `config/config_node2.toml` - Node 2 (자동 생성)

### 로그
```bash
# 메인 로그
tail -f ./log/syslogs/_2025-12-09.log

# 노드 2 로그
tail -f ./log/syslogs2/_2025-12-09.log

# 임시 로그
tail -f /tmp/abcfed_node1.log
```

---

## 🌐 멀티 노드 환경

### 노드 역할

**Node 1 (Boot/Producer)**
- P2P 포트: 30303
- REST 포트: 8000
- 역할: 제네시스 블록 생성, 블록 생성, 부트스트랩

**Node 2-N (Validator/Sync)**
- P2P 포트: 30304, 30305, ...
- REST 포트: 8001, 8002, ...
- 역할: 블록 동기화, 검증
- Boot 노드: 127.0.0.1:30303

### 동기화 프로세스
1. 핸드셰이크로 높이 정보 교환
2. 낮은 높이 노드가 블록 요청
3. 높은 높이 노드가 블록 전송 (최대 100개씩)
4. 수신 노드가 블록 검증 및 추가
5. 실시간 새 블록 수신으로 동기화 유지

---

## 📝 주요 특징

### UTXO 모델
```
Transaction:
  Inputs:  [이전 UTXO 참조]
  Outputs: [새로운 UTXO 생성]

잔액 = Σ(해당 주소의 모든 UTXO)
```

### 블록 구조
```go
Block {
  Header: {
    Hash       // 블록 해시
    PrevHash   // 이전 블록 해시
    Height     // 블록 높이
    Timestamp  // 생성 시간
    MerkleRoot // 트랜잭션 머클 루트
  }
  Transactions: [...]
}
```

### 제네시스 블록
- 고정 타임스탬프 (2024-01-01 00:00:00 UTC)
- 시스템 주소로 초기 밸런스 배포
- Boot 노드가 생성, 다른 노드는 복사

---

## ⚠️ 중요 사항

### 제네시스 블록 동기화
모든 노드는 **동일한 제네시스 블록**을 가져야 합니다.

```bash
# 제네시스 블록 셋업 (필수!)
./setup_genesis.sh 3
```

### 포트 사용
- P2P: 30303, 30304, 30305, ...
- REST: 8000, 8001, 8002, ...

포트가 이미 사용 중이면 노드가 시작되지 않습니다.

### 지갑 백업
지갑 파일(`./resource/wallet*/wallet.json`)은 **반드시 백업**하세요.
지갑을 잃어버리면 자산을 복구할 수 없습니다.

---

## 🐛 트러블슈팅

### 노드가 시작되지 않음
```bash
# 포트 확인
lsof -i :8000
lsof -i :30303

# 프로세스 정리
./stop_all_nodes.sh
pkill -9 abcfed
```

### 동기화가 안됨
```bash
# 제네시스 블록 재설정
./setup_genesis.sh 3
./stop_all_nodes.sh
./start_multi_nodes.sh 3
```

### DB 손상
```bash
# 완전 초기화
./clean_all.sh
./setup_multi_nodes.sh 3
```

**더 자세한 트러블슈팅은 [CLAUDE.md](CLAUDE.md#-디버깅--트러블슈팅)를 참고하세요.**

---

## 📖 더 알아보기

- [QUICK_START.md](QUICK_START.md) - 1분 빠른 시작
- [README_SCRIPTS.md](README_SCRIPTS.md) - 스크립트 가이드
- [USER_GUIDE.md](USER_GUIDE.md) - 사용자 가이드
- [CLAUDE.md](CLAUDE.md) - 개발자 가이드

---

## 📄 라이센스

MIT License

---

## 🤝 기여

이슈와 PR은 언제나 환영합니다!

**질문이 있으시면 GitHub Issues를 활용해주세요.**
