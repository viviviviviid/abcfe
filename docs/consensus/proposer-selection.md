# Proposer Selection

블록 제안자를 선출하는 알고리즘입니다.

## Selection Modes

ABCFe는 3가지 제안자 선출 모드를 지원합니다.

### 1. Round Robin (기본)

```go
ProposerSelectionMode = "roundrobin"
```

가장 단순하고 예측 가능한 방식입니다.

```
Height   Proposer
───────────────────
1        Validator[0]
2        Validator[1]
3        Validator[2]
4        Validator[0]  ← 반복
5        Validator[1]
...
```

**알고리즘:**
```go
func SelectProposer(height uint64, round uint32) *Validator {
    validators := GetActiveValidators()
    index := (height + uint64(round)) % uint64(len(validators))
    return validators[index]
}
```

**특징:**
- 예측 가능 (다음 제안자를 미리 알 수 있음)
- 공평한 기회 분배
- 단점: 악의적 노드가 타겟팅 가능

---

### 2. VRF (Verifiable Random Function)

```go
ProposerSelectionMode = "vrf"
```

이전 블록 해시를 시드로 사용하는 검증 가능한 랜덤 선출.

```
┌─────────────────────────────────────────────────────────────┐
│                     VRF Selection                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   Previous Block Hash                                       │
│         │                                                   │
│         ▼                                                   │
│   ┌─────────────────────────┐                               │
│   │ hash(prevHash + height  │                               │
│   │       + round)          │                               │
│   └───────────┬─────────────┘                               │
│               │                                             │
│               ▼                                             │
│        Random Seed                                          │
│               │                                             │
│               ▼                                             │
│   index = seed % len(validators)                            │
│               │                                             │
│               ▼                                             │
│        Selected Proposer                                    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**알고리즘:**
```go
func SelectProposerVRF(height uint64, round uint32, prevBlockHash Hash) *Validator {
    validators := GetActiveValidators()

    // Seed 생성
    seedData := fmt.Sprintf("%s:%d:%d", HashToString(prevBlockHash), height, round)
    seed := sha256.Sum256([]byte(seedData))
    seedInt := new(big.Int).SetBytes(seed[:])

    // 인덱스 계산
    index := seedInt.Mod(seedInt, big.NewInt(int64(len(validators))))

    return validators[index.Int64()]
}
```

**특징:**
- 랜덤하지만 검증 가능
- 모든 노드가 같은 결과 계산
- 이전 블록 커밋 후에만 다음 제안자 알 수 있음

---

### 3. Hybrid

```go
ProposerSelectionMode = "hybrid"
```

Round Robin과 VRF를 조합한 방식.

```
if round == 0 {
    // 첫 라운드: VRF 랜덤 선출
    return SelectProposerVRF(height, round, prevBlockHash)
} else {
    // 타임아웃 후: Round Robin으로 순차 진행
    return SelectProposer(height, round)
}
```

**특징:**
- 정상 상황: VRF로 랜덤 선출
- 타임아웃 발생: 예측 가능한 순서로 빠른 복구

---

## Configuration

`config.toml`에서 설정:

```toml
[consensus]
proposer_selection = "roundrobin"  # roundrobin, vrf, hybrid
```

## Round-based Selection

타임아웃 발생 시 라운드가 증가하고, 새 제안자가 선출됩니다.

```
Height 100, Round 0:
  Proposer = Validator[100 % 3] = Validator[1]
  → 20초 타임아웃 발생!

Height 100, Round 1:
  Proposer = Validator[(100 + 1) % 3] = Validator[2]
  → 정상 블록 생성

Height 101, Round 0:
  Proposer = Validator[101 % 3] = Validator[2]
```

## Validator Set

검증자 목록은 `config.toml`에서 정의:

```toml
[[validators]]
address = "90efb3f6337ff1cc..."
public_key = "04abc123..."
voting_power = 1

[[validators]]
address = "7d1afddad673b415..."
public_key = "04def456..."
voting_power = 1

[[validators]]
address = "d495b0530f654940..."
public_key = "04ghi789..."
voting_power = 1
```

## Voting Power

투표력(Voting Power)은 제안자 선출 확률에 영향을 미칩니다.

```go
// VRF with Voting Power
func SelectProposerVRFWeighted(height, round, prevHash) *Validator {
    // 투표력에 비례한 가중치 적용
    totalPower := sum(validator.VotingPower for all validators)
    randomValue := hash(prevHash, height, round) % totalPower

    cumulative := 0
    for _, v := range validators {
        cumulative += v.VotingPower
        if randomValue < cumulative {
            return v
        }
    }
}
```

## Code Reference

```go
// consensus/selection.go

type ProposerSelector struct {
    validatorSet *ValidatorSet
}

func (ps *ProposerSelector) SelectProposer(height uint64, round uint32) *Validator {
    validators := ps.validatorSet.GetActiveValidators()
    if len(validators) == 0 {
        return nil
    }
    index := (height + uint64(round)) % uint64(len(validators))
    return validators[index]
}

func (ps *ProposerSelector) SelectProposerVRF(height uint64, round uint32, prevBlockHash Hash) *Validator {
    validators := ps.validatorSet.GetActiveValidators()
    if len(validators) == 0 {
        return nil
    }

    seedData := fmt.Sprintf("%s:%d:%d", HashToString(prevBlockHash), height, round)
    seed := sha256.Sum256([]byte(seedData))
    seedInt := new(big.Int).SetBytes(seed[:])
    index := seedInt.Mod(seedInt, big.NewInt(int64(len(validators))))

    return validators[index.Int64()]
}
```

## See Also

- [BFT Consensus](bft-consensus.md) - 컨센서스 개요
- [Timeout & Recovery](timeout-recovery.md) - 타임아웃과 라운드 증가
