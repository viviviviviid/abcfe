package utils

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	prt "github.com/abcfe/abcfe-node/protocol"
)

// HashToString converts Hash type to hex string
func HashToString(hash prt.Hash) string {
	return hex.EncodeToString(hash[:])
}

// StringToHash converts hex string to Hash type
func StringToHash(str string) (prt.Hash, error) {
	// Remove 0x prefix
	if len(str) >= 2 && str[0:2] == "0x" {
		str = str[2:]
	}
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return prt.Hash{}, fmt.Errorf("invalid hash string: %v", err)
	}

	// Validate hash length
	if len(bytes) != 32 {
		return prt.Hash{}, fmt.Errorf("invalid hash length: %d (need 32 bytes)", len(bytes))
	}

	var hash prt.Hash
	copy(hash[:], bytes)
	return hash, nil
}

// BytesToHash converts byte array to Hash type
func BytesToHash(bytes []byte) prt.Hash {
	return prt.Hash(bytes)
}

// HashToBytes converts Hash type to byte array
func HashToBytes(hash prt.Hash) []byte {
	bytes := make([]byte, len(hash))
	copy(bytes, hash[:])
	return bytes
}

// AddressToString converts Address type to hex string
func AddressToString(address prt.Address) string {
	return hex.EncodeToString(address[:])
}

// StringToAddress converts hex string to Address type
func StringToAddress(str string) (prt.Address, error) {
	// Remove 0x prefix
	if len(str) >= 2 && str[0:2] == "0x" {
		str = str[2:]
	}
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return prt.Address{}, fmt.Errorf("invalid address string: %v", err)
	}

	// Validate address length
	if len(bytes) != 20 {
		return prt.Address{}, fmt.Errorf("invalid address length: %d (20 bytes required)", len(bytes))
	}

	var address prt.Address
	copy(address[:], bytes)
	return address, nil
}

// SignatureToString converts Signature type to hex string
func SignatureToString(sig prt.Signature) string {
	return hex.EncodeToString(sig[:])
}

// StringToSignature converts hex string to Signature type
func StringToSignature(str string) (prt.Signature, error) {
	// Remove 0x prefix
	if len(str) >= 2 && str[0:2] == "0x" {
		str = str[2:]
	}
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return prt.Signature{}, fmt.Errorf("invalid signature string: %v", err)
	}

	// Validate signature length (ECDSA ASN.1 signature is variable length, max 72 bytes)
	if len(bytes) > 72 {
		return prt.Signature{}, fmt.Errorf("invalid signature length: %d (max 72 bytes)", len(bytes))
	}

	var sig prt.Signature
	copy(sig[:], bytes)
	return sig, nil
}

// Serialization format constants
const (
	SerializationFormatGob = iota
	SerializationFormatJSON
)

// SerializeData serialize object to byte array
// format: serialization format (SerializationFormatGob or SerializationFormatJSON)
func SerializeData(data interface{}, format int) ([]byte, error) {
	switch format {
	case SerializationFormatGob:
		return gobEncode(data)
	case SerializationFormatJSON:
		return json.Marshal(data)
	default:
		return nil, fmt.Errorf("unsupported serialization format: %d", format)
	}
}

// DeserializeData deserialize byte array to object
// format: serialization format (SerializationFormatGob or SerializationFormatJSON)
func DeserializeData(data []byte, result interface{}, format int) error {
	switch format {
	case SerializationFormatGob:
		return gobDecode(data, result)
	case SerializationFormatJSON:
		return json.Unmarshal(data, result)
	default:
		return fmt.Errorf("unsupported serialization format: %d", format)
	}
}

// gobEncode encode data in Gob format
func gobEncode(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("Gob encoding error: %w", err)
	}
	return buf.Bytes(), nil
}

// gobDecode decode data in Gob format
func gobDecode(data []byte, result interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(result); err != nil {
		return fmt.Errorf("Gob decoding error: %w", err)
	}
	return nil
}

// Uint64ToString converts uint64 value to string
func Uint64ToString(value uint64) string {
	return strconv.FormatUint(value, 10)
}

// StringToUint64 converts string to uint64 value
func StringToUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

// Uint64ToBytes converts uint64 value to byte array (for DB key)
func Uint64ToBytes(value uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, value)
	return buf
}

// BytesToUint64 extracts uint64 value from byte array
func BytesToUint64(data []byte) uint64 {
	return binary.BigEndian.Uint64(data)
}

func MarshalJSON(hash prt.Hash) ([]byte, error) {
	return json.Marshal(hex.EncodeToString(hash[:]))
}

func UnmarshalJSON(data []byte, hash *prt.Hash) error {
	return json.Unmarshal(data, hash)
}
