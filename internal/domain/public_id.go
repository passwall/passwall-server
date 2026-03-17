package domain

import (
	"crypto/rand"
	"math/big"
)

const (
	publicIDAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	PublicIDLength   = 12
)

// GeneratePublicID creates a cryptographically random 12-character
// alphanumeric string suitable for use in URLs.
// 62^12 ≈ 3.2×10²¹ possible values — collision-safe at any practical scale.
func GeneratePublicID() (string, error) {
	alphabetLen := big.NewInt(int64(len(publicIDAlphabet)))
	b := make([]byte, PublicIDLength)
	for i := range b {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		b[i] = publicIDAlphabet[idx.Int64()]
	}
	return string(b), nil
}
