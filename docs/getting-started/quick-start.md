# Quick Start

1분 안에 ABCFe 노드를 실행해봅니다.

## Prerequisites

- Go 1.21+
- Make

## Build

```bash
# 저장소 클론
git clone https://github.com/abcfe/abcfe-node.git
cd abcfe-node

# 빌드
make build
```

## Run Single Node

```bash
./abcfed
```

**Output:**
```
[INFO] Starting ABCFe Node...
[INFO] REST API listening on :8000
[INFO] P2P listening on :30303
[INFO] Consensus engine started at height 1
```

## Run Multi-Node (PoA)

```bash
# 3개 노드로 PoA 네트워크 시작
./start_poa.sh 3
```

**Output:**
```
╔══════════════════════════════════════════════╗
║      ABCFe PoA 컨센서스 시작 스크립트         ║
║      노드 수: 3개                              ║
╚══════════════════════════════════════════════╝

Node     REST       P2P        Height     Peers
-------- ---------- ---------- ---------- ----------
Node 1   8000       30303      0          2
Node 2   8001       30304      0          2
Node 3   8002       30305      0          2

✓ 모든 노드 시작 완료!
```

## Verify

### Check Node Status

```bash
curl http://localhost:8000/api/v1/status | jq
```

```json
{
  "success": true,
  "data": {
    "nodeId": "abcfe-node-1",
    "networkId": "abcfe-mainnet",
    "currentHeight": 5,
    "latestBlockHash": "2b1e0940cc2308fe...",
    "peerCount": 2,
    "consensusState": "IDLE"
  }
}
```

### Watch Blocks

```bash
# 블록 높이 모니터링
watch -n 1 'curl -s http://localhost:8000/api/v1/status | jq .data.currentHeight'
```

### WebSocket Events

```bash
# websocat 설치 (macOS)
brew install websocat

# WebSocket 연결
websocat ws://localhost:8000/ws
```

**Output:**
```json
{"event":"connected","data":{"message":"Connected to ABCFe WebSocket"}}
{"event":"consensus_state_change","data":{"state":"PROPOSING","height":6,"round":0,"proposerAddr":"d8f443..."}}
{"event":"consensus_state_change","data":{"state":"VOTING","height":6,"round":0,"proposerAddr":"d8f443..."}}
{"event":"consensus_state_change","data":{"state":"COMMITTING","height":6,"round":0,"proposerAddr":"d8f443..."}}
{"event":"new_block","data":{"height":6,"hash":"abc123...","txCount":1}}
{"event":"consensus_state_change","data":{"state":"IDLE","height":7,"round":0,"proposerAddr":""}}
```

## Stop Nodes

```bash
pkill -f abcfed
```

## Clean Up

```bash
./clean_all.sh
```

## Next Steps

- [Installation](installation.md) - 상세 설치 가이드
- [Multi-Node Setup](../operations/multi-node.md) - 멀티 노드 설정
- [WebSocket API](../api/websocket-api.md) - 실시간 이벤트 구독
- [BFT Consensus](../consensus/bft-consensus.md) - 컨센서스 이해하기
