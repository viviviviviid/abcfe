package core

const (
	TxTypeGeneral uint8 = iota
	TxTypeStaking
	TxTypeUnStaking
	TxTypeCoinbase // Coinbase transaction (block reward + fee)
	TxTypeEtc
)
