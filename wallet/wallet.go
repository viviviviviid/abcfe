package wallet

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"

	"github.com/abcfe/abcfe-node/common/crypto"
	"github.com/abcfe/abcfe-node/common/utils"
	"github.com/abcfe/abcfe-node/core"
	prt "github.com/abcfe/abcfe-node/protocol"
)

type Wallet struct {
	Address       prt.Address       // 20-byte address
	PrivateKey    *ecdsa.PrivateKey // ECDSA private key
	PublicKey     *ecdsa.PublicKey  // ECDSA public key
	WalletManager *WalletManager
}

func NewWallet(keystore *WalletManager) *Wallet {
	return &Wallet{
		WalletManager: keystore,
	}
}

// Create new wallet
func (w *Wallet) CreateWallet() error {
	privateKey, publicKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	w.PrivateKey = privateKey
	w.PublicKey = publicKey
	w.Address, err = crypto.PublicKeyToAddress(publicKey)
	if err != nil {
		return fmt.Errorf("failed to convert publicKey to address: %w", err)
	}

	return nil
}

// Sign transaction
func (w *Wallet) SignTransaction(tx *core.Transaction) (*prt.Signature, error) {
	if w.PrivateKey == nil {
		return nil, fmt.Errorf("wallet not unlocked")
	}

	// Calculate transaction hash
	txHash := utils.Hash(tx)
	txHashBytes := utils.HashToBytes(txHash)

	// ECDSA signature
	signature, err := ecdsa.SignASN1(rand.Reader, w.PrivateKey, txHashBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	var sig prt.Signature
	copy(sig[:], signature)

	return &sig, nil
}

// Verify signature
func (w *Wallet) VerifySignature(tx *core.Transaction, sig *prt.Signature, address prt.Address) bool {
	// Calculate transaction hash
	txHash := utils.Hash(tx)
	txHashBytes := utils.HashToBytes(txHash)

	// Verify ECDSA signature
	return ecdsa.VerifyASN1(w.PublicKey, txHashBytes, sig[:])
}

// Create transaction
func (w *Wallet) CreateTransaction() (*core.Transaction, error) {
	// TODO: Implement transaction creation logic
	return nil, fmt.Errorf("not implemented yet")
}
