package app

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
)

// GetMD5Hash ...
func GetMD5Hash(text []byte) string {
	hasher := md5.New()
	hasher.Write(text)
	return hex.EncodeToString(hasher.Sum(nil))
}

// RandomMD5Hash returns random md5 hash for unique conifrim links
func RandomMD5Hash() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return GetMD5Hash(b), nil
}
