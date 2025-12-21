# WebSocket API

실시간 블록체인 이벤트를 수신하기 위한 WebSocket API입니다.

## Connection

```
ws://localhost:8000/ws
```

## Event Types

### 1. connected

연결 성공 시 전송됩니다.

```json
{
  "event": "connected",
  "data": {
    "message": "Connected to ABCFe WebSocket"
  }
}
```

### 2. consensus_state_change

컨센서스 상태가 변경될 때마다 전송됩니다.

```json
{
  "event": "consensus_state_change",
  "data": {
    "state": "PROPOSING",
    "height": 100,
    "round": 0,
    "proposerAddr": "d8f443307fb8c210e171e1765fc79b73b176ce9f"
  }
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| state | string | 현재 상태: `IDLE`, `PROPOSING`, `VOTING`, `COMMITTING` |
| height | number | 블록 높이 |
| round | number | 현재 라운드 (타임아웃 시 증가) |
| proposerAddr | string | 현재 블록 제안자 주소 (IDLE일 때 빈 문자열) |

**State Values:**

| State | Description |
|-------|-------------|
| `IDLE` | 대기 상태, 다음 블록 준비 중 |
| `PROPOSING` | 제안자가 블록 생성 중 |
| `VOTING` | 검증자들이 투표 중 (Prevote/Precommit) |
| `COMMITTING` | 블록 저장 및 전파 중 |

### 3. new_block

새 블록이 생성되면 전송됩니다.

```json
{
  "event": "new_block",
  "data": {
    "height": 100,
    "hash": "2b1e0940cc2308feaa93754bc61659e33e470d79c43327249bcef77c9783e4c0",
    "prevHash": "420ce908c82cc3df0738ad0ddb5f87407f897565226dd302b2be83b7cb70c148",
    "timestamp": 1766295957,
    "txCount": 1
  }
}
```

### 4. new_transaction

새 트랜잭션이 mempool에 추가되면 전송됩니다.

```json
{
  "event": "new_transaction",
  "data": {
    "txId": "abc123...",
    "timestamp": 1766295957,
    "memo": "Transfer",
    "inputs": 1,
    "outputs": 2
  }
}
```

## Event Flow Example

```
Time     Event                              Data
───────────────────────────────────────────────────────────────
0.0s     connected                          Connected to ABCFe
1.0s     consensus_state_change             PROPOSING, height=100
3.0s     consensus_state_change             VOTING, height=100
7.0s     consensus_state_change             COMMITTING, height=100
9.0s     new_block                          height=100
9.0s     consensus_state_change             IDLE, height=101
10.0s    consensus_state_change             PROPOSING, height=101
...
```

## JavaScript Client Example

```javascript
const ws = new WebSocket('ws://localhost:8000/ws');

ws.onopen = () => {
  console.log('Connected to ABCFe WebSocket');
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  switch (data.event) {
    case 'consensus_state_change':
      handleConsensusStateChange(data.data);
      break;
    case 'new_block':
      handleNewBlock(data.data);
      break;
    case 'new_transaction':
      handleNewTransaction(data.data);
      break;
  }
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected from WebSocket');
  // Reconnect logic here
};
```

## Python Client Example

```python
import asyncio
import websockets
import json

async def listen():
    uri = "ws://localhost:8000/ws"
    async with websockets.connect(uri) as ws:
        while True:
            message = await ws.recv()
            data = json.loads(message)

            if data['event'] == 'consensus_state_change':
                state = data['data']['state']
                height = data['data']['height']
                print(f"State: {state}, Height: {height}")

            elif data['event'] == 'new_block':
                height = data['data']['height']
                print(f"New block: {height}")

asyncio.run(listen())
```

## Connection Management

### Reconnection Strategy

```javascript
class WebSocketClient {
  constructor(url) {
    this.url = url;
    this.reconnectInterval = 3000;
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('Connected');
      this.reconnectInterval = 3000; // Reset on success
    };

    this.ws.onclose = () => {
      console.log('Disconnected, reconnecting...');
      setTimeout(() => this.connect(), this.reconnectInterval);
      this.reconnectInterval = Math.min(this.reconnectInterval * 2, 30000);
    };

    this.ws.onmessage = (event) => {
      this.handleMessage(JSON.parse(event.data));
    };
  }

  handleMessage(data) {
    // Handle events
  }
}
```

## Traffic Optimization

현재 WebSocket 트래픽:

| Metric | Value |
|--------|-------|
| Events per block | ~4개 (PROPOSING → VOTING → COMMITTING → NEW_BLOCK → IDLE) |
| Bytes per event | ~100-200 bytes |
| Events per minute | ~24개 (10초 블록 기준) |
| Bandwidth | ~200 bytes/sec |

## See Also

- [Frontend Integration](../frontend/node-visualization.md) - 프론트엔드 연동
- [REST API](rest-api.md) - REST API 문서
