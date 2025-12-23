package service

import (
	"testing"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Example: Using mock config in crypto service tests
func TestCryptoService_WithMockConfig(t *testing.T) {
	// Create a mock config with a known passphrase
	cfg := config.NewMockBuilder().
		WithPassphrase("test-encryption-passphrase").
		Build()

	// Create crypto service using the mock config
	cryptoSvc := NewCryptoService(cfg.Server.Passphrase)

	t.Run("encrypt and decrypt", func(t *testing.T) {
		plaintext := "secret-password-123"

		// Encrypt
		encrypted, err := cryptoSvc.Encrypt(plaintext, "")
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.NotEqual(t, plaintext, encrypted)

		// Decrypt
		decrypted, err := cryptoSvc.Decrypt(encrypted, "")
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("encrypt with custom passphrase", func(t *testing.T) {
		plaintext := "another-secret"
		customPass := "custom-passphrase"

		encrypted, err := cryptoSvc.Encrypt(plaintext, customPass)
		require.NoError(t, err)

		decrypted, err := cryptoSvc.Decrypt(encrypted, customPass)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("encrypt model with domain entity", func(t *testing.T) {
		login := &domain.Login{
			Title:      "Test Login",
			Username:   "testuser",
			Password:   "testpass123",
			TOTPSecret: "totp-secret",
		}

		// Encrypt
		err := cryptoSvc.EncryptModel(login, "")
		require.NoError(t, err)
		assert.NotEqual(t, "testuser", login.Username)
		assert.NotEqual(t, "testpass123", login.Password)

		// Decrypt
		err = cryptoSvc.DecryptModel(login, "")
		require.NoError(t, err)
		assert.Equal(t, "testuser", login.Username)
		assert.Equal(t, "testpass123", login.Password)
	})
}

// Example: Testing different encryption scenarios with different configs
func TestCryptoService_DifferentPassphrases(t *testing.T) {
	tests := []struct {
		name       string
		passphrase string
		plaintext  string
	}{
		{
			name:       "short passphrase",
			passphrase: "short123",
			plaintext:  "test data",
		},
		{
			name:       "long passphrase",
			passphrase: "this-is-a-very-long-passphrase-for-encryption",
			plaintext:  "sensitive information",
		},
		{
			name:       "special characters",
			passphrase: "p@$$phr@$e!123#",
			plaintext:  "credit card number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cryptoSvc := NewCryptoService(tt.passphrase)

			encrypted, err := cryptoSvc.Encrypt(tt.plaintext, "")
			require.NoError(t, err)

			decrypted, err := cryptoSvc.Decrypt(encrypted, "")
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

