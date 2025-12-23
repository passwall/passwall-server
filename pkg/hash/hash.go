package hash

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256 generates a SHA-256 hash of the input string
func SHA256(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	return hex.EncodeToString(hash.Sum(nil))
}

// VerifySHA256 verifies if the input matches the hash
func VerifySHA256(input, hash string) bool {
	return SHA256(input) == hash
}

