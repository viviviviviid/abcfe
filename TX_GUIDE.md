# ABCFe 트랜잭션 가이드

> **마지막 업데이트: 2025-12-28**

이 문서는 ABCFe 블록체인에서 트랜잭션을 생성하고 서명하는 방법을 상세히 설명합니다.

## 목차

1. [개요](#1-개요)
2. [암호화 스펙 (필독!)](#2-암호화-스펙-필독)
3. [TX ID 계산 방법](#3-tx-id-계산-방법)
4. [JSON 인코딩 규칙](#4-json-인코딩-규칙)
5. [Python 예제](#5-python-예제)
6. [JavaScript 예제](#6-javascript-예제)
7. [Go 예제](#7-go-예제)
8. [다중 Output 트랜잭션](#8-다중-output-트랜잭션)
9. [주의사항 및 트러블슈팅](#9-주의사항-및-트러블슈팅)

---

## 1. 개요

ABCFe에서 트랜잭션을 전송하는 방법은 두 가지입니다:

### 1.1 노드 운영자용 (서버 서명)

노드가 지갑을 관리하고 자동으로 서명합니다.

```bash
POST /api/v1/tx/send
```

### 1.2 일반 유저용 (클라이언트 서명)

클라이언트에서 직접 서명한 트랜잭션을 제출합니다.

```bash
POST /api/v1/tx/signed
```

**이 문서는 주로 클라이언트 서명 방식을 다룹니다.**

---

## 2. 암호화 스펙 (필독!)

> **이 섹션을 반드시 먼저 읽으세요!** 서버의 암호화 스펙과 일치하지 않으면 서명 검증이 실패합니다.

| 항목 | 값 | 설명 |
|------|-----|------|
| **타원 곡선** | **P-256 (secp256r1/prime256v1)** | secp256k1이 **아님**! |
| **서명 알고리즘** | ECDSA | 표준 ECDSA |
| **서명 포맷** | **ASN.1 DER** | raw (r\|\|s) 64바이트가 **아님** |
| **공개키 포맷** | **PKIX/X.509 SubjectPublicKeyInfo** | 단순 바이트가 **아님** |
| **해시 함수 (TX ID)** | SHA256 | JSON 직렬화 후 해시 |
| **주소 생성** | Keccak256(압축 공개키[1:])의 마지막 20바이트 | Ethereum과 유사 |
| **서명 대상** | TX ID 바이트 (32바이트) | 다시 해시하지 **않음** |

### 서버 암호화 코드 요약

```go
// 곡선: P-256 (secp256r1)
elliptic.P256()

// 공개키 포맷: PKIX/X.509
x509.MarshalPKIXPublicKey(publicKey)
x509.ParsePKIXPublicKey(publicKeyBytes)

// 서명 생성/검증: ASN.1 DER
ecdsa.SignASN1(rand.Reader, privateKey, data)
ecdsa.VerifyASN1(publicKey, data, signature)

// 주소 생성: Keccak256(압축 공개키[1:])[-20:]
hash := sha3.NewLegacyKeccak256()
hash.Write(compressedPubKey[1:])  // prefix 제거
address = hash.Sum(nil)[len-20:]
```

---

## 3. TX ID 계산 방법

TX ID는 트랜잭션 JSON을 SHA256 해시한 값입니다.

### 3.1 핵심 규칙

1. **networkId**: `"abcfe-mainnet"` 사용 (서버 설정과 일치)
2. **id**: 32개의 0으로 채운 배열 `[0,0,...,0]`
3. **signature**: 72개의 0으로 채운 배열 `[0,0,...,0]`
4. **JSON 키 순서**: Go 구조체 필드 정의 순서대로

### 3.2 Transaction 구조체 (Go)

```go
type Transaction struct {
    Version   string       `json:"version"`
    NetworkID string       `json:"networkId"`
    ID        [32]byte     `json:"id"`        // → 숫자 배열
    Timestamp int64        `json:"timestamp"`
    Inputs    []*TxInput   `json:"inputs"`
    Outputs   []*TxOutput  `json:"outputs"`
    Memo      string       `json:"memo"`
    Data      []byte       `json:"data"`       // → Base64 문자열
}

type TxInput struct {
    TxID        [32]byte   `json:"txId"`        // → 숫자 배열
    OutputIndex uint64     `json:"outputIndex"`
    Signature   [72]byte   `json:"signature"`   // → 숫자 배열 (72개 zero)
    PublicKey   []byte     `json:"publicKey"`   // → Base64 문자열!
}

type TxOutput struct {
    Address [20]byte `json:"address"`  // → 숫자 배열
    Amount  uint64   `json:"amount"`
    TxType  uint8    `json:"txType"`
}
```

### 3.3 필드 순서 (중요!)

**Transaction 필드 순서:**
```
version → networkId → id → timestamp → inputs → outputs → memo → data
```

**TxInput 필드 순서:**
```
txId → outputIndex → signature → publicKey
```

**TxOutput 필드 순서:**
```
address → amount → txType
```

---

## 4. JSON 인코딩 규칙

Go의 JSON 직렬화에서 **타입에 따라 인코딩 방식이 다릅니다**:

| Go 타입 | JSON 인코딩 | 예시 |
|---------|-------------|------|
| `[]byte` (슬라이스) | **Base64 문자열** | `"MFkwEwYH..."` |
| `[32]byte` (고정 배열) | **숫자 배열** | `[0,0,0,...,0]` |
| `[72]byte` (고정 배열) | **숫자 배열** | `[0,0,0,...,0]` |
| `[20]byte` (고정 배열) | **숫자 배열** | `[152,118,84,...]` |

### 4.1 TX ID 계산용 JSON 예시

```json
{"version":"1.0.0","networkId":"abcfe-mainnet","id":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamp":1735344000,"inputs":[{"txId":[161,178,195,212,229,246,161,178,195,212,229,246,161,178,195,212,229,246,161,178,195,212,229,246,161,178,195,212,229,246,161,178],"outputIndex":0,"signature":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"publicKey":"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE..."}],"outputs":[{"address":[152,118,84,50,16,254,220,186,152,118,84,50,16,254,220,186,152,118,84,50],"amount":1000,"txType":0}],"memo":"test payment","data":""}
```

### 4.2 보기 좋게 정렬 (참고용)

```json
{
  "version": "1.0.0",
  "networkId": "abcfe-mainnet",
  "id": [0,0,0,...,0],
  "timestamp": 1735344000,
  "inputs": [{
    "txId": [161,178,195,...],
    "outputIndex": 0,
    "signature": [0,0,0,...,0],
    "publicKey": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE..."
  }],
  "outputs": [{
    "address": [152,118,84,...],
    "amount": 1000,
    "txType": 0
  }],
  "memo": "test payment",
  "data": ""
}
```

### 4.3 필드별 인코딩 정리

| 필드 | Go 타입 | JSON 인코딩 |
|------|---------|-------------|
| `id` | `[32]byte` | 32개의 숫자 배열: `[0,0,...,0]` |
| `txId` | `[32]byte` | 32개의 숫자 배열: `[161,178,...]` |
| `signature` | `[72]byte` | 72개의 숫자 배열: `[0,0,...,0]` |
| `address` | `[20]byte` | 20개의 숫자 배열: `[152,118,...]` |
| `publicKey` | `[]byte` | **Base64 문자열**: `"MFkwEwYH..."` |
| `data` | `[]byte` | **Base64 문자열** (빈 경우 `""`) |

---

## 5. Python 예제

### 5.1 필요한 패키지

```bash
pip install cryptography pycryptodome requests
```

### 5.2 완전한 워크플로우

```python
import json
import time
import hashlib
import base64
import requests
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import Prehashed
from cryptography.hazmat.backends import default_backend
from Crypto.Hash import keccak  # pycryptodome

# === 1. 키 쌍 생성 (P-256 곡선!) ===
private_key = ec.generate_private_key(ec.SECP256R1(), default_backend())
public_key = private_key.public_key()

# === 2. 공개키를 PKIX 포맷으로 인코딩 ===
public_key_pkix = public_key.public_bytes(
    encoding=serialization.Encoding.DER,
    format=serialization.PublicFormat.SubjectPublicKeyInfo
)
print(f"Public Key (PKIX DER hex): {public_key_pkix.hex()}")

# === 3. 주소 생성 (서버와 동일한 방식) ===
compressed_pubkey = public_key.public_bytes(
    encoding=serialization.Encoding.X962,
    format=serialization.PublicFormat.CompressedPoint
)
# [1:] = prefix (0x02 or 0x03) 제거
k = keccak.new(digest_bits=256)
k.update(compressed_pubkey[1:])
address = k.digest()[-20:].hex()  # 마지막 20바이트
print(f"Address: {address}")

# === 4. UTXO 조회 ===
response = requests.get(f"http://localhost:8000/api/v1/address/{address}/utxo")
utxos = response.json()["data"]["utxos"]
print(f"Available UTXOs: {len(utxos)}")

if not utxos:
    print("No UTXOs available")
    exit()

# === 5. 트랜잭션 구성 (서명 없이 - TX ID 계산용) ===
recipient = "9876543210fedcba9876543210fedcba98765432"  # 0x 없이 40자
amount = 1000
fee = 100
utxo = utxos[0]
change = utxo["amount"] - amount - fee
timestamp = int(time.time())

# hex 문자열을 숫자 배열로 변환하는 함수
def hex_to_int_array(hex_str):
    return list(bytes.fromhex(hex_str))

# 공개키를 Base64로 변환 ([]byte는 Base64로 인코딩됨!)
public_key_base64 = base64.b64encode(public_key_pkix).decode('ascii')

# 서버의 Transaction 구조체와 일치하는 JSON 구성
# 중요: 필드 순서가 Go 구조체와 일치해야 함!
tx_for_hash = {
    "version": "1.0.0",
    "networkId": "abcfe-mainnet",  # 서버 설정과 일치!
    "id": [0] * 32,   # 빈 해시 (32 바이트 zero)
    "timestamp": timestamp,
    "inputs": [{
        "txId": hex_to_int_array(utxo["txId"]),
        "outputIndex": utxo["outputIndex"],
        "signature": [0] * 72,  # 빈 서명
        "publicKey": public_key_base64  # Base64 문자열!
    }],
    "outputs": [
        {"address": hex_to_int_array(recipient), "amount": amount, "txType": 0},
        {"address": hex_to_int_array(address), "amount": change, "txType": 0}
    ],
    "memo": "Payment from Python client",
    "data": ""  # []byte{} → "" (빈 문자열)
}

# === 6. TX ID 계산 (JSON 직렬화 후 SHA256) ===
tx_json = json.dumps(tx_for_hash, separators=(',', ':'), sort_keys=False)
print(f"TX JSON for hash: {tx_json[:200]}...")
tx_id = hashlib.sha256(tx_json.encode()).digest()
print(f"Calculated TX ID: {tx_id.hex()}")

# === 7. TX ID에 서명 (ASN.1 DER 포맷) ===
# 중요: tx_id를 다시 해시하지 않고 직접 서명!
signature = private_key.sign(
    tx_id,
    ec.ECDSA(Prehashed(hashes.SHA256()))
)
print(f"Signature (DER hex): {signature.hex()}")

# === 8. 서명된 트랜잭션 전송 ===
# API 요청 형식은 hex string 사용
signed_tx = {
    "version": "1.0.0",
    "timestamp": timestamp,
    "inputs": [{
        "txId": utxo["txId"],                # hex string (0x 없이)
        "outputIndex": utxo["outputIndex"],
        "signature": signature.hex(),         # hex string (0x 없이)
        "publicKey": public_key_pkix.hex()   # hex string (0x 없이) - PKIX 포맷!
    }],
    "outputs": [
        {"address": recipient, "amount": amount, "txType": 0},
        {"address": address, "amount": change, "txType": 0}
    ],
    "memo": "Payment from Python client",
    "data": []
}

response = requests.post(
    "http://localhost:8000/api/v1/tx/signed",
    json=signed_tx,
    headers={"Content-Type": "application/json"}
)

print(f"Response: {response.json()}")
```

### 5.3 서명 플로우 요약

1. **P-256 곡선**으로 키 쌍 생성
2. 공개키를 **PKIX DER 포맷**으로 인코딩
3. 트랜잭션 객체를 서명 없이 구성 (signature = 72 zero bytes)
4. JSON 직렬화 후 SHA256 해시 → **tx.ID**
5. tx.ID를 **직접 서명** (다시 해시하지 않음)
6. 서명은 **ASN.1 DER 포맷**으로 생성됨
7. 서명을 포함한 트랜잭션을 서버에 전송

---

## 6. JavaScript 예제

### 6.1 필요한 패키지

```bash
npm install js-sha3
```

### 6.2 Node.js 완전한 워크플로우

```javascript
const crypto = require('crypto');
const { createHash } = require('crypto');
const { keccak256 } = require('js-sha3');

// === 1. 키 쌍 생성 (P-256 곡선!) ===
const { privateKey, publicKey } = crypto.generateKeyPairSync('ec', {
    namedCurve: 'prime256v1'  // P-256 = secp256r1 = prime256v1
});

// === 2. 공개키를 SPKI DER 포맷으로 추출 ===
const publicKeyDer = publicKey.export({
    type: 'spki',
    format: 'der'
});
console.log('Public Key (SPKI DER hex):', publicKeyDer.toString('hex'));

// === 3. 주소 생성 (서버와 동일한 방식) ===
// SPKI에서 실제 공개키 좌표 추출
const pubKeyUncompressed = publicKeyDer.slice(-65);  // 마지막 65바이트
const x = pubKeyUncompressed.slice(1, 33);
const y = pubKeyUncompressed.slice(33);

// 압축된 공개키 생성 (02 or 03 prefix)
const prefix = y[31] % 2 === 0 ? 0x02 : 0x03;
const compressedPubKey = Buffer.concat([Buffer.from([prefix]), x]);

// Keccak256 해시 (prefix 제외)
const addressHash = keccak256(compressedPubKey.slice(1));
const address = addressHash.slice(-40);  // 마지막 20바이트 = 40 hex chars
console.log('Address:', address);

// === 4. 트랜잭션 전송 함수 ===
async function sendTransaction() {
    // UTXO 조회
    const utxoResponse = await fetch(`http://localhost:8000/api/v1/address/${address}/utxo`);
    const utxoData = await utxoResponse.json();
    const utxos = utxoData.data.utxos;

    if (utxos.length === 0) {
        console.log('No UTXOs available');
        return;
    }

    const utxo = utxos[0];
    const recipient = '9876543210fedcba9876543210fedcba98765432';
    const amount = 1000;
    const fee = 100;
    const change = utxo.amount - amount - fee;
    const timestamp = Math.floor(Date.now() / 1000);

    // === 5. 트랜잭션 구성 (서명 없이 - TX ID 계산용) ===
    // hex를 숫자 배열로 변환
    const hexToIntArray = (hex) => Array.from(Buffer.from(hex, 'hex'));

    // 공개키를 Base64로 변환 ([]byte는 Base64 문자열!)
    const publicKeyBase64 = publicKeyDer.toString('base64');

    // 중요: 필드 순서가 Go 구조체와 일치해야 함!
    const txForHash = {
        version: "1.0.0",
        networkId: "abcfe-mainnet",  // 서버 설정과 일치!
        id: new Array(32).fill(0),
        timestamp: timestamp,
        inputs: [{
            txId: hexToIntArray(utxo.txId),
            outputIndex: utxo.outputIndex,
            signature: new Array(72).fill(0),
            publicKey: publicKeyBase64  // Base64 문자열!
        }],
        outputs: [
            { address: hexToIntArray(recipient), amount: amount, txType: 0 },
            { address: hexToIntArray(address), amount: change, txType: 0 }
        ],
        memo: "Payment from JavaScript client",
        data: ""  // []byte{} → "" (빈 문자열)
    };

    // === 6. TX ID 계산 ===
    const txJson = JSON.stringify(txForHash);
    console.log('TX JSON for hash:', txJson.substring(0, 200) + '...');
    const txId = createHash('sha256').update(txJson).digest();
    console.log('Calculated TX ID:', txId.toString('hex'));

    // === 7. TX ID에 서명 (DER 포맷) ===
    // 이미 해시된 값에 직접 서명
    const signature = crypto.sign(null, txId, {
        key: privateKey,
        dsaEncoding: 'der'
    });
    console.log('Signature (DER hex):', signature.toString('hex'));

    // === 8. 서명된 트랜잭션 전송 ===
    const signedTx = {
        version: "1.0.0",
        timestamp: timestamp,
        inputs: [{
            txId: utxo.txId,
            outputIndex: utxo.outputIndex,
            signature: signature.toString('hex'),
            publicKey: publicKeyDer.toString('hex')  // SPKI DER 포맷!
        }],
        outputs: [
            { address: recipient, amount: amount, txType: 0 },
            { address: address, amount: change, txType: 0 }
        ],
        memo: "Payment from JavaScript client",
        data: []
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

### 6.3 서명 플로우 요약

1. **P-256 (prime256v1)** 곡선으로 키 쌍 생성
2. 공개키를 **SPKI DER 포맷**으로 추출
3. 트랜잭션 객체를 서명 없이 구성 (signature = 72 zero bytes)
4. JSON 직렬화 후 SHA256 해시 → **tx.ID**
5. tx.ID를 **직접 서명** (다시 해시하지 않음)
6. 서명은 **DER 포맷**으로 생성됨
7. 서명을 포함한 트랜잭션을 서버에 전송

---

## 7. Go 예제

서버와 동일한 라이브러리를 사용하므로 가장 호환성이 좋습니다.

### 7.1 필요한 의존성

```bash
go get golang.org/x/crypto/sha3
```

### 7.2 완전한 워크플로우

```go
package main

import (
    "bytes"
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/sha256"
    "crypto/x509"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "golang.org/x/crypto/sha3"
)

// 서버의 타입 정의와 일치
type Hash [32]byte
type Address [20]byte
type Signature [72]byte

type Transaction struct {
    Version   string      `json:"version"`
    NetworkID string      `json:"networkId"`
    ID        Hash        `json:"id"`
    Timestamp int64       `json:"timestamp"`
    Inputs    []*TxInput  `json:"inputs"`
    Outputs   []*TxOutput `json:"outputs"`
    Memo      string      `json:"memo"`
    Data      []byte      `json:"data"`
}

type TxInput struct {
    TxID        Hash      `json:"txId"`
    OutputIndex uint64    `json:"outputIndex"`
    Signature   Signature `json:"signature"`
    PublicKey   []byte    `json:"publicKey"`
}

type TxOutput struct {
    Address Address `json:"address"`
    Amount  uint64  `json:"amount"`
    TxType  uint8   `json:"txType"`
}

// API 요청/응답 타입
type SubmitSignedTxReq struct {
    Version   string          `json:"version"`
    Timestamp int64           `json:"timestamp"`
    Inputs    []SignedTxInput `json:"inputs"`
    Outputs   []TxOutputReq   `json:"outputs"`
    Memo      string          `json:"memo"`
    Data      []byte          `json:"data"`
}

type SignedTxInput struct {
    TxID        string `json:"txId"`
    OutputIndex uint64 `json:"outputIndex"`
    Signature   string `json:"signature"`
    PublicKey   string `json:"publicKey"`
}

type TxOutputReq struct {
    Address string `json:"address"`
    Amount  uint64 `json:"amount"`
    TxType  uint8  `json:"txType"`
}

type UTXOResp struct {
    TxId        string `json:"txId"`
    OutputIndex uint64 `json:"outputIndex"`
    Amount      uint64 `json:"amount"`
    Address     string `json:"address"`
    Height      uint64 `json:"height"`
}

type APIResponse struct {
    Success bool `json:"success"`
    Data    struct {
        UTXOs []UTXOResp `json:"utxos"`
        TxId  string     `json:"txId"`
    } `json:"data"`
    Error string `json:"error"`
}

func main() {
    // === 1. 키 쌍 생성 (P-256 곡선) ===
    privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        panic(err)
    }
    publicKey := &privateKey.PublicKey

    // === 2. 공개키를 PKIX 포맷으로 인코딩 ===
    publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Public Key (PKIX hex): %s\n", hex.EncodeToString(publicKeyBytes))

    // === 3. 주소 생성 ===
    address := publicKeyToAddress(publicKey)
    addressStr := hex.EncodeToString(address[:])
    fmt.Printf("Address: %s\n", addressStr)

    // === 4. UTXO 조회 ===
    utxos, err := getUTXOs(addressStr)
    if err != nil {
        fmt.Printf("Failed to get UTXOs: %v\n", err)
        return
    }
    if len(utxos) == 0 {
        fmt.Println("No UTXOs available")
        return
    }

    utxo := utxos[0]
    fmt.Printf("Using UTXO: %s:%d (amount: %d)\n", utxo.TxId, utxo.OutputIndex, utxo.Amount)

    // === 5. 트랜잭션 구성 ===
    recipient := "9876543210fedcba9876543210fedcba98765432"
    amount := uint64(1000)
    fee := uint64(100)
    change := utxo.Amount - amount - fee
    timestamp := time.Now().Unix()

    // UTXO의 txId를 Hash 타입으로 변환
    utxoTxIdBytes, _ := hex.DecodeString(utxo.TxId)
    var utxoTxId Hash
    copy(utxoTxId[:], utxoTxIdBytes)

    // recipient와 sender 주소를 Address 타입으로 변환
    recipientBytes, _ := hex.DecodeString(recipient)
    var recipientAddr Address
    copy(recipientAddr[:], recipientBytes)

    // 트랜잭션 객체 (서명 없이 - TX ID 계산용)
    tx := &Transaction{
        Version:   "1.0.0",
        NetworkID: "abcfe-mainnet",  // 서버 설정과 일치
        ID:        Hash{},
        Timestamp: timestamp,
        Inputs: []*TxInput{{
            TxID:        utxoTxId,
            OutputIndex: utxo.OutputIndex,
            Signature:   Signature{},  // 빈 서명 (72 zero bytes)
            PublicKey:   publicKeyBytes,
        }},
        Outputs: []*TxOutput{
            {Address: recipientAddr, Amount: amount, TxType: 0},
            {Address: address, Amount: change, TxType: 0},
        },
        Memo: "Payment from Go client",
        Data: []byte{},  // null 대신 빈 슬라이스
    }

    // === 6. TX ID 계산 ===
    txJSON, err := json.Marshal(tx)
    if err != nil {
        panic(err)
    }
    txIdHash := sha256.Sum256(txJSON)
    tx.ID = txIdHash
    fmt.Printf("Calculated TX ID: %s\n", hex.EncodeToString(txIdHash[:]))

    // === 7. TX ID에 서명 (ASN.1 DER) ===
    signature, err := ecdsa.SignASN1(rand.Reader, privateKey, txIdHash[:])
    if err != nil {
        panic(err)
    }
    fmt.Printf("Signature (DER hex): %s\n", hex.EncodeToString(signature))

    // 서명을 Signature 타입에 복사
    var sig Signature
    copy(sig[:], signature)
    tx.Inputs[0].Signature = sig

    // === 8. API로 전송 ===
    req := SubmitSignedTxReq{
        Version:   tx.Version,
        Timestamp: tx.Timestamp,
        Inputs: []SignedTxInput{{
            TxID:        utxo.TxId,
            OutputIndex: utxo.OutputIndex,
            Signature:   hex.EncodeToString(signature),
            PublicKey:   hex.EncodeToString(publicKeyBytes),
        }},
        Outputs: []TxOutputReq{
            {Address: recipient, Amount: amount, TxType: 0},
            {Address: addressStr, Amount: change, TxType: 0},
        },
        Memo: tx.Memo,
        Data: tx.Data,
    }

    resp, err := submitSignedTx(req)
    if err != nil {
        fmt.Printf("Failed to submit tx: %v\n", err)
        return
    }
    fmt.Printf("Response: %+v\n", resp)
}

// 공개키에서 주소 생성 (서버와 동일한 방식)
func publicKeyToAddress(publicKey *ecdsa.PublicKey) Address {
    // 압축된 공개키 생성
    compressed := elliptic.MarshalCompressed(publicKey.Curve, publicKey.X, publicKey.Y)

    // Keccak256 해시 (prefix 제거)
    hash := sha3.NewLegacyKeccak256()
    hash.Write(compressed[1:])
    hashBytes := hash.Sum(nil)

    // 마지막 20바이트
    var address Address
    copy(address[:], hashBytes[len(hashBytes)-20:])
    return address
}

// UTXO 조회
func getUTXOs(address string) ([]UTXOResp, error) {
    resp, err := http.Get(fmt.Sprintf("http://localhost:8000/api/v1/address/%s/utxo", address))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    var apiResp APIResponse
    if err := json.Unmarshal(body, &apiResp); err != nil {
        return nil, err
    }
    return apiResp.Data.UTXOs, nil
}

// 서명된 트랜잭션 제출
func submitSignedTx(req SubmitSignedTxReq) (*APIResponse, error) {
    reqBody, _ := json.Marshal(req)
    resp, err := http.Post(
        "http://localhost:8000/api/v1/tx/signed",
        "application/json",
        bytes.NewReader(reqBody),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    var apiResp APIResponse
    if err := json.Unmarshal(body, &apiResp); err != nil {
        return nil, err
    }
    return &apiResp, nil
}
```

### 7.3 Go 예제의 장점

- 서버와 동일한 라이브러리 사용으로 완벽한 호환성
- 타입 정의가 서버와 일치
- JSON 직렬화 결과가 서버와 동일

---

## 8. 다중 Output 트랜잭션

하나의 트랜잭션으로 **여러 수신자에게 동시에 전송**할 수 있습니다.

### 8.1 UTXO 모델 이해

```
Input(s)의 합계 = Output(s)의 합계 + 수수료

예시: 10,000 코인 UTXO로 3명에게 전송
├── Input: 10,000 (내 UTXO)
├── Output 1: 1,000 → 수신자 A
├── Output 2: 2,000 → 수신자 B
├── Output 3: 500 → 수신자 C
├── Output 4: 6,400 → 나 (잔액 반환)
└── 수수료: 100 (암묵적, Output에 포함 안 됨)
```

### 8.2 Python 예제 (다중 수신자)

```python
# 여러 수신자에게 전송
recipients = [
    {"address": "aaaa...aaaa", "amount": 1000},  # 수신자 A
    {"address": "bbbb...bbbb", "amount": 2000},  # 수신자 B
    {"address": "cccc...cccc", "amount": 500},   # 수신자 C
]

fee = 100
total_send = sum(r["amount"] for r in recipients)  # 3500
change = utxo["amount"] - total_send - fee         # 잔액

# outputs 구성
outputs = []

# 1. 수신자들에게 보내는 Output
for r in recipients:
    outputs.append({
        "address": r["address"],
        "amount": r["amount"],
        "txType": 0
    })

# 2. 잔액 반환 Output (반드시 마지막에 추가)
if change > 0:
    outputs.append({
        "address": my_address,
        "amount": change,
        "txType": 0
    })

# 서명된 트랜잭션 구성
signed_tx = {
    "version": "1.0.0",
    "timestamp": int(time.time()),
    "inputs": [{
        "txId": utxo["txId"],
        "outputIndex": utxo["outputIndex"],
        "signature": signature.hex(),
        "publicKey": public_key.hex()
    }],
    "outputs": outputs,  # 여러 Output
    "memo": "Multi-recipient payment",
    "data": []
}
```

### 8.3 주의사항

| 항목 | 설명 |
|------|------|
| **잔액 반환 필수** | `Input 합계 - Output 합계 = 수수료`가 되어야 함 |
| **Output 순서** | 순서는 자유이나, tx.ID 계산 시 동일한 순서 유지 필요 |
| **여러 Input** | UTXO가 부족하면 여러 Input 사용 가능 (각 Input에 서명 필요) |
| **수수료 계산** | 암묵적 수수료 = `Σ(Inputs) - Σ(Outputs)` |

### 8.4 여러 Input 사용 예시

```python
# 2개의 UTXO를 합쳐서 전송
inputs = []
total_input = 0

for utxo in my_utxos[:2]:  # 2개 UTXO 사용
    inputs.append({
        "txId": utxo["txId"],
        "outputIndex": utxo["outputIndex"],
        "signature": "",  # 각각 서명 필요
        "publicKey": public_key.hex()
    })
    total_input += utxo["amount"]

# 각 input에 대해 tx.ID로 서명
for i, inp in enumerate(inputs):
    inp["signature"] = signatures[i].hex()
```

---

## 9. 주의사항 및 트러블슈팅

### 9.1 서명 검증 실패의 주요 원인

| 문제 | 증상 | 해결 방법 |
|------|------|-----------|
| **잘못된 곡선** | `failed to parse public key` | **P-256 (secp256r1)** 사용, secp256k1 **아님** |
| **잘못된 공개키 포맷** | `failed to parse public key` | **PKIX/SPKI DER** 포맷 사용 |
| **잘못된 서명 포맷** | `invalid signature` | **ASN.1 DER** 포맷 사용, raw 64바이트 **아님** |
| **tx.ID 불일치** | `invalid signature` | 서버와 동일한 JSON 구조 사용 |
| **서명 데이터 불일치** | `invalid signature` | tx.ID 바이트를 **직접 서명** (다시 해시 **안함**) |
| **publicKey JSON 인코딩** | `tx hash mismatch` | **Base64 문자열**로 인코딩해야 함! |
| **고정 배열 인코딩** | `tx hash mismatch` | `[32]byte`, `[72]byte` 등은 **숫자 배열**로 |
| **networkId 불일치** | `network ID mismatch` | `"abcfe-mainnet"` 사용 (서버 설정 확인) |

### 9.2 디버깅 팁

1. 서버 로그에서 `[DEBUG] Server Recalculated ID:` 확인
2. 클라이언트에서 계산한 tx.ID와 서버 tx.ID 비교
3. JSON 직렬화 결과를 콘솔에 출력하여 비교
4. **publicKey가 Base64 문자열인지 확인** (가장 흔한 실수!)
5. **고정 배열들이 숫자 배열인지 확인**
6. **networkId가 서버 설정과 일치하는지 확인**

### 9.3 API 요청 vs TX ID 계산 차이

| 항목 | TX ID 계산용 JSON | API 요청 JSON |
|------|-------------------|---------------|
| **networkId** | `"abcfe-mainnet"` | 필드 없음 (서버가 자동 설정) |
| **id** | `[0,0,...,0]` (32개) | 필드 없음 |
| **signature** | `[0,0,...,0]` (72개) | hex 문자열 |
| **publicKey** | Base64 문자열 | hex 문자열 |
| **address** | 숫자 배열 | hex 문자열 |
| **txId** | 숫자 배열 | hex 문자열 |

### 9.4 보안 주의사항

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

## 관련 문서

- **[USER_GUIDE.md](USER_GUIDE.md)** - 전체 사용자 가이드
- **[CLAUDE.md](CLAUDE.md)** - 개발자용 아키텍처 가이드
- **[README.md](README.md)** - 프로젝트 개요
