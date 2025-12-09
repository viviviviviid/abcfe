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

// HashToString Hash 타입을 16진수 문자열로 변환
func HashToString(hash prt.Hash) string {
	return hex.EncodeToString(hash[:])
}

// StringToHash 16진수 문자열을 Hash 타입으로 변환
func StringToHash(str string) (prt.Hash, error) {
	// 0x 접두사 제거
	if len(str) >= 2 && str[0:2] == "0x" {
		str = str[2:]
	}
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return prt.Hash{}, fmt.Errorf("invalid hash string: %v", err)
	}

	// 해시 길이 검증
	if len(bytes) != 32 {
		return prt.Hash{}, fmt.Errorf("invalid hash lenght: %d (need 32 bytes)", len(bytes))
	}

	var hash prt.Hash
	copy(hash[:], bytes)
	return hash, nil
}

// BytesToHash 바이트 배열을 Hash 타입으로 변환
func BytesToHash(bytes []byte) prt.Hash {
	return prt.Hash(bytes)
}

// HashToBytes Hash 타입을 바이트 배열로 변환
func HashToBytes(hash prt.Hash) []byte {
	bytes := make([]byte, len(hash))
	copy(bytes, hash[:])
	return bytes
}

// AddressToString Address 타입을 16진수 문자열로 변환
func AddressToString(address prt.Address) string {
	return hex.EncodeToString(address[:])
}

// StringToAddress 16진수 문자열을 Address 타입으로 변환
func StringToAddress(str string) (prt.Address, error) {
	// 0x 접두사 제거
	if len(str) >= 2 && str[0:2] == "0x" {
		str = str[2:]
	}
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return prt.Address{}, fmt.Errorf("잘못된 주소 문자열: %v", err)
	}

	// 주소 길이 검증
	if len(bytes) != 20 {
		return prt.Address{}, fmt.Errorf("잘못된 주소 길이: %d (20바이트 필요)", len(bytes))
	}

	var address prt.Address
	copy(address[:], bytes)
	return address, nil
}

// SignatureToString Signature 타입을 16진수 문자열로 변환
func SignatureToString(sig prt.Signature) string {
	return hex.EncodeToString(sig[:])
}

// StringToSignature 16진수 문자열을 Signature 타입으로 변환
func StringToSignature(str string) (prt.Signature, error) {
	// 0x 접두사 제거
	if len(str) >= 2 && str[0:2] == "0x" {
		str = str[2:]
	}
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return prt.Signature{}, fmt.Errorf("잘못된 서명 문자열: %v", err)
	}

	// 서명 길이 검증 (ECDSA ASN.1 서명은 가변 길이, 최대 72바이트)
	if len(bytes) > 72 {
		return prt.Signature{}, fmt.Errorf("잘못된 서명 길이: %d (최대 72바이트)", len(bytes))
	}

	var sig prt.Signature
	copy(sig[:], bytes)
	return sig, nil
}

// 직렬화 방식 상수
const (
	SerializationFormatGob = iota
	SerializationFormatJSON
)

// SerializeData 객체를 바이트 배열로 직렬화
// format: 직렬화 방식 (SerializationFormatGob 또는 SerializationFormatJSON)
func SerializeData(data interface{}, format int) ([]byte, error) {
	switch format {
	case SerializationFormatGob:
		return gobEncode(data)
	case SerializationFormatJSON:
		return json.Marshal(data)
	default:
		return nil, fmt.Errorf("지원하지 않는 직렬화 형식: %d", format)
	}
}

// DeserializeData 바이트 배열을 객체로 역직렬화
// format: 직렬화 방식 (SerializationFormatGob 또는 SerializationFormatJSON)
func DeserializeData(data []byte, result interface{}, format int) error {
	switch format {
	case SerializationFormatGob:
		return gobDecode(data, result)
	case SerializationFormatJSON:
		return json.Unmarshal(data, result)
	default:
		return fmt.Errorf("지원하지 않는 직렬화 형식: %d", format)
	}
}

// gobEncode Gob 형식으로 데이터 인코딩
func gobEncode(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("Gob 인코딩 오류: %w", err)
	}
	return buf.Bytes(), nil
}

// gobDecode Gob 형식으로 데이터 디코딩
func gobDecode(data []byte, result interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(result); err != nil {
		return fmt.Errorf("Gob 디코딩 오류: %w", err)
	}
	return nil
}

// Uint64ToString uint64 값을 문자열로 변환
func Uint64ToString(value uint64) string {
	return strconv.FormatUint(value, 10)
}

// StringToUint64 문자열을 uint64 값으로 변환
func StringToUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

// Uint64ToBytes uint64 값을 바이트 배열로 변환 (DB 키용)
func Uint64ToBytes(value uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, value)
	return buf
}

// BytesToUint64 바이트 배열에서 uint64 값 추출
func BytesToUint64(data []byte) uint64 {
	return binary.BigEndian.Uint64(data)
}

func MarshalJSON(hash prt.Hash) ([]byte, error) {
	return json.Marshal(hex.EncodeToString(hash[:]))
}

func UnmarshalJSON(data []byte, hash *prt.Hash) error {
	return json.Unmarshal(data, hash)
}
