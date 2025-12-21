# BFT Consensus

ABCFe는 **PBFT(Practical Byzantine Fault Tolerant)** 기반의 컨센서스 알고리즘을 사용합니다.

## Overview

BFT 컨센서스는 네트워크의 최대 1/3 노드가 비잔틴(악의적) 행동을 하더라도 정상 동작을 보장합니다.

```
┌─────────────────────────────────────────────────────────────┐
│                    BFT Consensus Flow                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   IDLE ──→ PROPOSING ──→ VOTING ──→ COMMITTING ──→ IDLE    │
│    │          │            │            │           │       │
│    │          │            │            │           │       │
│    │       2초 대기     투표 수집      2초 대기    다음 블록  │
│    │      블록 생성    (2/3+ 필요)   블록 저장              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Consensus States

### 1. IDLE
- 이전 블록 커밋 후 대기 상태
- `BlockIntervalMs (1초)` 후 다음 라운드 시작

### 2. PROPOSING
- 제안자(Proposer)가 블록을 생성하고 브로드캐스트
- 다른 노드들은 제안을 수신 대기
- 지속 시간: `ProposingDurationMs (2초)`

### 3. VOTING
- 모든 검증자가 Prevote → Precommit 투표
- 2/3 이상 동의 시 다음 단계로 진행
- 투표는 랜덤 지연(0~3초) 후 전송 (시각화 용이)

### 4. COMMITTING
- 블록을 로컬 DB에 저장
- P2P로 다른 노드에 블록 전파
- 지속 시간: `CommittingDurationMs (2초)`

## Voting Process

```
┌─────────────────────────────────────────────────────────────┐
│                    Two-Phase Voting                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Phase 1: Prevote                                           │
│  ┌─────────┐     ┌─────────┐     ┌─────────┐               │
│  │ Node A  │────→│ Prevote │────→│ Collect │               │
│  │ Node B  │────→│ Prevote │────→│  2/3+   │               │
│  │ Node C  │────→│ Prevote │────→│         │               │
│  └─────────┘     └─────────┘     └────┬────┘               │
│                                       │                     │
│                                       ▼                     │
│  Phase 2: Precommit                                         │
│  ┌─────────┐     ┌──────────┐    ┌─────────┐               │
│  │ Node A  │────→│Precommit │───→│ Collect │               │
│  │ Node B  │────→│Precommit │───→│  2/3+   │               │
│  │ Node C  │────→│Precommit │───→│         │               │
│  └─────────┘     └──────────┘    └────┬────┘               │
│                                       │                     │
│                                       ▼                     │
│                               ┌──────────────┐              │
│                               │ Block Commit │              │
│                               └──────────────┘              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 2/3+ Majority Requirement

BFT 안전성을 위해 **2/3 이상**의 투표력(Voting Power)이 필요합니다.

```go
// 2/3 majority check
func HasTwoThirdsMajority(votedPower, totalPower uint64) bool {
    return votedPower * 3 > totalPower * 2
}
```

**예시 (3 노드, 각 1 투표력):**
- Total Power: 3
- 2/3+ 필요: 3 * 2 / 3 = 2 이상
- 최소 2개 노드 동의 필요

## Block Structure with BFT

```go
type Block struct {
    Header           BlockHeader
    Transactions     []*Transaction
    Proposer         Address           // 블록 제안자 주소
    Signature        Signature         // 제안자의 블록 서명
    CommitSignatures []CommitSignature // 2/3+ 검증자 서명
}

type CommitSignature struct {
    ValidatorAddress Address
    Signature        Signature
    Timestamp        int64
}
```

## Consensus Parameters

```go
const (
    // 블록 생성 관련
    BlockProduceTimeMs = 1000   // 블록 생성 체크 간격
    BlockIntervalMs    = 1000   // 블록 간 최소 간격

    // 단계별 지속 시간 (WebSocket 시각화용)
    ProposingDurationMs  = 2000 // PROPOSING 단계
    VotingDurationMs     = 3000 // VOTING 단계 (랜덤 지연 범위)
    CommittingDurationMs = 2000 // COMMITTING 단계

    // 타임아웃
    RoundTimeoutMs = 20000      // 라운드 타임아웃
)
```

## Example Timeline

```
Time    Event                           State
─────────────────────────────────────────────────────
0.0s    블록 커밋 완료                    IDLE
1.0s    BlockInterval 경과               IDLE → PROPOSING
1.0s    제안자가 블록 생성 시작
3.0s    ProposingDuration 종료           PROPOSING → VOTING
3.0s    모든 노드 Prevote 전송 시작
3.5s    Node A Prevote (랜덤 지연)
4.2s    Node B Prevote (랜덤 지연)
5.1s    Node C Prevote (랜덤 지연)
5.1s    2/3+ Prevote 도달 → Precommit 시작
5.8s    Node A Precommit
6.3s    Node B Precommit
6.9s    2/3+ Precommit 도달              VOTING → COMMITTING
8.9s    CommittingDuration 종료
8.9s    블록 저장 완료                    COMMITTING → IDLE
9.9s    다음 라운드 시작                  IDLE → PROPOSING
```

## See Also

- [State Machine](state-machine.md) - 상태 전이 상세
- [Proposer Selection](proposer-selection.md) - 제안자 선출 방식
- [Timeout & Recovery](timeout-recovery.md) - 타임아웃 처리
