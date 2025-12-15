package core

import (
	"fmt"
	"time"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

// Account status constants
const (
	AccountStatusNormal     = "NORMAL"
	AccountStatusStaking    = "STAKING"
	AccountStatusValidating = "VALIDATING"
	AccountStatusProposing  = "PROPOSING"
	AccountStatusJailed     = "JAILED"
)

type Account struct {
	Address   prt.Address `json:"address"`   // Account address
	Status    string      `json:"status"`    // Account status
	Balance   uint64      `json:"balance"`   // Balance
	CreatedAt int64       `json:"createdAt"` // Account creation time
	UpdatedAt int64       `json:"updatedAt"` // Last updated time
}

// AccountTxList transaction list of account
type AccountTxList struct {
	TxHashes []prt.Hash `json:"txHashes"`
}

// GetAccount gets account info
func (p *BlockChain) GetAccount(address prt.Address) (*Account, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := []byte(prt.PrefixAddress + utils.AddressToString(address))
	data, err := p.db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, nil // Account not found
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	var account Account
	if err := utils.DeserializeData(data, &account, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize account: %w", err)
	}

	return &account, nil
}

// CreateAccount creates new account
func (p *BlockChain) CreateAccount(address prt.Address) (*Account, error) {
	now := time.Now().Unix()
	account := &Account{
		Address:   address,
		Status:    AccountStatusNormal,
		Balance:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := p.saveAccount(account); err != nil {
		return nil, err
	}

	return account, nil
}

// GetOrCreateAccount gets or creates account
func (p *BlockChain) GetOrCreateAccount(address prt.Address) (*Account, error) {
	account, err := p.GetAccount(address)
	if err != nil {
		return nil, err
	}

	if account == nil {
		return p.CreateAccount(address)
	}

	return account, nil
}

// saveAccount saves account info
func (p *BlockChain) saveAccount(account *Account) error {
	key := []byte(prt.PrefixAddress + utils.AddressToString(account.Address))
	data, err := utils.SerializeData(account, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize account: %w", err)
	}

	if err := p.db.Put(key, data, nil); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	return nil
}

// AddAccountTx adds transaction to account tx list
func (p *BlockChain) AddAccountTx(address prt.Address, txHash prt.Hash) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := []byte(prt.PrefixAddressTxs + utils.AddressToString(address))

	// Get existing list
	var txList AccountTxList
	data, err := p.db.Get(key, nil)
	if err != nil && err != leveldb.ErrNotFound {
		return fmt.Errorf("failed to get tx list: %w", err)
	}

	if err == nil {
		if err := utils.DeserializeData(data, &txList, utils.SerializationFormatGob); err != nil {
			return fmt.Errorf("failed to deserialize tx list: %w", err)
		}
	}

	// Add new transaction
	txList.TxHashes = append(txList.TxHashes, txHash)

	// Save
	newData, err := utils.SerializeData(txList, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize tx list: %w", err)
	}

	if err := p.db.Put(key, newData, nil); err != nil {
		return fmt.Errorf("failed to save tx list: %w", err)
	}

	return nil
}

// AddAccountTxReceived adds received transaction
func (p *BlockChain) AddAccountTxReceived(address prt.Address, txHash prt.Hash) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := []byte(prt.PrefixAddressReceived + utils.AddressToString(address))

	var txList AccountTxList
	data, err := p.db.Get(key, nil)
	if err != nil && err != leveldb.ErrNotFound {
		return fmt.Errorf("failed to get received tx list: %w", err)
	}

	if err == nil {
		if err := utils.DeserializeData(data, &txList, utils.SerializationFormatGob); err != nil {
			return fmt.Errorf("failed to deserialize received tx list: %w", err)
		}
	}

	txList.TxHashes = append(txList.TxHashes, txHash)

	newData, err := utils.SerializeData(txList, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize received tx list: %w", err)
	}

	if err := p.db.Put(key, newData, nil); err != nil {
		return fmt.Errorf("failed to save received tx list: %w", err)
	}

	return nil
}

