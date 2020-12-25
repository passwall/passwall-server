package app

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
)

// GetMD5Hash ...
func GetMD5Hash(text []byte) (string, error) {
	hasher := md5.New()
	if _, err := hasher.Write(text); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// RandomMD5Hash returns random md5 hash for unique conifrim links
func RandomMD5Hash() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	r, err := GetMD5Hash(b)
	if err != nil {
		return "", err
	}

	return r, nil
}
