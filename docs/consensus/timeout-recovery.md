# Timeout & Recovery

컨센서스 과정에서 지연이나 장애가 발생했을 때의 복구 메커니즘입니다.

## Timeout Parameters

```go
const (
    RoundTimeoutMs = 20000  // 라운드 타임아웃 (20초)
)
```

## Timeout Scenarios

### Scenario 1: Normal Operation

```
┌─────────────────────────────────────────────────────────┐
│ Normal Block Production (~8-10초)                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  IDLE ─(1s)─→ PROPOSING ─(2s)─→ VOTING ─(~4s)─→        │
│                                                         │
│  ─→ COMMITTING ─(2s)─→ IDLE                            │
│                                                         │
│  Total: ~9초 (20초 타임아웃 이내)                        │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Scenario 2: Single Round Timeout

제안자가 오프라인이거나 네트워크 지연으로 20초 초과 시:

```
┌─────────────────────────────────────────────────────────┐
│ Round Timeout Flow                                      │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Height: 100, Round: 0                                  │
│  Proposer: Node A (오프라인)                             │
│                                                         │
│  0s   ─ IDLE (Node A 대기 중)                           │
│  ...                                                    │
│  20s  ─ TIMEOUT 발생!                                   │
│         │                                               │
│         ▼                                               │
│  ┌──────────────────────────────────────────┐          │
│  │ Round 증가: Round 0 → Round 1            │          │
│  │ 새 제안자 선택: Node B                    │          │
│  │ VoteSet 초기화                           │          │
│  │ 새 타임아웃 타이머 시작                   │          │
│  └──────────────────────────────────────────┘          │
│         │                                               │
│         ▼                                               │
│  Height: 100, Round: 1                                  │
│  Proposer: Node B (정상)                                │
│                                                         │
│  → 정상 블록 생성 진행                                   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Scenario 3: Multiple Consecutive Timeouts

3번 연속 타임아웃 시 블록 동기화 시도:

```
┌─────────────────────────────────────────────────────────┐
│ 3 Consecutive Timeouts → Block Sync                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Timeout 1: Round 0 → Round 1                          │
│      │                                                  │
│      ▼                                                  │
│  Timeout 2: Round 1 → Round 2                          │
│      │                                                  │
│      ▼                                                  │
│  Timeout 3: Round 2 → 블록 동기화 시도!                  │
│      │                                                  │
│      ▼                                                  │
│  ┌──────────────────────────────────────────┐          │
│  │ 1. consecutiveTimeouts = 3 감지          │          │
│  │ 2. 피어에게 블록 요청 (SyncBlocks)        │          │
│  │ 3. 누락된 블록 수신 및 적용               │          │
│  │ 4. 새 높이로 컨센서스 재시작              │          │
│  └──────────────────────────────────────────┘          │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Timeout Handler Code

```go
// consensus/engine.go

func (e *ConsensusEngine) handleRoundTimeout(height uint64, round uint32) {
    e.mu.Lock()
    defer e.mu.Unlock()

    // 이미 다음 높이/라운드로 진행했으면 무시
    if e.consensus.CurrentHeight != height || e.consensus.CurrentRound != round {
        e.consecutiveTimeouts = 0
        return
    }

    e.consecutiveTimeouts++
    logger.Warn("[Consensus] Round timeout at height ", height,
                " round ", round, " (consecutive: ", e.consecutiveTimeouts, ")")

    // 3번 연속 타임아웃 시 블록 동기화 시도
    if e.consecutiveTimeouts >= 3 {
        logger.Warn("[Consensus] Too many consecutive timeouts, attempting block sync...")
        e.consecutiveTimeouts = 0

        if e.syncer != nil && e.syncer.GetPeerCount() > 0 {
            go func() {
                if err := e.syncer.SyncBlocks(); err != nil {
                    logger.Debug("[Consensus] Block sync during timeout: ", err)
                } else {
                    logger.Info("[Consensus] Block sync completed after timeout")
                }
            }()
        }

        // 동기화 후 높이 업데이트
        newHeight, _ := e.blockchain.GetLatestHeight()
        if newHeight > currentHeight {
            e.consensus.CurrentHeight = newHeight + 1
            e.consensus.CurrentRound = 0
            return
        }
    }

    // 라운드 증가 → 다음 제안자 차례
    e.consensus.IncrementRound()
    e.proposedBlock = nil
    e.prevotes = nil
    e.precommits = nil

    // 다음 라운드 제안자가 로컬 노드인 경우 블록 제안
    proposer := e.consensus.SelectProposerByMode(height, e.consensus.CurrentRound, prevBlockHash)
    if proposer.Address == e.consensus.LocalValidator.Address {
        e.proposeBlock()
        e.broadcastProposalInternal()
    }

    // 새 타임아웃 타이머 시작
    e.startRoundTimer()
}
```

## Recovery Summary

| Situation | Action | Result |
|-----------|--------|--------|
| 제안자 오프라인 | 20초 후 라운드 증가 | 다음 제안자가 블록 생성 |
| 네트워크 지연 | 투표 늦게 도착해도 2/3 모이면 진행 | 정상 커밋 |
| 1번 타임아웃 | Round++ | 다음 제안자로 교체 |
| 2번 타임아웃 | Round++ | 또 다음 제안자로 교체 |
| 3번 연속 타임아웃 | 블록 동기화 시도 | 피어에서 블록 받아옴 |
| 블록 검증 실패 | 해당 Proposal 무시 | 타임아웃 후 다음 라운드 |
| 서명 검증 실패 | 해당 투표 무시 | 다른 투표로 2/3 채움 |

## Timing Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Timeout Timeline                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Round 0        Round 1        Round 2        Sync          │
│  ├────────────┼────────────┼────────────┼──────────────┤   │
│  0s           20s          40s          60s           70s+  │
│                                                             │
│  Proposer A   Proposer B   Proposer C   Block Sync          │
│  (timeout)    (timeout)    (timeout)    from peers          │
│                                                             │
│  ▼            ▼            ▼            ▼                   │
│  Wait for     Wait for     Wait for     Sync blocks         │
│  proposal     proposal     proposal     Update height       │
│  ...          ...          ...          Continue            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Best Practices

1. **네트워크 안정성**: 피어 간 연결 상태 모니터링
2. **타임아웃 조정**: 네트워크 상황에 따라 `RoundTimeoutMs` 조정
3. **로그 모니터링**: `[Consensus] Round timeout` 로그 확인
4. **피어 수 유지**: 최소 2개 이상의 피어와 연결 유지

## See Also

- [BFT Consensus](bft-consensus.md) - 컨센서스 개요
- [State Machine](state-machine.md) - 상태 전이
- [Proposer Selection](proposer-selection.md) - 제안자 선출
