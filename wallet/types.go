package wallet

import (
	prt "github.com/abcfe/abcfe-node/protocol"
)

// Existing keystore-related types
type CipherParams struct {
	IV string `json:"iv"` // Initialization vector
}

type KDFParams struct {
	DkLen int    `json:"dklen"` // Derived key length
	N     int    `json:"n"`     // CPU/Memory cost
	P     int    `json:"p"`     // Parallelization parameter
	R     int    `json:"r"`     // Block size
	Salt  string `json:"salt"`  // Salt
}

type Crypto struct {
	Cipher       string       `json:"cipher"`     // "aes-128-ctr"
	CipherText   string       `json:"ciphertext"` // Encrypted private key
	CipherParams CipherParams `json:"cipherparams"`
	KDF          string       `json:"kdf"` // "scrypt"
	KDFParams    KDFParams    `json:"kdfparams"`
	MAC          string       `json:"mac"` // Integrity check
}

// Mnemonic-based wallet types
type MnemonicWallet struct {
	Mnemonic     string     `json:"mnemonic"`      // 12/15/18/21/24 words
	Seed         []byte     `json:"seed"`          // Seed derived from mnemonic
	MasterKey    []byte     `json:"master_key"`    // Master private key (bytes)
	Accounts     []*Account `json:"accounts"`      // Derived accounts
	CurrentIndex int        `json:"current_index"` // Currently used account index
}

type Account struct {
	Index      int         `json:"index"`       // Account index (0, 1, 2...)
	Address    prt.Address `json:"address"`     // 20-byte address
	PrivateKey []byte      `json:"private_key"` // Private key (bytes)
	PublicKey  []byte      `json:"public_key"`  // Public key (bytes)
	Path       string      `json:"path"`        // BIP-44 path (m/44'/60'/0'/0/0)
	Unlocked   bool        `json:"unlocked"`    // Unlock status
}

// BIP-44 path constants
const (
	BIP44Purpose  = 44
	BIP44CoinType = 60 // Ethereum
	BIP44Account  = 0
	BIP44Change   = 0 // External
	BIP44Index    = 0
)
