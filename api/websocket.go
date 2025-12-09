package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/common/utils"
	"github.com/abcfe/abcfe-node/core"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 모든 origin 허용 (개발용)
	},
}

// WSEventType 이벤트 타입
type WSEventType string

const (
	EventNewBlock          WSEventType = "new_block"
	EventNewTransaction    WSEventType = "new_transaction"
	EventBlockConfirmed    WSEventType = "block_confirmed"
	EventConsensusStateChange WSEventType = "consensus_state_change"
)

// WSMessage WebSocket 메시지 구조
type WSMessage struct {
	Event WSEventType `json:"event"`
	Data  interface{} `json:"data"`
}

// WSHub 클라이언트 연결 관리
type WSHub struct {
	clients    map[*WSClient]bool
	broadcast  chan WSMessage
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
}

// WSClient WebSocket 클라이언트
type WSClient struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte
}

// NewWSHub 새 Hub 생성
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan WSMessage),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

// Run Hub 실행
func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logger.Debug("WebSocket client connected. Total:", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			logger.Debug("WebSocket client disconnected. Total:", len(h.clients))

		case message := <-h.broadcast:
			data, err := json.Marshal(message)
			if err != nil {
				logger.Error("Failed to marshal WebSocket message:", err)
				continue
			}

			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastNewBlock 새 블록 알림 브로드캐스트
func (h *WSHub) BroadcastNewBlock(block *core.Block) {
	blockData := map[string]interface{}{
		"height":    block.Header.Height,
		"hash":      utils.HashToString(block.Header.Hash),
		"prevHash":  utils.HashToString(block.Header.PrevHash),
		"timestamp": block.Header.Timestamp,
		"txCount":   len(block.Transactions),
	}

	h.broadcast <- WSMessage{
		Event: EventNewBlock,
		Data:  blockData,
	}
}

// BroadcastNewTransaction 새 트랜잭션 알림 브로드캐스트
func (h *WSHub) BroadcastNewTransaction(tx *core.Transaction) {
	txData := map[string]interface{}{
		"txId":      utils.HashToString(tx.ID),
		"timestamp": tx.Timestamp,
		"memo":      tx.Memo,
		"inputs":    len(tx.Inputs),
		"outputs":   len(tx.Outputs),
	}

	h.broadcast <- WSMessage{
		Event: EventNewTransaction,
		Data:  txData,
	}
}

// GetClientCount 연결된 클라이언트 수 반환
func (h *WSHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// BroadcastConsensusState 컨센서스 상태 변경 브로드캐스트
func (h *WSHub) BroadcastConsensusState(state string, height uint64, round uint32, proposerAddr string) {
	stateData := map[string]interface{}{
		"state":        state,
		"height":       height,
		"round":        round,
		"proposerAddr": proposerAddr,
	}

	h.broadcast <- WSMessage{
		Event: EventConsensusStateChange,
		Data:  stateData,
	}
}

// HandleWebSocket WebSocket 연결 핸들러
func HandleWebSocket(hub *WSHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("WebSocket upgrade error:", err)
			return
		}

		client := &WSClient{
			hub:  hub,
			conn: conn,
			send: make(chan []byte, 256),
		}

		hub.register <- client

		// 클라이언트에게 연결 성공 메시지 전송
		welcomeMsg := WSMessage{
			Event: "connected",
			Data: map[string]interface{}{
				"message": "Connected to ABCFe WebSocket",
			},
		}
		data, _ := json.Marshal(welcomeMsg)
		client.send <- data

		// 읽기/쓰기 고루틴 시작
		go client.writePump()
		go client.readPump()
	}
}

// writePump 클라이언트에게 메시지 전송
func (c *WSClient) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

// readPump 클라이언트로부터 메시지 수신
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("WebSocket read error:", err)
			}
			break
		}
		// 클라이언트 메시지 처리 (현재는 무시)
	}
}
