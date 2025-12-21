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
		return true // Allow all origins (for development)
	},
}

// WSEventType event type
type WSEventType string

const (
	EventNewBlock             WSEventType = "new_block"
	EventNewTransaction       WSEventType = "new_transaction"
	EventBlockConfirmed       WSEventType = "block_confirmed"
	EventConsensusStateChange WSEventType = "consensus_state_change"
)

// WSMessage WebSocket message structure
type WSMessage struct {
	Event WSEventType `json:"event"`
	Data  interface{} `json:"data"`
}

// ConsensusStateProvider provides current consensus state
type ConsensusStateProvider func() (state string, height uint64, round uint32, proposerAddr string)

// LatestBlockProvider provides current latest block
type LatestBlockProvider func() *core.Block

// WSHub client connection management
type WSHub struct {
	clients                map[*WSClient]bool
	broadcast              chan WSMessage
	register               chan *WSClient
	unregister             chan *WSClient
	mu                     sync.RWMutex
	consensusStateProvider ConsensusStateProvider
	latestBlockProvider    LatestBlockProvider
}

// WSClient WebSocket client
type WSClient struct {
	hub  *WSHub
	conn *websocket.Conn
	send chan []byte
}

// NewWSHub creates new Hub
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan WSMessage),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

// SetConsensusStateProvider sets the consensus state provider callback
func (h *WSHub) SetConsensusStateProvider(provider ConsensusStateProvider) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.consensusStateProvider = provider
}

// SetLatestBlockProvider sets the latest block provider callback
func (h *WSHub) SetLatestBlockProvider(provider LatestBlockProvider) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.latestBlockProvider = provider
}

// getCurrentConsensusState gets current consensus state message
func (h *WSHub) getCurrentConsensusState() []byte {
	h.mu.RLock()
	provider := h.consensusStateProvider
	h.mu.RUnlock()

	if provider == nil {
		return nil
	}

	state, height, round, proposerAddr := provider()

	msg := WSMessage{
		Event: EventConsensusStateChange,
		Data: map[string]interface{}{
			"state":        state,
			"height":       height,
			"round":        round,
			"proposerAddr": proposerAddr,
		},
	}
	data, _ := json.Marshal(msg)
	return data
}

// getLatestBlockMessage gets latest block message
func (h *WSHub) getLatestBlockMessage() []byte {
	h.mu.RLock()
	provider := h.latestBlockProvider
	h.mu.RUnlock()

	if provider == nil {
		return nil
	}

	block := provider()
	if block == nil {
		return nil
	}

	msg := WSMessage{
		Event: EventNewBlock,
		Data: map[string]interface{}{
			"height":    block.Header.Height,
			"hash":      utils.HashToString(block.Header.Hash),
			"prevHash":  utils.HashToString(block.Header.PrevHash),
			"timestamp": block.Header.Timestamp,
			"txCount":   len(block.Transactions),
		},
	}
	data, _ := json.Marshal(msg)
	return data
}

// Run runs the Hub
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

// BroadcastNewBlock broadcasts new block notification
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

// BroadcastNewTransaction broadcasts new transaction notification
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

// GetClientCount returns connected client count
func (h *WSHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// BroadcastConsensusState broadcasts consensus state change
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



// HandleWebSocket WebSocket connection handler
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

		// Send connection success message to client
		welcomeMsg := WSMessage{
			Event: "connected",
			Data: map[string]interface{}{
				"message": "Connected to ABCFe WebSocket",
			},
		}
		data, _ := json.Marshal(welcomeMsg)
		client.send <- data

		// Send current consensus state immediately after connection
		if stateData := hub.getCurrentConsensusState(); stateData != nil {
			client.send <- stateData
		}

		// Send latest block immediately after connection
		if blockData := hub.getLatestBlockMessage(); blockData != nil {
			client.send <- blockData
		}

		// Start read/write goroutines
		go client.writePump()
		go client.readPump()
	}
}

// writePump sends message to client
func (c *WSClient) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		message, ok := <-c.send
		if !ok {
			// If channel closed, send normal close message
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			// Treat client disconnection as normal closure
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived) {
				logger.Error("WebSocket write error:", err)
			} else {
				logger.Debug("WebSocket write closed:", err)
			}
			return
		}
	}
}

// readPump receives message from client
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			// Do not log normal client closure as error
			// CloseGoingAway (1001): Browser tab close, page navigation
			// CloseNoStatusReceived (1005): Connection closed without status code (browser closed, etc.)
			// CloseNormalClosure (1000): Normal closure
			if websocket.IsUnexpectedCloseError(err, 
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived,
				websocket.CloseAbnormalClosure) {
				logger.Error("WebSocket read error:", err)
			} else {
				// Log normal closure only at Debug level
				logger.Debug("WebSocket client disconnected:", err)
			}
			break
		}
		// Handle client message (currently ignored)
	}
}
