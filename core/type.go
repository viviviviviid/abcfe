package core

const (
	TxTypeGeneral uint8 = iota
	TxTypeStaking
	TxTypeUnStaking
	TxTypeCoinbase // Coinbase 트랜잭션 (블록 보상 + 수수료)
	TxTypeEtc
)
