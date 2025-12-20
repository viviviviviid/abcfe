package p2p

import (
	"sync"
	"time"
)

// RateLimitConfig rate limiting configuration
type RateLimitConfig struct {
	// Per-peer limits (messages per second)
	MaxMessagesPerSecond int           // Default: 100 messages/sec
	BurstSize            int           // Burst allowance: 200
	BanDuration          time.Duration // Ban duration when exceeded: 60 seconds

	// Per-message type limits (messages per second)
	MaxBlocksPerSecond    int // GetBlocks requests: 5/sec
	MaxTxPerSecond        int // NewTx messages: 50/sec
	MaxProposalsPerSecond int // Proposal messages: 10/sec
	MaxVotesPerSecond     int // Vote messages: 50/sec
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		MaxMessagesPerSecond:  100,
		BurstSize:             200,
		BanDuration:           60 * time.Second,
		MaxBlocksPerSecond:    5,
		MaxTxPerSecond:        50,
		MaxProposalsPerSecond: 10,
		MaxVotesPerSecond:     50,
	}
}

// PeerRateLimiter tracks rate limits for a single peer
type PeerRateLimiter struct {
	mu sync.Mutex

	// Token bucket for overall messages
	tokens       float64
	maxTokens    float64
	refillRate   float64 // tokens per second
	lastRefill   time.Time

	// Message type counters (sliding window)
	messageCounters map[MessageType]*messageCounter

	// Ban status
	bannedUntil time.Time
}

// messageCounter tracks message count in a sliding window
type messageCounter struct {
	count      int
	windowStart time.Time
}

// RateLimiter manages rate limiting for all peers
type RateLimiter struct {
	mu sync.RWMutex

	config *RateLimitConfig
	peers  map[string]*PeerRateLimiter // key: peer address or ID

	// Cleanup
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		config:          config,
		peers:           make(map[string]*PeerRateLimiter),
		cleanupInterval: 5 * time.Minute,
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// cleanupLoop periodically removes stale peer entries
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

// cleanup removes stale peer rate limiters
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	staleThreshold := 10 * time.Minute

	for peerID, prl := range rl.peers {
		prl.mu.Lock()
		if now.Sub(prl.lastRefill) > staleThreshold {
			delete(rl.peers, peerID)
		}
		prl.mu.Unlock()
	}
}

// getPeerLimiter gets or creates a rate limiter for a peer
func (rl *RateLimiter) getPeerLimiter(peerID string) *PeerRateLimiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if prl, exists := rl.peers[peerID]; exists {
		return prl
	}

	prl := &PeerRateLimiter{
		tokens:          float64(rl.config.BurstSize),
		maxTokens:       float64(rl.config.BurstSize),
		refillRate:      float64(rl.config.MaxMessagesPerSecond),
		lastRefill:      time.Now(),
		messageCounters: make(map[MessageType]*messageCounter),
	}
	rl.peers[peerID] = prl
	return prl
}

// AllowMessage checks if a message from a peer is allowed
// Returns: (allowed bool, reason string)
func (rl *RateLimiter) AllowMessage(peerID string, msgType MessageType) (bool, string) {
	prl := rl.getPeerLimiter(peerID)
	prl.mu.Lock()
	defer prl.mu.Unlock()

	now := time.Now()

	// Check if peer is banned
	if now.Before(prl.bannedUntil) {
		return false, "peer is temporarily banned"
	}

	// Refill tokens (token bucket algorithm)
	elapsed := now.Sub(prl.lastRefill).Seconds()
	prl.tokens += elapsed * prl.refillRate
	if prl.tokens > prl.maxTokens {
		prl.tokens = prl.maxTokens
	}
	prl.lastRefill = now

	// Check overall rate limit
	if prl.tokens < 1 {
		// Rate exceeded - ban the peer
		prl.bannedUntil = now.Add(rl.config.BanDuration)
		return false, "overall rate limit exceeded, peer banned"
	}

	// Check message type specific limits
	if !rl.checkMessageTypeLimit(prl, msgType, now) {
		return false, "message type rate limit exceeded"
	}

	// Consume a token
	prl.tokens--

	return true, ""
}

// checkMessageTypeLimit checks rate limit for specific message type
func (rl *RateLimiter) checkMessageTypeLimit(prl *PeerRateLimiter, msgType MessageType, now time.Time) bool {
	// Get limit for this message type
	limit := rl.getMessageTypeLimit(msgType)
	if limit == 0 {
		return true // No limit for this type
	}

	// Get or create counter
	counter, exists := prl.messageCounters[msgType]
	if !exists {
		counter = &messageCounter{
			count:       0,
			windowStart: now,
		}
		prl.messageCounters[msgType] = counter
	}

	// Reset window if expired (1 second window)
	if now.Sub(counter.windowStart) >= time.Second {
		counter.count = 0
		counter.windowStart = now
	}

	// Check limit
	if counter.count >= limit {
		return false
	}

	counter.count++
	return true
}

// getMessageTypeLimit returns the rate limit for a message type
func (rl *RateLimiter) getMessageTypeLimit(msgType MessageType) int {
	switch msgType {
	case MsgTypeGetBlocks:
		return rl.config.MaxBlocksPerSecond
	case MsgTypeNewTx:
		return rl.config.MaxTxPerSecond
	case MsgTypeProposal:
		return rl.config.MaxProposalsPerSecond
	case MsgTypeVote:
		return rl.config.MaxVotesPerSecond
	default:
		return 0 // No specific limit
	}
}

// IsBanned checks if a peer is currently banned
func (rl *RateLimiter) IsBanned(peerID string) bool {
	rl.mu.RLock()
	prl, exists := rl.peers[peerID]
	rl.mu.RUnlock()

	if !exists {
		return false
	}

	prl.mu.Lock()
	defer prl.mu.Unlock()

	return time.Now().Before(prl.bannedUntil)
}

// GetPeerStats returns rate limit stats for a peer
func (rl *RateLimiter) GetPeerStats(peerID string) map[string]interface{} {
	rl.mu.RLock()
	prl, exists := rl.peers[peerID]
	rl.mu.RUnlock()

	if !exists {
		return nil
	}

	prl.mu.Lock()
	defer prl.mu.Unlock()

	stats := map[string]interface{}{
		"tokens":      prl.tokens,
		"maxTokens":   prl.maxTokens,
		"isBanned":    time.Now().Before(prl.bannedUntil),
		"bannedUntil": prl.bannedUntil,
	}

	return stats
}