// AddAccountTxSent adds sent transaction
func (p *BlockChain) AddAccountTxSent(address prt.Address, txHash prt.Hash) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := []byte(prt.PrefixAddressSent + utils.AddressToString(address))

	var txList AccountTxList
	data, err := p.db.Get(key, nil)
	if err != nil && err != leveldb.ErrNotFound {
		return fmt.Errorf("failed to get sent tx list: %w", err)
	}

	if err == nil {
		if err := utils.DeserializeData(data, &txList, utils.SerializationFormatGob); err != nil {
			return fmt.Errorf("failed to deserialize sent tx list: %w", err)
		}
	}

	txList.TxHashes = append(txList.TxHashes, txHash)

	newData, err := utils.SerializeData(txList, utils.SerializationFormatGob)
	if err != nil {
		return fmt.Errorf("failed to serialize sent tx list: %w", err)
	}

	if err := p.db.Put(key, newData, nil); err != nil {
		return fmt.Errorf("failed to save sent tx list: %w", err)
	}

	return nil
}

// UpdateAccountBalance updates balance based on UTXO
func (p *BlockChain) UpdateAccountBalance(address prt.Address) error {
	// Get or create account
	account, err := p.GetOrCreateAccount(address)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Calculate balance based on UTXO
	utxoList, err := p.GetUtxoList(address, false)
	if err != nil {
		return fmt.Errorf("failed to get utxo list: %w", err)
	}

	account.Balance = p.CalBalanceUtxo(utxoList)
	account.UpdatedAt = time.Now().Unix()

	return p.saveAccount(account)
}

// UpdateAccountStatus updates account status
func (p *BlockChain) UpdateAccountStatus(address prt.Address, status string) error {
	account, err := p.GetOrCreateAccount(address)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	account.Status = status
	account.UpdatedAt = time.Now().Unix()

	return p.saveAccount(account)
}

// GetAccountTxList gets all transactions of account
func (p *BlockChain) GetAccountTxList(address prt.Address) ([]prt.Hash, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := []byte(prt.PrefixAddressTxs + utils.AddressToString(address))
	data, err := p.db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return []prt.Hash{}, nil
		}
		return nil, fmt.Errorf("failed to get tx list: %w", err)
	}

	var txList AccountTxList
	if err := utils.DeserializeData(data, &txList, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize tx list: %w", err)
	}

	return txList.TxHashes, nil
}

// GetAccountReceivedTxList gets received transaction list
func (p *BlockChain) GetAccountReceivedTxList(address prt.Address) ([]prt.Hash, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := []byte(prt.PrefixAddressReceived + utils.AddressToString(address))
	data, err := p.db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return []prt.Hash{}, nil
		}
		return nil, fmt.Errorf("failed to get received tx list: %w", err)
	}

	var txList AccountTxList
	if err := utils.DeserializeData(data, &txList, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize received tx list: %w", err)
	}

	return txList.TxHashes, nil
}

// GetAccountSentTxList gets sent transaction list
func (p *BlockChain) GetAccountSentTxList(address prt.Address) ([]prt.Hash, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := []byte(prt.PrefixAddressSent + utils.AddressToString(address))
	data, err := p.db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return []prt.Hash{}, nil
		}
		return nil, fmt.Errorf("failed to get sent tx list: %w", err)
	}

	var txList AccountTxList
	if err := utils.DeserializeData(data, &txList, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize sent tx list: %w", err)
	}

	return txList.TxHashes, nil
}

// GetBalance gets balance based on UTXO
func (p *BlockChain) GetBalance(address prt.Address) (uint64, error) {
	utxoList, err := p.GetUtxoList(address, false)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	balance := p.CalBalanceUtxo(utxoList)
	return balance, nil
}
