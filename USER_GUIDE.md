# ABCFe Node 사용자 가이드

이 가이드는 ABCFe 블록체인 노드를 실행하고, 지갑을 생성하며, 트랜잭션을 전송하고, API 및 WebSocket을 사용하는 전체 과정을 설명합니다.

## 목차

1. [노드 빌드 및 실행](#1-노드-빌드-및-실행)
2. [지갑 관리](#2-지갑-관리)
3. [트랜잭션 전송](#3-트랜잭션-전송)
4. [REST API 사용](#4-rest-api-사용)
5. [WebSocket 실시간 알림](#5-websocket-실시간-알림)
6. [멀티 노드 환경](#6-멀티-노드-환경)

---

## 1. 노드 빌드 및 실행

### 1.1 빌드

```bash
# 프로젝트 루트 디렉토리에서
make build

# 또는 직접 빌드
go build -o abcfed cmd/node/main.go
```

빌드가 완료되면 `abcfed` 실행 파일이 생성됩니다.

### 1.2 설정 파일 확인

노드를 실행하기 전에 `config/config.toml` 파일을 확인하고 필요에 따라 수정합니다:

```toml
[Common]
Mode = "dev"
Port = 3000

[DB]
Path = "./resource/db/leveldb_3000.db"

[Wallet]
Path = "./resource/wallet"

[Server]
RestPort = 8000

[Genesis]
InitialAddresses = [
    "0x1234567890abcdef1234567890abcdef12345678",
]
InitialBalances = [100000000]
```

### 1.3 노드 실행

```bash
# 포그라운드 실행
./abcfed

# 또는 데몬 모드로 실행
./abcfed node start

# 데몬 상태 확인
./abcfed node status

# 데몬 중지
./abcfed node stop

# 데몬 재시작
./abcfed node restart
```

노드가 정상적으로 실행되면 다음과 같은 로그가 출력됩니다:

```
[INFO] Starting ABCFe Node...
[INFO] REST Server started on :8000
[INFO] WebSocket Server started on /ws
[INFO] Consensus engine started
[INFO] Blockchain initialized at height: 0
```

---

## 2. 지갑 관리

### 2.1 새 지갑 생성

```bash
./abcfed wallet create
```

출력 예시:
```
Wallet created successfully!
Mnemonic: word1 word2 word3 ... word12
Address: 0xabcdef1234567890abcdef1234567890abcdef12
Please save your mnemonic phrase securely!
```

⚠️ **중요**: 니모닉 문구는 안전한 곳에 보관하세요. 지갑 복구에 필요합니다.

### 2.2 기존 지갑 복구

```bash
./abcfed wallet restore
```

프롬프트에서 니모닉 문구를 입력하면 지갑이 복구됩니다.

### 2.3 지갑 계정 추가

하나의 지갑에 여러 계정(주소)을 생성할 수 있습니다:

```bash
./abcfed wallet add-account
```

### 2.4 지갑 정보 조회

```bash
# 지갑 목록 조회
./abcfed wallet list

# 니모닉 조회 (보안 주의)
./abcfed wallet show-mnemonic
```

출력 예시:
```
Wallet: wallet.json
Accounts:
  [0] 0xabcdef1234567890abcdef1234567890abcdef12
  [1] 0x9876543210fedcba9876543210fedcba98765432
```

---

## 3. 트랜잭션 전송

### 3.1 방법 1: 지갑을 통한 전송 (간편)

노드가 지갑을 관리하고 자동으로 서명합니다.

**API 엔드포인트**: `POST /api/v1/tx/send`

**요청 예시** (curl):
```bash
curl -X POST http://localhost:8000/api/v1/tx/send \
  -H "Content-Type: application/json" \
  -d '{
    "accountIndex": 0,
    "to": "0x9876543210fedcba9876543210fedcba98765432",
    "amount": 5000,
    "memo": "Payment for services",
    "data": null
  }'
```

**요청 파라미터**:
- `accountIndex`: 지갑 내 계정 인덱스 (0부터 시작)
- `to`: 수신자 주소 (0x로 시작하는 40자리 hex)
- `amount`: 전송할 코인 수량
- `memo`: 선택적 메모
- `data`: 선택적 추가 데이터 (바이트 배열)

**응답 예시**:
```json
{
  "status": "success",
  "data": {
    "txId": "0xabcd1234...",
    "message": "Transaction submitted to mempool"
  }
}
```

### 3.2 방법 2: 서명된 트랜잭션 제출 (고급)

외부에서 서명한 트랜잭션을 제출합니다.

**API 엔드포인트**: `POST /api/v1/tx/signed`

**요청 예시**:
```bash
curl -X POST http://localhost:8000/api/v1/tx/signed \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": [
      {
        "txId": "0x1234...",
        "outputIndex": 0,
        "signature": "0xabcd...",
        "publicKey": "0x04..."
      }
    ],
    "outputs": [
      {
        "to": "0x9876543210fedcba9876543210fedcba98765432",
        "amount": 5000
      },
      {
        "to": "0xabcdef1234567890abcdef1234567890abcdef12",
        "amount": 4900
      }
    ],
    "memo": "Signed transaction",
    "data": null,
    "txType": 0
  }'
```

**서명 프로세스**:
1. UTXO 조회 (`GET /api/v1/address/{address}/utxo`)
2. 트랜잭션 구성 (inputs + outputs)
3. 각 input에 대해 ECDSA 서명 생성 (private key 사용)
4. 서명된 트랜잭션 제출

---

## 4. REST API 사용

모든 API는 `http://localhost:8000/api/v1` 경로를 사용합니다.

### 4.1 노드 상태 조회

```bash
curl http://localhost:8000/api/v1/status
```

**응답**:
```json
{
  "status": "success",
  "data": {
    "currentHeight": 42,
    "currentBlockHash": "0xabcd1234...",
    "genesisHash": "0x0000...",
    "networkId": "abcfe-mainnet",
    "mempoolSize": 3
  }
}
```

### 4.2 블록 조회

#### 최신 블록
```bash
curl http://localhost:8000/api/v1/block/latest
```

#### 높이로 조회
```bash
curl http://localhost:8000/api/v1/block/height/42
```

#### 해시로 조회
```bash
curl http://localhost:8000/api/v1/block/hash/0xabcd1234...
```

#### 블록 목록 (페이지네이션)
```bash
curl "http://localhost:8000/api/v1/blocks?page=1&limit=10"
```

**응답**:
```json
{
  "status": "success",
  "data": {
    "blocks": [...],
    "total": 42,
    "page": 1,
    "limit": 10,
    "totalPages": 5
  }
}
```

### 4.3 트랜잭션 조회

```bash
curl http://localhost:8000/api/v1/tx/0xabcd1234...
```

### 4.4 주소 관련 조회

#### 잔액 조회
```bash
curl http://localhost:8000/api/v1/address/0xabcd.../balance
```

**응답**:
```json
{
  "status": "success",
  "data": {
    "address": "0xabcd...",
    "balance": 10000
  }
}
```

#### UTXO 조회
```bash
curl http://localhost:8000/api/v1/address/0xabcd.../utxo
```

**응답**:
```json
{
  "status": "success",
  "data": {
    "utxos": [
      {
        "txId": "0x1234...",
        "outputIndex": 0,
        "amount": 5000,
        "address": "0xabcd...",
        "height": 40
      }
    ]
  }
}
```

### 4.5 멤풀 조회

```bash
curl http://localhost:8000/api/v1/mempool/list
```

### 4.6 컨센서스 상태 조회

```bash
curl http://localhost:8000/api/v1/consensus/status
```

**응답**:
```json
{
  "status": "success",
  "data": {
    "state": "IDLE",
    "currentHeight": 42,
    "currentRound": 0,
    "proposer": "0xabcd...",
    "validators": [
      {
        "address": "0xabcd...",
        "stakingAmount": 100000,
        "isActive": true
      }
    ],
    "votingPower": {
      "0xabcd...": 100000
    }
  }
}
```

**컨센서스 상태**:
- `IDLE`: 대기 중
- `PROPOSING`: 블록 제안 중
- `VOTING`: 투표 진행 중
- `COMMITTING`: 블록 커밋 중

### 4.7 네트워크 통계

```bash
curl http://localhost:8000/api/v1/stats
```

**응답**:
```json
{
  "status": "success",
  "data": {
    "blockHeight": 42,
    "totalTransactions": 150,
    "mempoolSize": 3,
    "activeConnections": 5,
    "avgBlockTime": 5.2
  }
}
```

### 4.8 지갑 계정 조회 (API)

```bash
curl http://localhost:8000/api/v1/wallet/accounts
```

### 4.9 새 계정 생성 (API)

```bash
curl -X POST http://localhost:8000/api/v1/wallet/account/new
```

---

## 5. WebSocket 실시간 알림

WebSocket을 통해 블록체인 이벤트를 실시간으로 수신할 수 있습니다.

### 5.1 연결

**WebSocket URL**: `ws://localhost:8000/ws`

### 5.2 이벤트 타입

1. **new_block**: 새 블록이 생성되었을 때
2. **new_transaction**: 새 트랜잭션이 멤풀에 추가되었을 때
3. **block_confirmed**: 블록이 확정되었을 때
4. **consensus_state_change**: 컨센서스 상태가 변경되었을 때

### 5.3 JavaScript 예제

```javascript
// WebSocket 연결
const ws = new WebSocket('ws://localhost:8000/ws');

// 연결 성공
ws.onopen = () => {
  console.log('WebSocket connected');
};

// 메시지 수신
ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  
  switch(message.type) {
    case 'new_block':
      console.log('New block:', message.data);
      // 블록 정보: height, hash, timestamp, txCount 등
      updateBlockUI(message.data);
      break;
      
    case 'new_transaction':
      console.log('New transaction:', message.data);
      // 트랜잭션 정보: txId, from, to, amount 등
      updateMempoolUI(message.data);
      break;
      
    case 'block_confirmed':
      console.log('Block confirmed:', message.data);
      // 확정된 블록 정보
      break;
      
    case 'consensus_state_change':
      console.log('Consensus state:', message.data);
      // 컨센서스 상태: state, height, round, proposer
      updateConsensusUI(message.data);
      break;
  }
};

// 연결 종료
ws.onclose = () => {
  console.log('WebSocket disconnected');
};

// 에러 처리
ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};
```

### 5.4 이벤트 데이터 예시

#### new_block
```json
{
  "type": "new_block",
  "data": {
    "height": 43,
    "hash": "0xabcd1234...",
    "prevHash": "0x9876...",
    "timestamp": 1702123456,
    "txCount": 5,
    "merkleRoot": "0xdef456...",
    "proposer": "0xabcd..."
  },
  "timestamp": 1702123456
}
```

#### consensus_state_change
```json
{
  "type": "consensus_state_change",
  "data": {
    "state": "PROPOSING",
    "height": 43,
    "round": 0,
    "proposer": "0xabcd1234..."
  },
  "timestamp": 1702123456
}
```

### 5.5 Python 예제

```python
import websocket
import json

def on_message(ws, message):
    data = json.loads(message)
    print(f"Received: {data['type']}")
    print(f"Data: {data['data']}")

def on_error(ws, error):
    print(f"Error: {error}")

def on_close(ws, close_status_code, close_msg):
    print("WebSocket closed")

def on_open(ws):
    print("WebSocket connected")

if __name__ == "__main__":
    ws = websocket.WebSocketApp(
        "ws://localhost:8000/ws",
        on_open=on_open,
        on_message=on_message,
        on_error=on_error,
        on_close=on_close
    )
    ws.run_forever()
```

---

## 6. 멀티 노드 환경

### 6.1 두 번째 노드 설정

`config/config_node2.toml` 파일을 생성하거나 수정:

```toml
[Common]
Mode = "dev"
Port = 3001

[DB]
Path = "./resource/db2/leveldb_3001.db"

[Wallet]
Path = "./resource/wallet2"

[Server]
RestPort = 8001

[P2P]
BootstrapNodes = ["localhost:3000"]
```

### 6.2 두 번째 노드 실행

```bash
./abcfed --config config/config_node2.toml
```

### 6.3 멀티 노드 테스트 스크립트

프로젝트에 포함된 `test_multi_node.sh` 스크립트를 사용:

```bash
chmod +x test_multi_node.sh
./test_multi_node.sh
```

이 스크립트는 자동으로:
1. 두 개의 노드를 시작
2. 블록 동기화 확인
3. 트랜잭션 전송 테스트
4. 양쪽 노드의 상태 비교

### 6.4 노드 간 동기화 확인

**Node 1**:
```bash
curl http://localhost:8000/api/v1/status
```

**Node 2**:
```bash
curl http://localhost:8001/api/v1/status
```

두 노드의 `currentHeight`와 `currentBlockHash`가 동일해야 합니다.

---

## 7. 실전 시나리오

### 7.1 시나리오: Genesis → User1 → User2 코인 전송

#### Step 1: 노드 시작
```bash
./abcfed
```

#### Step 2: User1 지갑 생성
```bash
./abcfed wallet create
# Address 저장: 0xUser1Address...
```

#### Step 3: Genesis가 User1에게 코인 전송

Genesis 주소는 `config.toml`의 `InitialAddresses`에 정의되어 있습니다.

```bash
curl -X POST http://localhost:8000/api/v1/tx/send \
  -H "Content-Type: application/json" \
  -d '{
    "accountIndex": 0,
    "to": "0xUser1Address...",
    "amount": 10000,
    "memo": "Initial funding"
  }'
```

#### Step 4: User1 잔액 확인
```bash
curl http://localhost:8000/api/v1/address/0xUser1Address.../balance
```

#### Step 5: User2 지갑 생성
```bash
./abcfed wallet create
# Address 저장: 0xUser2Address...
```

#### Step 6: User1 계정을 노드 지갑에 추가

User1의 니모닉으로 지갑 복구 또는 import:
```bash
./abcfed wallet restore
# User1의 니모닉 입력
```

#### Step 7: User1이 User2에게 코인 전송
```bash
curl -X POST http://localhost:8000/api/v1/tx/send \
  -H "Content-Type: application/json" \
  -d '{
    "accountIndex": 0,
    "to": "0xUser2Address...",
    "amount": 3000,
    "memo": "Payment to User2"
  }'
```

#### Step 8: User2 잔액 확인
```bash
curl http://localhost:8000/api/v1/address/0xUser2Address.../balance
```

#### Step 9: WebSocket으로 실시간 모니터링

브라우저 콘솔에서:
```javascript
const ws = new WebSocket('ws://localhost:8000/ws');
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

새 블록과 트랜잭션이 실시간으로 표시됩니다.

---

## 8. 문제 해결

### 8.1 노드가 시작되지 않음

**증상**: `./abcfed` 실행 시 에러 발생

**해결**:
1. 포트가 이미 사용 중인지 확인:
   ```bash
   lsof -i :8000
   lsof -i :3000
   ```
2. 기존 프로세스 종료:
   ```bash
   pkill -9 -f abcfed
   ```
3. DB 파일 권한 확인:
   ```bash
   ls -la resource/db/
   ```

### 8.2 트랜잭션이 실패함

**증상**: API 응답이 `"status": "error"`

**원인**:
- 잔액 부족
- 잘못된 주소 형식
- 서명 오류 (서명된 TX의 경우)

**해결**:
1. 잔액 확인:
   ```bash
   curl http://localhost:8000/api/v1/address/{address}/balance
   ```
2. UTXO 확인:
   ```bash
   curl http://localhost:8000/api/v1/address/{address}/utxo
   ```
3. 로그 확인:
   ```bash
   tail -f log/syslogs/_$(date +%Y-%m-%d).log
   ```

### 8.3 노드 간 동기화 안 됨

**증상**: 두 노드의 블록 높이가 다름

**해결**:
1. P2P 연결 확인 (향후 구현)
2. 두 노드 재시작
3. 제네시스 블록 일치 여부 확인

### 8.4 WebSocket 연결 실패

**증상**: `ws.onerror` 이벤트 발생

**해결**:
1. 노드가 실행 중인지 확인
2. 올바른 포트 사용 확인 (`config.toml`의 `RestPort`)
3. CORS 설정 확인 (크로스 도메인의 경우)

---

## 9. 추가 자료

- **CLAUDE.md**: 개발자용 아키텍처 및 명령어 가이드
- **config/config.toml**: 노드 설정 파일
- **Makefile**: 빌드 및 테스트 명령어

---

## 10. API 레퍼런스 요약

| 메서드 | 엔드포인트 | 설명 |
|--------|-----------|------|
| GET | `/api/v1/status` | 노드 상태 조회 |
| GET | `/api/v1/block/latest` | 최신 블록 |
| GET | `/api/v1/block/height/{height}` | 높이로 블록 조회 |
| GET | `/api/v1/block/hash/{hash}` | 해시로 블록 조회 |
| GET | `/api/v1/blocks` | 블록 목록 (페이지네이션) |
| GET | `/api/v1/tx/{txId}` | 트랜잭션 조회 |
| POST | `/api/v1/tx/send` | 지갑으로 트랜잭션 전송 |
| POST | `/api/v1/tx/signed` | 서명된 트랜잭션 제출 |
| GET | `/api/v1/address/{address}/balance` | 주소 잔액 조회 |
| GET | `/api/v1/address/{address}/utxo` | UTXO 조회 |
| GET | `/api/v1/mempool/list` | 멤풀 조회 |
| GET | `/api/v1/consensus/status` | 컨센서스 상태 |
| GET | `/api/v1/stats` | 네트워크 통계 |
| GET | `/api/v1/wallet/accounts` | 지갑 계정 목록 |
| POST | `/api/v1/wallet/account/new` | 새 계정 생성 |
| WS | `/ws` | WebSocket 연결 |

---

## 마치며

이 가이드는 ABCFe 블록체인의 기본적인 사용법을 다룹니다. 더 자세한 개발자 정보는 `CLAUDE.md` 파일을 참고하세요.

문제가 발생하거나 기능 요청이 있으면 이슈를 등록해주세요.

**Happy Blockchain Building! 🚀**

