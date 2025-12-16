package utils

import (
	"crypto/sha256"
	"fmt"

	prt "github.com/abcfe/abcfe-node/protocol"
)

// Take an interface, hash its content, and return hex encoding of the hash
// Use JSON serialization - GOB has issues with changing hash after network transmission
func Hash(i interface{}) prt.Hash {
	data, err := SerializeData(i, SerializationFormatJSON)
	if err != nil {
		s := fmt.Sprintf("%v", i)
		return sha256.Sum256([]byte(s))
	}
	return sha256.Sum256(data)
}
