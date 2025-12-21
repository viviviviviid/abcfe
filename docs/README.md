# ABCFe Blockchain Node

ABCFe는 BFT(Byzantine Fault Tolerant) 기반의 PoA(Proof of Authority) 블록체인 노드입니다.

## Features

- **BFT Consensus**: 2/3+ 검증자 합의 기반 블록 생성
- **UTXO Model**: Bitcoin 스타일의 트랜잭션 모델
- **HD Wallet**: BIP-39, BIP-44 호환 계층적 결정 지갑
- **P2P Network**: TCP 기반 피어 통신
- **REST API**: 블록, 트랜잭션, 지갑 관리 API
- **WebSocket**: 실시간 컨센서스 상태 스트리밍

## Quick Links

- [Quick Start](getting-started/quick-start.md) - 1분 안에 노드 실행하기
- [Consensus Mechanism](consensus/bft-consensus.md) - BFT 컨센서스 이해하기
- [WebSocket API](api/websocket-api.md) - 실시간 이벤트 구독하기
- [Frontend Integration](frontend/node-visualization.md) - 프론트엔드 연동 가이드

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        ABCFe Node                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │ REST API│  │WebSocket│  │Consensus│  │   P2P   │        │
│  │  :8000  │  │   /ws   │  │ Engine  │  │ :30303  │        │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘        │
│       │            │            │            │              │
│       └────────────┴─────┬──────┴────────────┘              │
│                          │                                  │
│                   ┌──────┴──────┐                           │
│                   │  BlockChain │                           │
│                   │   (Core)    │                           │
│                   └──────┬──────┘                           │
│                          │                                  │
│                   ┌──────┴──────┐                           │
│                   │   LevelDB   │                           │
│                   └─────────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

## Consensus Parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| BlockIntervalMs | 1000ms | 블록 간 최소 간격 |
| ProposingDurationMs | 2000ms | 제안 단계 지속 시간 |
| VotingDurationMs | 3000ms | 투표 단계 지속 시간 |
| CommittingDurationMs | 2000ms | 커밋 단계 지속 시간 |
| RoundTimeoutMs | 20000ms | 라운드 타임아웃 |

## Requirements

- Go 1.21+
- LevelDB
- macOS / Linux
