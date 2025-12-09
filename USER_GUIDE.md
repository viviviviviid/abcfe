# ABCFe Node 사용자 가이드

이 가이드는 ABCFe 블록체인 노드를 실행하고, 지갑을 생성하며, 트랜잭션을 전송하고, API 및 WebSocket을 사용하는 전체 과정을 설명합니다.

## 🚀 빠른 시작

**처음 시작하는 경우:**
```bash
# 1. 빌드
make build

# 2. 멀티 노드 자동 셋업 (추천)
./setup_multi_nodes.sh 3

# 완료! 노드가 실행 중입니다.
```

**자세한 내용은 [QUICK_START.md](QUICK_START.md)를 참고하세요.**

---

## 사용자 구분

- **노드 운영자**: 노드를 직접 실행하고 관리하는 사용자 (CLI 사용)
- **일반 유저**: API를 통해 블록체인과 상호작용하는 사용자 (클라이언트 사이드 지갑)

## 목차

1. [노드 빌드 및 실행](#1-노드-빌드-및-실행) *(노드 운영자)*
2. [노드 운영자의 지갑 관리 (CLI)](#2-노드-운영자의-지갑-관리-cli) *(노드 운영자)*
3. [일반 유저의 지갑 관리 (클라이언트 사이드)](#3-일반-유저의-지갑-관리-클라이언트-사이드) *(일반 유저)*
4. [트랜잭션 전송](#4-트랜잭션-전송) *(모두)*
5. [REST API 사용](#5-rest-api-사용) *(모두)*
6. [WebSocket 실시간 알림](#6-websocket-실시간-알림) *(모두)*
7. [멀티 노드 환경](#7-멀티-노드-환경) *(노드 운영자)*
8. [관리 스크립트](#8-관리-스크립트) ⭐ *(노드 운영자)*

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

## 2. 노드 운영자의 지갑 관리 (CLI)

> 이 섹션은 노드 바이너리에 접근 가능한 **노드 운영자**를 위한 가이드입니다.
> **일반 유저**는 [섹션 3](#3-일반-유저의-지갑-관리-클라이언트-사이드)을 참고하세요.

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

### 2.2.1 외부에서 생성한 니모닉 사용

ABCFe CLI로 니모닉을 생성하지 않고, 외부 도구로 생성한 니모닉을 사용할 수도 있습니다.

⚠️ **중요**: 니모닉은 BIP-39 표준을 준수해야 합니다.

**Python으로 니모닉 생성**:
```python
from mnemonic import Mnemonic

mnemo = Mnemonic("english")
words = mnemo.generate(strength=128)  # 12단어
print(words)
# 출력: abandon ability able about above absent absorb abstract absurd abuse access accident
```

**JavaScript로 니모닉 생성**:
```javascript
const bip39 = require('bip39');
const mnemonic = bip39.generateMnemonic();
console.log(mnemonic);
```

**온라인 도구** (오프라인 사용 권장):
- https://iancoleman.io/bip39/ - 브라우저에서 BIP-39 니모닉 생성

생성한 니모닉을 `wallet restore` 명령어로 입력하면 지갑이 복구됩니다.

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

## 3. 일반 유저의 지갑 관리 (클라이언트 사이드)

> 이 섹션은 노드에 접근할 수 없는 **일반 유저**를 위한 가이드입니다.
> 클라이언트(브라우저, 앱)에서 지갑을 직접 관리합니다.

### 3.1 개요

일반 유저는 다음과 같은 방식으로 블록체인과 상호작용합니다:

1. **클라이언트 사이드에서 지갑 관리**
   - 니모닉 생성 및 저장 (사용자 책임)
   - 니모닉으로부터 개인키/공개키 유도
   - 주소 생성

2. **트랜잭션 서명**
   - UTXO 조회 (API)
   - 트랜잭션 구성
   - 클라이언트에서 개인키로 서명

3. **API로 전송**
   - 서명된 트랜잭션을 `POST /api/v1/tx/signed`로 전송
   - 노드는 서명 검증 후 실행

### 3.2 Python 예제 (완전한 워크플로우)

```python
import hashlib
import ecdsa
from mnemonic import Mnemonic
import requests
import json

# 1. 니모닉 생성 (또는 기존 니모닉 사용)
mnemo = Mnemonic("english")
mnemonic_words = mnemo.generate(strength=128)  # 12단어
print(f"Mnemonic: {mnemonic_words}")

# 2. 니모닉으로부터 시드 생성
seed = mnemo.to_seed(mnemonic_words, passphrase="")

# 3. 개인키 유도 (간단한 예제, 실제로는 BIP-32/44 사용)
private_key_bytes = seed[:32]
sk = ecdsa.SigningKey.from_string(private_key_bytes, curve=ecdsa.SECP256k1)
vk = sk.get_verifying_key()

# 4. 공개키에서 주소 생성
public_key_bytes = b'\x04' + vk.to_string()  # 압축되지 않은 형식
addr_hash = hashlib.sha256(public_key_bytes).digest()[:20]
address = "0x" + addr_hash.hex()
print(f"Address: {address}")

# 5. UTXO 조회
response = requests.get(f"http://localhost:8000/api/v1/address/{address}/utxo")
utxos = response.json()["data"]["utxos"]
print(f"Available UTXOs: {len(utxos)}")

# 6. 트랜잭션 구성
recipient = "0x9876543210fedcba9876543210fedcba98765432"
amount = 1000

if utxos:
    utxo = utxos[0]  # 첫 번째 UTXO 사용
    
    # 트랜잭션 데이터 구성 (서명할 데이터)
    tx_data = {
        "txId": utxo["txId"],
        "outputIndex": utxo["outputIndex"],
        "to": recipient,
        "amount": amount
    }
    
    # 7. 서명 생성
    tx_bytes = json.dumps(tx_data, sort_keys=True).encode()
    tx_hash = hashlib.sha256(tx_bytes).digest()
    signature = sk.sign_digest(tx_hash, sigencode=ecdsa.util.sigencode_der)
    
    # 8. 서명된 트랜잭션 전송
    signed_tx = {
        "inputs": [{
            "txId": utxo["txId"],
            "outputIndex": utxo["outputIndex"],
            "signature": "0x" + signature.hex(),
            "publicKey": "0x" + public_key_bytes.hex()
        }],
        "outputs": [{
            "to": recipient,
            "amount": amount
        }, {
            "to": address,  # 잔액 반환
            "amount": utxo["amount"] - amount - 100  # 수수료 제외
        }],
        "memo": "Payment from Python client",
        "data": None,
        "txType": 0
    }
    
    response = requests.post(
        "http://localhost:8000/api/v1/tx/signed",
        json=signed_tx,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"Response: {response.json()}")
```

### 3.3 JavaScript 예제 (브라우저/Node.js)

```javascript
const bip39 = require('bip39');
const { HDKey } = require('@scure/bip32');
const { keccak256 } = require('js-sha3');
const secp256k1 = require('secp256k1');

// 1. 니모닉 생성
const mnemonic = bip39.generateMnemonic();
console.log('Mnemonic:', mnemonic);

// 2. 시드 생성
const seed = bip39.mnemonicToSeedSync(mnemonic);

// 3. HD 키 유도 (BIP-44: m/44'/60'/0'/0/0)
const hdkey = HDKey.fromMasterSeed(seed);
const path = "m/44'/60'/0'/0/0";
const child = hdkey.derive(path);
const privateKey = child.privateKey;
const publicKey = secp256k1.publicKeyCreate(privateKey, false);

// 4. 주소 생성
const hash = keccak256(publicKey.slice(1));
const address = '0x' + Buffer.from(hash.slice(-20)).toString('hex');
console.log('Address:', address);

// 5. UTXO 조회
async function sendTransaction() {
    const utxoResponse = await fetch(`http://localhost:8000/api/v1/address/${address}/utxo`);
    const utxoData = await utxoResponse.json();
    const utxos = utxoData.data.utxos;
    
    if (utxos.length === 0) {
        console.log('No UTXOs available');
        return;
    }
    
    const utxo = utxos[0];
    const recipient = '0x9876543210fedcba9876543210fedcba98765432';
    const amount = 1000;
    
    // 6. 트랜잭션 해시 생성
    const txData = JSON.stringify({
        txId: utxo.txId,
        outputIndex: utxo.outputIndex,
        to: recipient,
        amount: amount
    });
    const txHash = Buffer.from(keccak256(txData), 'hex');
    
    // 7. 서명 생성
    const signature = secp256k1.ecdsaSign(txHash, privateKey);
    const signatureBytes = secp256k1.signatureExport(signature.signature);
    
    // 8. 서명된 트랜잭션 전송
    const signedTx = {
        inputs: [{
            txId: utxo.txId,
            outputIndex: utxo.outputIndex,
            signature: '0x' + Buffer.from(signatureBytes).toString('hex'),
            publicKey: '0x' + Buffer.from(publicKey).toString('hex')
        }],
        outputs: [{
            to: recipient,
            amount: amount
        }, {
            to: address,
            amount: utxo.amount - amount - 100  // 잔액 - 수수료
        }],
        memo: 'Payment from JavaScript client',
        data: null,
        txType: 0
    };
    
    const response = await fetch('http://localhost:8000/api/v1/tx/signed', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(signedTx)
    });
    
    const result = await response.json();
    console.log('Response:', result);
}

sendTransaction();
```

### 3.4 브라우저 지갑 예제 (간단한 UI)

```html
<!DOCTYPE html>
<html>
<head>
    <title>ABCFe Wallet</title>
    <script src="https://cdn.jsdelivr.net/npm/bip39@3.0.4/dist/bip39.min.js"></script>
</head>
<body>
    <h1>ABCFe Simple Wallet</h1>
    
    <div>
        <h2>1. Create Wallet</h2>
        <button onclick="createWallet()">Generate Mnemonic</button>
        <p id="mnemonic"></p>
        <p id="address"></p>
    </div>
    
    <div>
        <h2>2. Check Balance</h2>
        <button onclick="checkBalance()">Check Balance</button>
        <p id="balance"></p>
    </div>
    
    <div>
        <h2>3. Send Transaction</h2>
        <input type="text" id="recipient" placeholder="Recipient address" />
        <input type="number" id="amount" placeholder="Amount" />
        <button onclick="sendTx()">Send</button>
        <p id="result"></p>
    </div>
    
    <script>
        let privateKey, publicKey, myAddress;
        
        function createWallet() {
            const mnemonic = bip39.generateMnemonic();
            document.getElementById('mnemonic').textContent = 'Mnemonic: ' + mnemonic;
            
            // 여기서는 간단히 시드의 처음 32바이트를 개인키로 사용
            // 실제로는 BIP-32/44 HD 키 유도 사용 권장
            const seed = bip39.mnemonicToSeedSync(mnemonic);
            privateKey = seed.slice(0, 32);
            
            // 주소 생성 (실제 구현 필요)
            myAddress = '0x' + Array.from(privateKey.slice(0, 20))
                .map(b => b.toString(16).padStart(2, '0')).join('');
            
            document.getElementById('address').textContent = 'Address: ' + myAddress;
            
            // 보안 주의: 니모닉을 안전하게 저장하세요!
            localStorage.setItem('mnemonic', mnemonic);
        }
        
        async function checkBalance() {
            if (!myAddress) {
                alert('Create wallet first!');
                return;
            }
            
            const response = await fetch(`http://localhost:8000/api/v1/address/${myAddress}/balance`);
            const data = await response.json();
            document.getElementById('balance').textContent = 
                'Balance: ' + data.data.balance + ' coins';
        }
        
        async function sendTx() {
            if (!myAddress) {
                alert('Create wallet first!');
                return;
            }
            
            const recipient = document.getElementById('recipient').value;
            const amount = parseInt(document.getElementById('amount').value);
            
            // 실제로는 트랜잭션 서명 후 POST /api/v1/tx/signed로 전송
            // 위의 JavaScript 예제 참고
            
            alert('서명 및 전송 로직 구현 필요 (위의 예제 참고)');
        }
    </script>
</body>
</html>
```

### 3.5 보안 주의사항

⚠️ **중요**: 클라이언트 사이드 지갑 관리 시 주의사항

1. **니모닉/개인키 보관**
   - 절대 서버로 전송하지 마세요
   - 브라우저 localStorage 사용 시 XSS 공격 주의
   - 가능하면 암호화하여 저장
   - 하드웨어 지갑 사용 권장

2. **HTTPS 사용**
   - API 통신은 반드시 HTTPS 사용
   - Man-in-the-middle 공격 방지

3. **서명 검증**
   - 트랜잭션 서명 전 내용을 사용자에게 명확히 표시
   - 피싱 공격 주의

4. **의존성 보안**
   - npm 패키지 사용 시 신뢰할 수 있는 패키지만 사용
   - 정기적인 보안 업데이트

---

## 4. 트랜잭션 전송

### 4.1 방법 1: 지갑을 통한 전송 (간편) - 노드 운영자용

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

### 4.2 방법 2: 서명된 트랜잭션 제출 (일반 유저용)

일반 유저는 클라이언트 사이드에서 트랜잭션을 서명한 후 제출합니다.

> 💡 **완전한 예제 코드는 [섹션 3](#3-일반-유저의-지갑-관리-클라이언트-사이드)을 참고하세요.**

**API 엔드포인트**: `POST /api/v1/tx/signed`

**서명 프로세스**:
1. **UTXO 조회**: `GET /api/v1/address/{address}/utxo`
2. **트랜잭션 구성**: inputs + outputs
3. **서명 생성**: 각 input에 대해 ECDSA 서명 (클라이언트에서)
4. **트랜잭션 제출**: 서명된 트랜잭션을 API로 전송

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

**중요**: 
- 일반 유저는 노드에 개인키를 노출하지 않습니다
- 모든 서명은 클라이언트 사이드에서 수행됩니다
- Python/JavaScript 예제 코드는 [섹션 3.2](#32-python-예제-완전한-워크플로우), [3.3](#33-javascript-예제-브라우저nodejs) 참고

---

## 5. REST API 사용

모든 API는 `http://localhost:8000/api/v1` 경로를 사용합니다.

### 5.1 노드 상태 조회

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

### 5.2 블록 조회

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

### 5.3 트랜잭션 조회

```bash
curl http://localhost:8000/api/v1/tx/0xabcd1234...
```

### 5.4 주소 관련 조회

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

### 5.5 멤풀 조회

```bash
curl http://localhost:8000/api/v1/mempool/list
```

### 5.6 컨센서스 상태 조회

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

### 5.7 네트워크 통계

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

### 5.8 지갑 계정 조회 (API)

```bash
curl http://localhost:8000/api/v1/wallet/accounts
```

### 5.9 새 계정 생성 (API)

```bash
curl -X POST http://localhost:8000/api/v1/wallet/account/new
```

---

## 6. WebSocket 실시간 알림

WebSocket을 통해 블록체인 이벤트를 실시간으로 수신할 수 있습니다.

### 6.1 연결

**WebSocket URL**: `ws://localhost:8000/ws`

### 6.2 이벤트 타입

1. **new_block**: 새 블록이 생성되었을 때
2. **new_transaction**: 새 트랜잭션이 멤풀에 추가되었을 때
3. **block_confirmed**: 블록이 확정되었을 때
4. **consensus_state_change**: 컨센서스 상태가 변경되었을 때

### 6.3 JavaScript 예제

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

### 6.4 이벤트 데이터 예시

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

### 6.5 Python 예제

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

## 8. 관리 스크립트

ABCFe는 멀티 노드 환경을 쉽게 관리할 수 있는 스크립트를 제공합니다.

### 8.1 전체 자동 셋업

```bash
# 한 번에 모든 것을 셋업 (가장 추천!)
./setup_multi_nodes.sh 3

# 실행 내용:
# 1. 기존 노드 중지
# 2. DB 초기화 (선택적)
# 3. 지갑 생성 (3개)
# 4. 제네시스 블록 셋업
# 5. 노드 시작 (3개)
# 6. 상태 확인
```

### 8.2 개별 스크립트

#### 지갑 생성
```bash
./create_wallets.sh 3

# Node 1: ./resource/wallet/wallet.json
# Node 2: ./resource/wallet2/wallet.json
# Node 3: ./resource/wallet3/wallet.json
```

#### 제네시스 블록 셋업
```bash
./setup_genesis.sh 3

# 실행 내용:
# 1. Boot 노드(Node 1)에서 제네시스 블록 생성
# 2. 다른 노드들에게 제네시스 블록 복사
# 3. 모든 노드가 동일한 체인에서 시작하도록 보장
```

#### 노드 시작
```bash
./start_multi_nodes.sh 3

# Node 1: Port 30303, REST 8000 (Boot/Producer)
# Node 2: Port 30304, REST 8001 (Validator/Sync)
# Node 3: Port 30305, REST 8002 (Validator/Sync)
```

#### 상태 확인
```bash
./check_nodes.sh

# 출력 예시:
# Node 1 (REST: 8000): ✓ 실행 중 (Height: 567)
# Node 2 (REST: 8001): ✓ 실행 중 (Height: 567)
# Node 3 (REST: 8002): ✓ 실행 중 (Height: 567)
# ✓ 모든 노드가 동기화되었습니다 (Height: 567)
```

#### 노드 중지
```bash
./stop_all_nodes.sh

# 모든 abcfed 프로세스 종료
```

#### 데이터 정리
```bash
./clean_all.sh

# DB 및 로그 삭제 (지갑은 유지)
```

### 8.3 사용 시나리오

#### 처음 시작
```bash
./setup_multi_nodes.sh 3
```

#### 노드 재시작
```bash
./stop_all_nodes.sh
./start_multi_nodes.sh 3
```

#### 완전 초기화
```bash
./clean_all.sh
./setup_multi_nodes.sh 3
```

#### 노드 추가 (2개 → 3개)
```bash
# 새 지갑 생성
./abcfed wallet create --wallet-dir=./resource/wallet3

# 제네시스 블록 복사
./setup_genesis.sh 3

# 모든 노드 재시작
./stop_all_nodes.sh
./start_multi_nodes.sh 3
```

**자세한 내용은 [README_SCRIPTS.md](README_SCRIPTS.md)를 참고하세요.**

---

## 7. 멀티 노드 환경

### 7.1 두 번째 노드 설정

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

### 7.2 두 번째 노드 실행

```bash
./abcfed --config config/config_node2.toml
```

### 7.3 멀티 노드 테스트 스크립트

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

### 7.4 노드 간 동기화 확인

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

## 8. 실전 시나리오

### 8.1 시나리오: Genesis → User1 → User2 코인 전송

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

## 9. 문제 해결

### 9.1 노드가 시작되지 않음

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

### 9.2 트랜잭션이 실패함

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

### 9.3 노드 간 동기화 안 됨

**증상**: 두 노드의 블록 높이가 다름

**해결**:
1. P2P 연결 확인 (향후 구현)
2. 두 노드 재시작
3. 제네시스 블록 일치 여부 확인

### 9.4 WebSocket 연결 실패

**증상**: `ws.onerror` 이벤트 발생

**해결**:
1. 노드가 실행 중인지 확인
2. 올바른 포트 사용 확인 (`config.toml`의 `RestPort`)
3. CORS 설정 확인 (크로스 도메인의 경우)

---

## 10. 추가 자료

### 문서
- **[README.md](README.md)** - 프로젝트 개요
- **[QUICK_START.md](QUICK_START.md)** - 1분 빠른 시작 가이드
- **[README_SCRIPTS.md](README_SCRIPTS.md)** - 스크립트 상세 가이드
- **[CLAUDE.md](CLAUDE.md)** - 개발자용 아키텍처 가이드

### 설정 파일
- **config/config.toml**: 메인 노드 설정
- **config/dev.config.toml**: 개발 환경 설정
- **config/prod.config.toml**: 프로덕션 환경 설정

### 빌드
- **Makefile**: 빌드 및 테스트 명령어

---

## 11. API 레퍼런스 요약

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

