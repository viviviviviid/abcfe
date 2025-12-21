# Node Visualization Guide

프론트엔드에서 컨센서스 상태를 시각화하는 방법입니다.

## Overview

WebSocket의 `consensus_state_change` 이벤트만으로 모든 노드의 상태를 계산할 수 있습니다.

```
┌─────────────────────────────────────────────────────────────┐
│                   Frontend Visualization                    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   WebSocket Event          Frontend Calculation             │
│   ─────────────────        ─────────────────────           │
│                                                             │
│   state: PROPOSING         Node A: PROPOSING (제안자)       │
│   proposerAddr: A          Node B: IDLE                     │
│                            Node C: IDLE                     │
│                                                             │
│   state: VOTING            Node A: VOTING                   │
│   proposerAddr: A          Node B: VOTING                   │
│                            Node C: VOTING                   │
│                                                             │
│   state: COMMITTING        Node A: COMMITTING               │
│   proposerAddr: A          Node B: COMMITTING               │
│                            Node C: COMMITTING               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Implementation

### Step 1: Define Validators

검증자 목록은 설정에서 가져오거나 API로 조회합니다.

```javascript
// 설정에서 가져온 검증자 목록
const validators = [
  { address: '90efb3f6337ff1cc...', name: 'Node 1' },
  { address: '7d1afddad673b415...', name: 'Node 2' },
  { address: 'd495b0530f654940...', name: 'Node 3' },
];
```

### Step 2: State Management

```javascript
// 각 노드의 현재 상태
const nodeStates = {};

// 초기화
validators.forEach(v => {
  nodeStates[v.address] = 'IDLE';
});
```

### Step 3: Handle Events

```javascript
function handleConsensusStateChange(data) {
  const { state, proposerAddr, height, round } = data;

  validators.forEach(validator => {
    const nodeId = validator.address;

    switch (state) {
      case 'PROPOSING':
        // 제안자만 PROPOSING, 나머지는 IDLE
        nodeStates[nodeId] = (nodeId === proposerAddr) ? 'PROPOSING' : 'IDLE';
        break;

      case 'VOTING':
      case 'COMMITTING':
        // 모든 노드가 같은 상태
        nodeStates[nodeId] = state;
        break;

      case 'IDLE':
        // 모든 노드가 IDLE
        nodeStates[nodeId] = 'IDLE';
        break;
    }
  });

  // UI 업데이트
  updateVisualization();
}
```

### Step 4: Visualization

```javascript
function updateVisualization() {
  validators.forEach(validator => {
    const state = nodeStates[validator.address];
    const element = document.getElementById(`node-${validator.address}`);

    // 상태별 스타일 적용
    element.className = `node node-${state.toLowerCase()}`;
    element.querySelector('.state').textContent = state;
  });
}
```

## CSS Styling

```css
.node {
  width: 100px;
  height: 100px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
}

.node-idle {
  background: #gray;
  border: 3px solid #888;
}

.node-proposing {
  background: #4CAF50;
  border: 3px solid #2E7D32;
  animation: pulse 1s infinite;
}

.node-voting {
  background: #2196F3;
  border: 3px solid #1565C0;
}

.node-committing {
  background: #FF9800;
  border: 3px solid #EF6C00;
}

@keyframes pulse {
  0% { transform: scale(1); }
  50% { transform: scale(1.05); }
  100% { transform: scale(1); }
}
```

## React Component Example

```jsx
import React, { useState, useEffect } from 'react';

function ConsensusVisualizer({ validators }) {
  const [nodeStates, setNodeStates] = useState({});
  const [currentHeight, setCurrentHeight] = useState(0);
  const [currentProposer, setCurrentProposer] = useState('');

  useEffect(() => {
    const ws = new WebSocket('ws://localhost:8000/ws');

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);

      if (data.event === 'consensus_state_change') {
        const { state, proposerAddr, height } = data.data;

        setCurrentHeight(height);
        setCurrentProposer(proposerAddr);

        const newStates = {};
        validators.forEach(v => {
          if (state === 'PROPOSING') {
            newStates[v.address] = v.address === proposerAddr ? 'PROPOSING' : 'IDLE';
          } else {
            newStates[v.address] = state;
          }
        });
        setNodeStates(newStates);
      }
    };

    return () => ws.close();
  }, [validators]);

  return (
    <div className="consensus-visualizer">
      <h2>Height: {currentHeight}</h2>
      <div className="nodes">
        {validators.map(v => (
          <NodeCircle
            key={v.address}
            name={v.name}
            address={v.address}
            state={nodeStates[v.address] || 'IDLE'}
            isProposer={v.address === currentProposer}
          />
        ))}
      </div>
    </div>
  );
}

function NodeCircle({ name, address, state, isProposer }) {
  return (
    <div className={`node node-${state.toLowerCase()}`}>
      <div className="name">{name}</div>
      <div className="state">{state}</div>
      {isProposer && <div className="proposer-badge">Proposer</div>}
    </div>
  );
}
```

## State Flow Visualization

```
Time →

Height 100:
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  IDLE   │PROPOSING│ VOTING  │COMMITING│  IDLE   │
├─────────┼─────────┼─────────┼─────────┼─────────┤
│ Node A  │ Node A* │ Node A  │ Node A  │ Node A  │
│ Node B  │ Node B  │ Node B  │ Node B  │ Node B  │
│ Node C  │ Node C  │ Node C  │ Node C  │ Node C  │
└─────────┴─────────┴─────────┴─────────┴─────────┘
           * = Proposer

Height 101:
┌─────────┬─────────┬─────────┬─────────┬─────────┐
│  IDLE   │PROPOSING│ VOTING  │COMMITING│  IDLE   │
├─────────┼─────────┼─────────┼─────────┼─────────┤
│ Node A  │ Node A  │ Node A  │ Node A  │ Node A  │
│ Node B  │ Node B* │ Node B  │ Node B  │ Node B  │
│ Node C  │ Node C  │ Node C  │ Node C  │ Node C  │
└─────────┴─────────┴─────────┴─────────┴─────────┘
           * = Proposer (라운드로빈으로 변경)
```

## Tips

### 1. Address Truncation

주소가 길므로 표시할 때는 앞/뒤 일부만 표시:

```javascript
function truncateAddress(address) {
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}
// "d8f443...ce9f"
```

### 2. Animation Timing

상태 전환 시 부드러운 애니메이션:

```css
.node {
  transition: background-color 0.5s ease,
              transform 0.3s ease;
}
```

### 3. Proposer Highlight

현재 제안자를 강조 표시:

```jsx
{isProposer && (
  <div className="proposer-indicator">
    <span className="crown">👑</span>
  </div>
)}
```

## See Also

- [WebSocket API](../api/websocket-api.md) - WebSocket 이벤트 상세
- [BFT Consensus](../consensus/bft-consensus.md) - 컨센서스 동작 이해
