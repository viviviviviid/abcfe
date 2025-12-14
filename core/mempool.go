package core

import (
	"fmt"
	"sort"
	"sync"

	prt "github.com/abcfe/abcfe-node/protocol"

	"github.com/abcfe/abcfe-node/common/utils"
)

// TxWithFee 트랜잭션과 수수료 정보를 함께 저장
type TxWithFee struct {
	Tx  *Transaction
	Fee uint64 // 암묵적 수수료 (캐싱)
}

type Mempool struct {
	transactions map[string]*TxWithFee // 수수료 정보 포함
	mu           sync.RWMutex
}

func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]*TxWithFee),
	}
}

// NewTransaction 트랜잭션을 mempool에 추가 (수수료 정보와 함께)
func (p *Mempool) NewTranaction(tx *Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	txId := utils.HashToString(tx.ID)

	if _, exists := p.transactions[txId]; exists {
		return fmt.Errorf("tx already exists in mempool")
	}

	// 수수료는 나중에 BlockChain.AddTxToMempool에서 설정됨
	// 여기서는 일단 0으로 저장
	p.transactions[txId] = &TxWithFee{
		Tx:  tx,
		Fee: 0,
	}
	return nil
}

// NewTransactionWithFee 수수료 정보와 함께 트랜잭션을 mempool에 추가
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

// GetTxs 블록 추가시 멤풀에서 트랜잭션 추출 (수수료 높은 순으로 정렬)
func (p *Mempool) GetTxs() []*Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// TxWithFee 슬라이스로 변환
	txsWithFee := make([]*TxWithFee, 0, len(p.transactions))
	for _, txWithFee := range p.transactions {
		txsWithFee = append(txsWithFee, txWithFee)
	}

	// 수수료 높은 순으로 정렬 (내림차순)
	sort.Slice(txsWithFee, func(i, j int) bool {
		return txsWithFee[i].Fee > txsWithFee[j].Fee
	})

	// Transaction만 추출
	txs := make([]*Transaction, 0, len(txsWithFee))
	for _, txWithFee := range txsWithFee {
		txs = append(txs, txWithFee.Tx)
	}

	// 트랜잭션 수가 최대값보다 많으면 제한 (수수료 높은 것들 우선)
	if len(txs) > prt.MaxTxsPerBlock {
		return txs[:prt.MaxTxsPerBlock]
	}

	return txs
}

// GetTxCount mempool의 트랜잭션 개수 반환
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

// Clear Mempool 초기화
func (p *Mempool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.transactions = make(map[string]*TxWithFee)
}

// UpdateTxFee mempool에 있는 트랜잭션의 수수료 업데이트
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
