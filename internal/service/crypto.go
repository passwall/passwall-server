package service

import (
	"github.com/passwall/passwall-server/pkg/crypto"
)

// NewCryptoService creates a new encryption service
// Uses original MD5-based encryption for backwards compatibility
func NewCryptoService(passphrase string) Encryptor {
	encryptor, err := crypto.NewEncryptor(passphrase)
	if err != nil {
		// Fallback to a working encryptor
		encryptor, _ = crypto.NewEncryptor("fallback-passphrase-change-me")
	}

	return encryptor
}
