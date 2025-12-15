package rest

// General response structure
type RestResp struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Blockchain status response
type BlockchainStatResp struct {
	Height    uint64 `json:"height"`
	BlockHash string `json:"blockHash"`
}

// Block response
type BlockResp struct {
	Header           BlockHeaderResp       `json:"header"`
	Transactions     []TxResp              `json:"transactions"`               // Transaction ID list
	Proposer         string                `json:"proposer"`                   // Block proposer address
	Signature        string                `json:"signature"`                  // Proposer signature
	CommitSignatures []CommitSignatureResp `json:"commitSignatures,omitempty"` // BFT validator signatures
}

// BFT commit signature response
type CommitSignatureResp struct {
	ValidatorAddress string `json:"validatorAddress"` // Validator address
	Signature        string `json:"signature"`        // Validator signature
	// Timestamp        int64  `json:"timestamp"`        // Signature time
}

type BlockHeaderResp struct {
	Hash       string `json:"hash"`
	PrevHash   string `json:"prevHash"`   // Previous block hash
	Version    string `json:"version"`    // Blockchain protocol version
	Height     uint64 `json:"height"`     // Block height (changed to uint64)
	MerkleRoot string `json:"merkleRoot"` // Transaction Merkle root
	Timestamp  int64  `json:"timestamp"`  // Block creation time (Unix timestamp)
	// StateRoot  Hash   `json:"stateRoot"`  // State Merkle root (UTXO or account state)
}

// Transaction response
type TxResp struct {
	ID        string        `json:"id"`
	Version   string        `json:"version"`
	Timestamp int64         `json:"timestamp"`
	Inputs    []interface{} `json:"inputs"`
	Outputs   []interface{} `json:"outputs"`
	Memo      string        `json:"memo"`
	Fee       uint64        `json:"fee"` // Implicit fee (InputSum - OutputSum)
}

type SubmitTxReq struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
	Fee    uint64 `json:"fee"` // Fee (optional, minimum fee applies if 0)
	Memo   string `json:"memo"`
	Data   []byte `json:"data"`
}

// Submit signed transaction request (signed by client)
type SubmitSignedTxReq struct {
	Version   string          `json:"version"`
	Timestamp int64           `json:"timestamp"`
	Inputs    []SignedTxInput `json:"inputs"`
	Outputs   []TxOutputReq   `json:"outputs"`
	Memo      string          `json:"memo"`
	Data      []byte          `json:"data"`
}

type SignedTxInput struct {
	TxID        string `json:"txId"` // hex string
	OutputIndex uint64 `json:"outputIndex"`
	Signature   string `json:"signature"` // hex string
	PublicKey   string `json:"publicKey"` // hex string
}

type TxOutputReq struct {
	Address string `json:"address"` // hex string
	Amount  uint64 `json:"amount"`
	TxType  uint8  `json:"txType"`
}

// Send request using server wallet
type SendTxReq struct {
	AccountIndex int    `json:"accountIndex"` // Wallet account index (default 0)
	To           string `json:"to"`
	Amount       uint64 `json:"amount"`
	Fee          uint64 `json:"fee"` // Fee (optional, minimum fee applies if 0)
	Memo         string `json:"memo"`
	Data         []byte `json:"data"`
}

// Wallet account response
type WalletAccountResp struct {
	Index   int    `json:"index"`
	Address string `json:"address"`
	Path    string `json:"path"`
}
