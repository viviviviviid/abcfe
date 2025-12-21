# Consensus State Machine

컨센서스 엔진은 4가지 상태를 순환하는 유한 상태 기계(FSM)입니다.

## State Diagram

```
                              ┌─────────────────────────────────────┐
                              │                                     │
                              ▼                                     │
┌──────────┐  BlockInterval  ┌──────────────┐                       │
│          │ ───────────────→│              │                       │
│   IDLE   │                 │  PROPOSING   │                       │
│          │←────────────────│              │                       │
└──────────┘   Timeout/Fail  └──────┬───────┘                       │
      ▲                             │                               │
      │                             │ ProposingDuration             │
      │                             ▼                               │
      │                      ┌──────────────┐                       │
      │                      │              │                       │
      │    RoundTimeout      │   VOTING     │                       │
      │ ◄────────────────────│              │                       │
      │                      └──────┬───────┘                       │
      │                             │                               │
      │                             │ 2/3+ Precommit                │
      │                             ▼                               │
      │                      ┌──────────────┐                       │
      │                      │              │                       │
      │                      │ COMMITTING   │───────────────────────┘
      │                      │              │   CommittingDuration
      │                      └──────────────┘
      │                             │
      └─────────────────────────────┘
              Block Committed
```

## State Definitions

### IDLE

```go
const StateIdle ConsensusState = "IDLE"
```

**진입 조건:**
- 노드 시작 시 초기 상태
- 블록 커밋 완료 후
- 라운드 타임아웃 후 (다음 라운드 대기)

**동작:**
- `BlockIntervalMs (1초)` 대기
- 다음 블록 높이의 제안자 확인

**탈출 조건:**
- 제안자인 경우 → PROPOSING
- 제안자가 아닌 경우 → Proposal 수신 대기

---

### PROPOSING

```go
const StateProposing ConsensusState = "PROPOSING"
```

**진입 조건:**
- 로컬 노드가 현재 라운드의 제안자인 경우
- 다른 노드로부터 Proposal 수신 시

**동작 (제안자):**
1. Mempool에서 트랜잭션 선택
2. 블록 헤더 구성 (해시, 높이, 타임스탬프)
3. 블록 서명
4. P2P로 Proposal 브로드캐스트

**동작 (비-제안자):**
1. Proposal 메시지 수신
2. 블록 유효성 검증
3. 제안자 서명 검증

**지속 시간:** `ProposingDurationMs (2초)`

**탈출 조건:**
- ProposingDuration 종료 → VOTING

---

### VOTING

```go
const StateVoting ConsensusState = "VOTING"
```

**진입 조건:**
- PROPOSING 단계 완료 후

**동작:**
1. **Prevote 단계**
   - 블록 해시에 서명하여 Prevote 전송
   - 랜덤 지연 (0~3초) 후 전송
   - 2/3+ Prevote 수집 시 Precommit 단계로

2. **Precommit 단계**
   - 블록 해시에 서명하여 Precommit 전송
   - 랜덤 지연 (0~3초) 후 전송
   - 2/3+ Precommit 수집 시 커밋

**탈출 조건:**
- 2/3+ Precommit 도달 → COMMITTING
- RoundTimeout → IDLE (다음 라운드)

---

### COMMITTING

```go
const StateCommitting ConsensusState = "COMMITTING"
```

**진입 조건:**
- 2/3+ Precommit 수집 완료

**동작:**
1. CommitSignatures 배열에 검증자 서명 수집
2. 블록을 로컬 DB에 저장
3. UTXO 상태 업데이트
4. P2P로 새 블록 브로드캐스트
5. WebSocket으로 `new_block` 이벤트 전송

**지속 시간:** `CommittingDurationMs (2초)`

**탈출 조건:**
- CommittingDuration 종료 → IDLE

---

## State Transition Table

| Current State | Event | Next State | Action |
|--------------|-------|------------|--------|
| IDLE | BlockInterval 경과 & 제안자 | PROPOSING | 블록 생성 시작 |
| IDLE | BlockInterval 경과 & 비-제안자 | IDLE | Proposal 대기 |
| IDLE | Proposal 수신 | PROPOSING | 블록 검증 |
| PROPOSING | ProposingDuration 종료 | VOTING | Prevote 전송 |
| VOTING | 2/3+ Prevote | VOTING | Precommit 전송 |
| VOTING | 2/3+ Precommit | COMMITTING | 블록 저장 시작 |
| VOTING | RoundTimeout | IDLE | 라운드 증가 |
| COMMITTING | CommittingDuration 종료 | IDLE | 다음 높이로 |

## Code Reference

```go
// consensus/engine.go

func (e *ConsensusEngine) proposeBlock() {
    // PROPOSING 상태 설정 및 브로드캐스트
    e.consensus.State = StateProposing
    e.broadcastState(proposerAddr)

    // 2초 대기 (프론트엔드에서 PROPOSING 상태 확인 가능)
    time.Sleep(time.Duration(ProposingDurationMs) * time.Millisecond)

    // 블록 생성 로직...
}

func (e *ConsensusEngine) broadcastProposal() {
    // VOTING 상태로 전환
    e.consensus.State = StateVoting
    e.broadcastState(proposerID)

    // Prevote 전송
    e.castVote(VoteTypePrevote, blockHash)
}

func (e *ConsensusEngine) commitBlockWithSignatures(block *core.Block) {
    // COMMITTING 상태 설정
    e.consensus.State = StateCommitting
    e.broadcastState(proposerAddr)

    // 2초 대기
    time.Sleep(time.Duration(CommittingDurationMs) * time.Millisecond)

    // 블록 저장
    e.blockchain.AddBlock(*block)

    // IDLE로 전환
    e.consensus.UpdateHeight(block.Header.Height + 1)
    e.broadcastState("")
}
```

## See Also

- [BFT Consensus](bft-consensus.md) - 컨센서스 개요
- [Timeout & Recovery](timeout-recovery.md) - 타임아웃 처리
