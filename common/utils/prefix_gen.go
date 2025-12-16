package utils

import (
	"strconv"

	prt "github.com/abcfe/abcfe-node/protocol"
)

// "blk:h:"
func GetBlockHeightKey(height uint64) []byte {
	hStr := Uint64ToString(height)
	hKey := []byte(prt.PrefixBlockByHeight + hStr)
	return hKey
}

// "blk:"
func GetBlockHashKey(hash prt.Hash) []byte {
	blkHashStr := HashToString(hash)
	blkKey := []byte(prt.PrefixBlock + blkHashStr)
	return blkKey
}

// "tx:"
func GetTxHashKey(txHash prt.Hash) []byte {
	txHashStr := HashToString(txHash)
	txKey := []byte(prt.PrefixTxs + txHashStr)
	return txKey
}

// "tx:status:"
func GetTxStatusKey(txHash prt.Hash) []byte {
	txHashStr := HashToString(txHash)
	txKey := []byte(prt.PrefixTxStatus + txHashStr)
	return txKey
}

// "tx:blk:"
func GetTxBlockHashKey(txHash prt.Hash) []byte {
	txHashStr := HashToString(txHash)
	txKey := []byte(prt.PrefixTxBlock + txHashStr)
	return txKey
}

// "tx:in:"
// [Usage Pattern 1] tx:in:TxHash:Index = Specific input data
// [Usage Pattern 2] tx:in:TxHash = All input data list
func GetTxInputKey(txHash prt.Hash, index int) []byte {
	txHashStr := HashToString(txHash)
	if index >= 0 { // Distinguish specific / all by index existence
		return []byte(prt.PrefixTxIn + txHashStr + ":" + strconv.Itoa(index))
	}
	return []byte(prt.PrefixTxIn + txHashStr) // Access all input data
}

// "tx:out:"
// [Usage Pattern 1] tx:out:TxHash:Index = Specific output data
// [Usage Pattern 2] tx:out:TxHash = All output data list
func GetTxOutputKey(txHash prt.Hash, index int) []byte {
	txHashStr := HashToString(txHash)
	if index >= 0 { // Distinguish specific / all by index existence
		return []byte(prt.PrefixTxOut + txHashStr + ":" + strconv.Itoa(index))
	}
	return []byte(prt.PrefixTxOut + txHashStr) // Access all output data
}

// "utxo:txhash:outputindex"
func GetUtxoKey(txHash prt.Hash, outputIndex int) []byte {
	txHashStr := HashToString(txHash)
	utxoKey := []byte(prt.PrefixUtxo + txHashStr + ":" + strconv.Itoa(outputIndex))
	return utxoKey
}

// "utxo:addr:"
func GetUtxoListKey(address prt.Address) []byte {
	addressStr := AddressToString(address)
	utxoListKey := []byte(prt.PrefixUtxoList + addressStr)
	return utxoListKey
}

// "utxo:bal:"
func GetUtxoBalanceKey(address prt.Address) []byte {
	addressStr := AddressToString(address)
	utxoBalanceKey := []byte(prt.PrefixUtxoBalance + addressStr)
	return utxoBalanceKey
}
