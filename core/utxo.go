package core

import (
	"fmt"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
	"github.com/syndtr/goleveldb/leveldb"
)

type UTXOSet map[string]*UTXO    // Key: TxId + OutputIndex 문자열 조합
type AddrUTXOSet map[string]bool // user의 UTXO key 리스트
type UTXO struct {
	TxId        prt.Hash
	OutputIndex uint64
	TxOut       TxOutput
	Height      uint64
	Spent       bool // true : spent
	SpentHeight uint64
}

// UTXO 관련 접두사
func (p *BlockChain) UpdateUtxo(batch *leveldb.Batch, blk Block) error {
	for _, tx := range blk.Transactions {
		if blk.Header.Height > 0 { // Genesis Block은 output만 처리
			for _, input := range tx.Inputs {
				// 1. input으로 사용한 UTXO 사용처리
				utxoKey := utils.GetUtxoKey(input.TxID, int(input.OutputIndex))
				utxoBytes, err := p.db.Get(utxoKey, nil)
				if err != nil {
					return fmt.Errorf("failed to get utxo data from db: %w", err)
				}

				var utxo UTXO
				if err := utils.DeserializeData(utxoBytes, &utxo, utils.SerializationFormatGob); err != nil {
					return fmt.Errorf("failed to deserialize utxo data: %w", err)
				}

				// UTXO가 이미 사용되었는지 검증
				if utxo.Spent {
					return fmt.Errorf("UTXO already spent: %s:%d", utils.HashToString(input.TxID), input.OutputIndex)
				}

				utxo.Spent = true                    // utxo 사용처리
				utxo.SpentHeight = blk.Header.Height // 소비된 블록 높이

				utxoUpdBytes, err := utils.SerializeData(utxo, utils.SerializationFormatGob)
				if err != nil {
					return fmt.Errorf("failed to block serialization: %w", err)
				}

				batch.Put(utxoKey, utxoUpdBytes)

				// 2. 주소별 UTXO 리스트에서 해당 UTXO 제거
				utxoListKey := utils.GetUtxoListKey(utxo.TxOut.Address)
				utxoListBytes, err := p.db.Get(utxoListKey, nil)
				if err != nil {
					return fmt.Errorf("failed to get utxo data from db: %w", err)
				}

				var utxoList AddrUTXOSet
				if err := utils.DeserializeData(utxoListBytes, &utxoList, utils.SerializationFormatGob); err != nil {
					return fmt.Errorf("failed to deserialize utxo list: %w", err)
				}

				// Map에서 제거. O(1)
				delete(utxoList, string(utxoKey))

				updatedListBytes, err := utils.SerializeData(utxoList, utils.SerializationFormatGob)
				if err != nil {
					return fmt.Errorf("failed to serialize utxo list: %w", err)
				}
				batch.Put(utxoListKey, updatedListBytes)
			}
		}
	}

	// 2. OUTPUT 처리
	// 주소별로 UTXO 리스트를 한 번만 로드 => 덮어쓰기 안되도록
	addressUTXOMap := make(map[prt.Address]AddrUTXOSet)

	for _, tx := range blk.Transactions {
		for outputIndex, output := range tx.Outputs {
			// 주소별 UTXO 리스트가 이미 로드되었는지 확인
			utxoList, exists := addressUTXOMap[output.Address]
			if !exists {
				// 처음 나온 주소면 DB에서 로드
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

			// UTXO 생성 및 저장
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

			// 메모리의 리스트에 추가
			utxoList[string(utxoKey)] = true
		}
	}

	// 3. 모든 변경사항을 한 번에 저장
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

// 최종 balance는 mempool에 사용된 자금도 포함해야함.
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

		// spent key값 확인
		if utxo.Spent {
			continue // 사용한 UTXO
		}

		if mempoolCheck {
			if p.isOnMempool(utxo.TxId, utxo.OutputIndex) {
				continue // 멤풀에 있는 UTXO // 사용됨으로 간주
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

// GetUtxoByTxIdAndIdx TxID와 OutputIndex로 특정 UTXO 조회
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
