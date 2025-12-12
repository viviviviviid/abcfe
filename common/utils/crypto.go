package utils

import (
	"crypto/sha256"
	"fmt"

	prt "github.com/abcfe/abcfe-node/protocol"
)

// 인터페이스를 가져와 해당 내용을 해싱한 후 해시의 16진수 인코딩을 반환
// JSON 직렬화 사용 - GOB는 네트워크 전송 후 해시가 달라지는 문제 있음
func Hash(i interface{}) prt.Hash {
	data, err := SerializeData(i, SerializationFormatJSON)
	if err != nil {
		s := fmt.Sprintf("%v", i)
		return sha256.Sum256([]byte(s))
	}
	return sha256.Sum256(data)
}
