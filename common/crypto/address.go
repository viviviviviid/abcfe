package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"

	prt "github.com/abcfe/abcfe-node/protocol"
	"golang.org/x/crypto/sha3"
)

func PublicKeyToAddress(publicKey *ecdsa.PublicKey) (prt.Address, error) {
	// Convert public key to compressed format
	pubBytes := elliptic.MarshalCompressed(publicKey.Curve, publicKey.X, publicKey.Y)

	// Keccak256 hash
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubBytes[1:]) // Remove compression prefix
	hashBytes := hash.Sum(nil)

	// Convert last 20 bytes to Address type
	var address prt.Address
	copy(address[:], hashBytes[len(hashBytes)-20:])

	return address, nil
}

// Add 0x prefix to address
func AddressTo0xPrefixString(address prt.Address) string {
	return "0x" + hex.EncodeToString(address[:])
}
