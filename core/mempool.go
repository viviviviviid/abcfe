package core

import (
	"fmt"
	"sort"
	"sync"

	prt "github.com/abcfe/abcfe-node/protocol"

	"github.com/abcfe/abcfe-node/common/utils"
)

// TxWithFee stores transaction and fee information together
type TxWithFee struct {
	Tx  *Transaction
	Fee uint64 // Implicit fee (caching)
}

type Mempool struct {
	transactions map[string]*TxWithFee // Includes fee info
	mu           sync.RWMutex
}

func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]*TxWithFee),
	}
}

// NewTranaction adds transaction to mempool (with fee info)
func (p *Mempool) NewTranaction(tx *Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	txId := utils.HashToString(tx.ID)

	if _, exists := p.transactions[txId]; exists {
		return fmt.Errorf("tx already exists in mempool")
	}

	// Fee is set later in BlockChain.AddTxToMempool
	// Saved as 0 for now
	p.transactions[txId] = &TxWithFee{
		Tx:  tx,
		Fee: 0,
	}
	return nil
}

// NewTransactionWithFee adds transaction to mempool with fee info
func (p *Mempool) NewTransactionWithFee(tx *Transaction, fee uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	txId := utils.HashToString(tx.ID)

	if _, exists := p.transactions[txId]; exists {
		return fmt.Errorf("tx already exists in mempool")
	}

	p.transactions[txId] = &TxWithFee{
		Tx:  tx,
		Fee: fee,
	}
	return nil
}

func (p *Mempool) GetTx(txId prt.Hash) *Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	strTxId := utils.HashToString(txId)

	if txWithFee, exists := p.transactions[strTxId]; exists {
		return txWithFee.Tx
	}
	return nil
}

// GetTxs extracts transactions from mempool for block addition (sorted by fee descending)
func (p *Mempool) GetTxs() []*Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Convert to TxWithFee slice
	txsWithFee := make([]*TxWithFee, 0, len(p.transactions))
	for _, txWithFee := range p.transactions {
		txsWithFee = append(txsWithFee, txWithFee)
	}

	// Sort by fee descending
	sort.Slice(txsWithFee, func(i, j int) bool {
		return txsWithFee[i].Fee > txsWithFee[j].Fee
	})

	// Extract only Transaction
	txs := make([]*Transaction, 0, len(txsWithFee))
	for _, txWithFee := range txsWithFee {
		txs = append(txs, txWithFee.Tx)
	}

	// Limit tx count if exceeds max (prioritize higher fees)
	if len(txs) > prt.MaxTxsPerBlock {
		return txs[:prt.MaxTxsPerBlock]
	}

	return txs
}

// GetTxCount returns transaction count in mempool
func (p *Mempool) GetTxCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.transactions)
}

func (p *Mempool) DelTx(txId prt.Hash) {
	p.mu.Lock()
	defer p.mu.Unlock()

	strTxId := utils.HashToString(txId)

	delete(p.transactions, strTxId)
}

// Clear clears mempool
func (p *Mempool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.transactions = make(map[string]*TxWithFee)
}

// UpdateTxFee updates fee of transaction in mempool
func (p *Mempool) UpdateTxFee(txId prt.Hash, fee uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	strTxId := utils.HashToString(txId)
	if txWithFee, exists := p.transactions[strTxId]; exists {
		txWithFee.Fee = fee
	}
}

func (p *BlockChain) isOnMempool(txId prt.Hash, outputIndex uint64) bool {
	p.Mempool.mu.RLock()
	defer p.Mempool.mu.RUnlock()

	for _, txWithFee := range p.Mempool.transactions {
		for _, input := range txWithFee.Tx.Inputs {
			if input.OutputIndex == outputIndex && input.TxID == txId {
				return true
			}
		}
	}

	return false
}
