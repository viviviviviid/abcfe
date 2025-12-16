package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"fmt"

	prt "github.com/abcfe/abcfe-node/protocol"
)

// SignData performs ECDSA signature on data
func SignData(privateKey *ecdsa.PrivateKey, data []byte) (prt.Signature, error) {
	var sig prt.Signature

	if privateKey == nil {
		return sig, fmt.Errorf("private key is nil")
	}

	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, data)
	if err != nil {
		return sig, fmt.Errorf("failed to sign data: %w", err)
	}

	if len(signature) > len(sig) {
		return sig, fmt.Errorf("signature too long: %d bytes", len(signature))
	}

	copy(sig[:], signature)
	return sig, nil
}

// VerifySignature verifies ECDSA signature
func VerifySignature(publicKey *ecdsa.PublicKey, data []byte, sig prt.Signature) bool {
	if publicKey == nil {
		return false
	}

	// Parse actual signature length from ASN.1 DER format
	// DER: 30 <len> <r> <s>
	// 30 = SEQUENCE tag
	// <len> = signature data length (r + s parts)
	if len(sig) < 2 || sig[0] != 0x30 {
		return false
	}

	// Actual signature length = 2 (tag + length) + content length
	sigLen := 2 + int(sig[1])
	if sigLen > len(sig) {
		return false
	}

	return ecdsa.VerifyASN1(publicKey, data, sig[:sigLen])
}

// VerifySignatureWithBytes verifies signature with byte public key
func VerifySignatureWithBytes(publicKeyBytes []byte, data []byte, sig prt.Signature) (bool, error) {
	if len(publicKeyBytes) == 0 {
		return false, fmt.Errorf("public key bytes is empty")
	}

	pub, err := x509.ParsePKIXPublicKey(publicKeyBytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return false, fmt.Errorf("not an ECDSA public key")
	}

	return VerifySignature(publicKey, data, sig), nil
}

// PublicKeyToBytes converts public key to bytes
func PublicKeyToBytes(publicKey *ecdsa.PublicKey) ([]byte, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	return x509.MarshalPKIXPublicKey(publicKey)
}

// BytesToPublicKey converts bytes to public key
func BytesToPublicKey(data []byte) (*ecdsa.PublicKey, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("public key bytes is empty")
	}

	pub, err := x509.ParsePKIXPublicKey(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}

	return publicKey, nil
}
