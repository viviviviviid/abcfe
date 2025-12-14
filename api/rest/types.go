package rest

// 일반적인 응답 구조체
type RestResp struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// 블록체인 상태 응답
type BlockchainStatResp struct {
	Height    uint64 `json:"height"`
	BlockHash string `json:"blockHash"`
}

// 블록 응답
type BlockResp struct {
	Header           BlockHeaderResp       `json:"header"`
	Transactions     []TxResp              `json:"transactions"`               // 트랜잭션 ID 목록
	Proposer         string                `json:"proposer"`                   // 블록 제안자 주소
	Signature        string                `json:"signature"`                  // 제안자 서명
	CommitSignatures []CommitSignatureResp `json:"commitSignatures,omitempty"` // BFT 검증자 서명들
}

// BFT 커밋 서명 응답
type CommitSignatureResp struct {
	ValidatorAddress string `json:"validatorAddress"` // 검증자 주소
	Signature        string `json:"signature"`        // 검증자 서명
	// Timestamp        int64  `json:"timestamp"`        // 서명 시간
}

type BlockHeaderResp struct {
	Hash       string `json:"hash"`
	PrevHash   string `json:"prevHash"`   // 이전 블록 해시
	Version    string `json:"version"`    // 블록체인 프로토콜 버전
	Height     uint64 `json:"height"`     // 블록 높이 (uint64로 변경)
	MerkleRoot string `json:"merkleRoot"` // 트랜잭션 머클 루트
	Timestamp  int64  `json:"timestamp"`  // 블록 생성 시간 (Unix 타임스탬프)
	// StateRoot  Hash   `json:"stateRoot"`  // 상태 머클 루트 (UTXO 또는 계정 상태)
}

// 트랜잭션 응답
type TxResp struct {
	ID        string        `json:"id"`
	Version   string        `json:"version"`
	Timestamp int64         `json:"timestamp"`
	Inputs    []interface{} `json:"inputs"`
	Outputs   []interface{} `json:"outputs"`
	Memo      string        `json:"memo"`
	Fee       uint64        `json:"fee"` // 암묵적 수수료 (InputSum - OutputSum)
}

type SubmitTxReq struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
	Fee    uint64 `json:"fee"` // 수수료 (선택, 0이면 최소 수수료 적용)
	Memo   string `json:"memo"`
	Data   []byte `json:"data"`
}

// 서명된 트랜잭션 제출 요청 (클라이언트가 서명)
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

// 서버 지갑으로 전송 요청
type SendTxReq struct {
	AccountIndex int    `json:"accountIndex"` // 지갑 계정 인덱스 (기본 0)
	To           string `json:"to"`
	Amount       uint64 `json:"amount"`
	Fee          uint64 `json:"fee"` // 수수료 (선택, 0이면 최소 수수료 적용)
	Memo         string `json:"memo"`
	Data         []byte `json:"data"`
}

// 지갑 계정 응답
type WalletAccountResp struct {
	Index   int    `json:"index"`
	Address string `json:"address"`
	Path    string `json:"path"`
}
