package app

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
)

// GetMD5Hash ...
func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// RandomMD5Hash returns random md5 hash for unique conifrim links
func RandomMD5Hash() string {
	b := make([]byte, 16)
	rand.Read(b)
	return GetMD5Hash(string(b))
}
