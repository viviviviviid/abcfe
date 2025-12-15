package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"math/big"
)

func GenerateKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	publicKey := privateKey.PublicKey
	return privateKey, &publicKey, err
}

// Derive master key from seed (simple version)
func DeriveMasterKey(seed []byte) (*ecdsa.PrivateKey, error) {
	// Hash seed with SHA256 to use as private key
	hash := sha256.Sum256(seed)

	// Convert hash to private key
	privateKey := new(ecdsa.PrivateKey)
	privateKey.PublicKey.Curve = elliptic.P256()
	privateKey.D = new(big.Int).SetBytes(hash[:])

	// Calculate public key
	privateKey.PublicKey.X, privateKey.PublicKey.Y = privateKey.PublicKey.Curve.ScalarBaseMult(hash[:])

	return privateKey, nil
}

// Derive account key from path (simple version)
func DeriveAccountKey(masterKey *ecdsa.PrivateKey, path string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	// Hash path to create unique offset per account
	pathHash := sha256.Sum256([]byte(path))

	// Add offset to master key
	offset := new(big.Int).SetBytes(pathHash[:])
	newD := new(big.Int).Add(masterKey.D, offset)
	newD.Mod(newD, masterKey.PublicKey.Curve.Params().N)

	// Create new private key
	privateKey := new(ecdsa.PrivateKey)
	privateKey.PublicKey.Curve = elliptic.P256()
	privateKey.D = newD

	// Calculate public key
	privateKey.PublicKey.X, privateKey.PublicKey.Y = privateKey.PublicKey.Curve.ScalarBaseMult(newD.Bytes())

	return privateKey, &privateKey.PublicKey, nil
}

// Helper function to convert private key to bytes
func PrivateKeyToBytes(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	if privateKey == nil {
		return nil, nil
	}
	return x509.MarshalECPrivateKey(privateKey)
}

// Helper function to convert bytes to private key
func BytesToPrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	if len(data) == 0 {
		return nil, nil
	}
	return x509.ParseECPrivateKey(data)
}
