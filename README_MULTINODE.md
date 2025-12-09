# 멀티 노드 실행 가이드

## 빠른 시작

### 1단계: 노드별 지갑 생성

먼저 각 노드마다 지갑을 생성해야 합니다:

```bash
# 5개 노드용 지갑 생성
./create_wallets.sh 5
```

이 명령은:
- 각 노드의 지갑 디렉토리 생성 (`./resource/wallet`, `./resource/wallet2`, ...)
- 지갑 파일 생성 (`wallet.json`)
- 니모닉과 주소 출력

**⚠️ 중요: 니모닉은 안전하게 보관하세요!**

### 2단계: 멀티 노드 실행

```bash
# 5개 노드 실행
./start_multi_nodes.sh 5
```

### 3단계: 노드 상태 확인

```bash
./check_nodes.sh
```

## 상세 가이드

### 개별 노드 지갑 생성

특정 노드의 지갑만 생성하려면:

```bash
# Node 1 (기본 지갑)
./abcfed wallet create --wallet-dir="./resource/wallet"

# Node 2
./abcfed wallet create --wallet-dir="./resource/wallet2"

# Node 3
./abcfed wallet create --wallet-dir="./resource/wallet3"
```

### 지갑 확인

```bash
# Node 1 지갑 목록
./abcfed wallet list --wallet-dir="./resource/wallet"

# Node 2 지갑 목록
./abcfed wallet list --wallet-dir="./resource/wallet2"
```

### 니모닉으로 지갑 복구

```bash
./abcfed wallet restore --wallet-dir="./resource/wallet2" \
  --mnemonic="your twelve word mnemonic phrase here"
```

### 수동으로 노드 실행

각 노드를 개별적으로 실행:

```bash
# Terminal 1: Node 1
./abcfed

# Terminal 2: Node 2
./abcfed --config config/config_node2.toml

# Terminal 3: Node 3
./abcfed --config config/config_node3.toml
```

## 디렉토리 구조

```
abcfe-node-newest/
├── resource/
│   ├── wallet/           # Node 1 지갑
│   │   └── wallet.json
│   ├── wallet2/          # Node 2 지갑
│   │   └── wallet.json
│   ├── wallet3/          # Node 3 지갑
│   │   └── wallet.json
│   ├── db/               # Node 1 DB
│   ├── db2/              # Node 2 DB
│   └── db3/              # Node 3 DB
├── config/
│   ├── config.toml       # Node 1 설정
│   ├── config_node2.toml # Node 2 설정
│   └── config_node3.toml # Node 3 설정
└── log/
    ├── syslogs/          # Node 1 로그
    ├── syslogs2/         # Node 2 로그
    └── syslogs3/         # Node 3 로그
```

## 문제 해결

### 지갑 파일 없음 오류

```
Error: Failed to load wallet: failed to read wallet file
```

**해결**: 노드 실행 전에 지갑을 먼저 생성하세요.

```bash
./create_wallets.sh [노드 개수]
```

### 포트 이미 사용 중

```
Error: bind: address already in use
```

**해결**: 기존 노드를 먼저 중지하세요.

```bash
./stop_all_nodes.sh
```

### 지갑 초기화

기존 지갑을 삭제하고 새로 생성:

```bash
# 주의: 지갑 데이터가 영구 삭제됩니다!
rm -rf ./resource/wallet*
./create_wallets.sh 5
```

## 유용한 명령어

### 모든 노드 중지

```bash
./stop_all_nodes.sh
```

### 노드 상태 실시간 확인

```bash
watch -n 2 ./check_nodes.sh
```

### 특정 노드 로그 확인

```bash
# Node 1
tail -f /tmp/abcfed_node1.log

# Node 2
tail -f /tmp/abcfed_node2.log
```

### 특정 노드 API 호출

```bash
# Node 1 상태
curl http://localhost:8000/api/v1/status | jq

# Node 2 상태
curl http://localhost:8001/api/v1/status | jq

# Node 3 상태
curl http://localhost:8002/api/v1/status | jq
```

## 테스트 시나리오

### 시나리오 1: 기본 멀티 노드 테스트

```bash
# 1. 지갑 생성
./create_wallets.sh 3

# 2. 노드 실행
./start_multi_nodes.sh 3

# 3. 상태 확인
./check_nodes.sh

# 4. 모든 노드 동기화 확인
for port in 8000 8001 8002; do
  echo "Node at port $port:"
  curl -s http://localhost:${port}/api/v1/status | jq '.data.currentHeight'
done

# 5. 중지
./stop_all_nodes.sh
```

### 시나리오 2: 트랜잭션 전파 테스트

```bash
# Node 1에서 트랜잭션 전송
curl -X POST http://localhost:8000/api/v1/tx/send \
  -H "Content-Type: application/json" \
  -d '{
    "accountIndex": 0,
    "to": "0x9876543210fedcba9876543210fedcba98765432",
    "amount": 1000,
    "memo": "Test transaction"
  }'

# 모든 노드의 mempool 확인
for port in 8000 8001 8002; do
  echo "Node $port mempool:"
  curl -s http://localhost:${port}/api/v1/mempool/list | jq
done
```

## 참고

- [USER_GUIDE.md](USER_GUIDE.md) - 전체 사용자 가이드
- [CLAUDE.md](CLAUDE.md) - 개발자 가이드

