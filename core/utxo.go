package core

import (
	"fmt"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

type UTXOSet map[string]*UTXO    // Key: TxId + OutputIndex string combination
type AddrUTXOSet map[string]bool // user's UTXO key list
type UTXO struct {
	TxId        prt.Hash
	OutputIndex uint64
	TxOut       TxOutput
	Height      uint64
	Spent       bool // true : spent
	SpentHeight uint64
}

// Update UTXO
func (p *BlockChain) UpdateUtxo(batch *leveldb.Batch, blk Block) error {
	for _, tx := range blk.Transactions {
		if blk.Header.Height > 0 { // Genesis Block processes only output
			for _, input := range tx.Inputs {
				// 1. Mark input UTXO as spent
				utxoKey := utils.GetUtxoKey(input.TxID, int(input.OutputIndex))
				utxoBytes, err := p.db.Get(utxoKey, nil)
				if err != nil {
					return fmt.Errorf("failed to get utxo data from db: %w", err)
				}

				var utxo UTXO
				if err := utils.DeserializeData(utxoBytes, &utxo, utils.SerializationFormatGob); err != nil {
					return fmt.Errorf("failed to deserialize utxo data: %w", err)
				}

				// Verify if UTXO is already spent
				if utxo.Spent {
					return fmt.Errorf("UTXO already spent: %s:%d", utils.HashToString(input.TxID), input.OutputIndex)
				}

				utxo.Spent = true                    // Mark UTXO as spent
				utxo.SpentHeight = blk.Header.Height // Height of block where spent

				utxoUpdBytes, err := utils.SerializeData(utxo, utils.SerializationFormatGob)
				if err != nil {
					return fmt.Errorf("failed to block serialization: %w", err)
				}

				batch.Put(utxoKey, utxoUpdBytes)

				// 2. Remove UTXO from address UTXO list
				utxoListKey := utils.GetUtxoListKey(utxo.TxOut.Address)
				utxoListBytes, err := p.db.Get(utxoListKey, nil)
				if err != nil {
					return fmt.Errorf("failed to get utxo data from db: %w", err)
				}

				var utxoList AddrUTXOSet
				if err := utils.DeserializeData(utxoListBytes, &utxoList, utils.SerializationFormatGob); err != nil {
					return fmt.Errorf("failed to deserialize utxo list: %w", err)
				}

				// Remove from Map. O(1)
				delete(utxoList, string(utxoKey))

				updatedListBytes, err := utils.SerializeData(utxoList, utils.SerializationFormatGob)
				if err != nil {
					return fmt.Errorf("failed to serialize utxo list: %w", err)
				}
				batch.Put(utxoListKey, updatedListBytes)
			}
		}
	}

	// 2. Process OUTPUT
	// Load UTXO list per address once => prevent overwriting
	addressUTXOMap := make(map[prt.Address]AddrUTXOSet)

	for _, tx := range blk.Transactions {
		for outputIndex, output := range tx.Outputs {
			// Check if address UTXO list is already loaded
			utxoList, exists := addressUTXOMap[output.Address]
			if !exists {
				// Load from DB if first time address
				utxoListKey := utils.GetUtxoListKey(output.Address)
				utxoListBytes, err := p.db.Get(utxoListKey, nil)
				if err == nil {
					if err := utils.DeserializeData(utxoListBytes, &utxoList, utils.SerializationFormatGob); err != nil {
						return fmt.Errorf("failed to deserialize utxo list: %w", err)
					}
				} else if err != leveldb.ErrNotFound {
					return fmt.Errorf("failed to get utxo list: %w", err)
				} else {
					utxoList = make(AddrUTXOSet)
				}
				addressUTXOMap[output.Address] = utxoList
			}

			// Create and save UTXO
			newUtxo := UTXO{
				TxId:        tx.ID,
				OutputIndex: uint64(outputIndex),
				TxOut:       *output,
				Height:      blk.Header.Height,
				Spent:       false,
				SpentHeight: 0,
			}

			utxoKey := utils.GetUtxoKey(tx.ID, outputIndex)
			utxoBytes, err := utils.SerializeData(newUtxo, utils.SerializationFormatGob)
			if err != nil {
				return fmt.Errorf("failed to serialize utxo: %w", err)
			}
			batch.Put(utxoKey, utxoBytes)

			// Add to memory list
			utxoList[string(utxoKey)] = true
		}
	}

	// 3. Save all changes at once
	for address, utxoList := range addressUTXOMap {
		utxoListKey := utils.GetUtxoListKey(address)
		updatedListBytes, err := utils.SerializeData(utxoList, utils.SerializationFormatGob)
		if err != nil {
			return fmt.Errorf("failed to serialize utxo list: %w", err)
		}
		batch.Put(utxoListKey, updatedListBytes)
	}

	return nil
}

// Final balance should include funds used in mempool.
func (p *BlockChain) GetUtxoList(address prt.Address, mempoolCheck bool) ([]*UTXO, error) {
	utxoListKey := utils.GetUtxoListKey(address)
	utxoListBytes, err := p.db.Get(utxoListKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get utxo data from db: %w", err)
	}

	var utxoList AddrUTXOSet
	if err := utils.DeserializeData(utxoListBytes, &utxoList, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize utxo list: %w", err)
	}

	var result []*UTXO
	for utxoKey := range utxoList {
		utxoBytes, err := p.db.Get([]byte(utxoKey), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get utxo data from db: %w", err)
		}

		var utxo UTXO
		if err := utils.DeserializeData(utxoBytes, &utxo, utils.SerializationFormatGob); err != nil {
			return nil, fmt.Errorf("failed to deserialize utxo list: %w", err)
		}

		// Check spent key value
		if utxo.Spent {
			continue // Spent UTXO
		}

		if mempoolCheck {
			if p.isOnMempool(utxo.TxId, utxo.OutputIndex) {
				continue // UTXO in mempool // Considered spent
			}
		}

		result = append(result, &utxo)
	}

	return result, nil
}

func (p *BlockChain) CalBalanceUtxo(utxos []*UTXO) uint64 {
	var amount uint64
	for _, utxo := range utxos {
		if !utxo.Spent {
			amount += utxo.TxOut.Amount
		}
	}
	return amount
}

// GetUtxoByTxIdAndIdx gets specific UTXO by TxID and OutputIndex
func (p *BlockChain) GetUtxoByTxIdAndIdx(txId prt.Hash, outputIndex uint64) (*UTXO, error) {
	utxoKey := utils.GetUtxoKey(txId, int(outputIndex))
	utxoBytes, err := p.db.Get(utxoKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get utxo data from db: %w", err)
	}

	var utxo UTXO
	if err := utils.DeserializeData(utxoBytes, &utxo, utils.SerializationFormatGob); err != nil {
		return nil, fmt.Errorf("failed to deserialize utxo data: %w", err)
	}

	return &utxo, nil
}
