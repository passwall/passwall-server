package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEncryptor(t *testing.T) {
	enc, err := NewEncryptor("test-passphrase")
	require.NoError(t, err)
	assert.NotNil(t, enc)
}

func TestEncryptor_EncryptDecrypt(t *testing.T) {
	enc, err := NewEncryptor("test-passphrase-for-encryption")
	require.NoError(t, err)

	plaintext := "sensitive-password-123"

	// Encrypt
	encrypted, err := enc.Encrypt(plaintext, "")
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, plaintext, encrypted)

	// Decrypt
	decrypted, err := enc.Decrypt(encrypted, "")
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncryptor_EncryptDecrypt_EmptyString(t *testing.T) {
	enc, err := NewEncryptor("test-passphrase")
	require.NoError(t, err)

	encrypted, err := enc.Encrypt("", "")
	require.NoError(t, err)
	assert.Equal(t, "", encrypted)

	decrypted, err := enc.Decrypt("", "")
	require.NoError(t, err)
	assert.Equal(t, "", decrypted)
}

func TestEncryptor_DecryptInvalidCiphertext(t *testing.T) {
	enc, err := NewEncryptor("test-passphrase")
	require.NoError(t, err)

	// Invalid base64
	_, err = enc.Decrypt("not-valid-base64!@#", "")
	assert.Error(t, err)

	// Too short
	_, err = enc.Decrypt("YWJj", "") // "abc" in base64 - too short for GCM
	assert.Error(t, err)
}

func TestEncryptor_EncryptModel(t *testing.T) {
	type TestModel struct {
		ID       uint   `json:"id"`
		Title    string `json:"title"`
		Username string `json:"username" encrypt:"true"`
		Password string `json:"password" encrypt:"true"`
		Note     string `json:"note"`
	}

	enc, err := NewEncryptor("test-passphrase")
	require.NoError(t, err)

	model := &TestModel{
		ID:       1,
		Title:    "Test Login",
		Username: "testuser",
		Password: "testpass123",
		Note:     "This should not be encrypted",
	}

	// Encrypt
	err = enc.EncryptModel(model, "")
	require.NoError(t, err)

	// Check encrypted fields
	assert.NotEqual(t, "testuser", model.Username)
	assert.NotEqual(t, "testpass123", model.Password)

	// Check non-encrypted fields unchanged
	assert.Equal(t, "Test Login", model.Title)
	assert.Equal(t, "This should not be encrypted", model.Note)

	// Decrypt
	err = enc.DecryptModel(model, "")
	require.NoError(t, err)

	// Verify decrypted values
	assert.Equal(t, "testuser", model.Username)
	assert.Equal(t, "testpass123", model.Password)
}

func TestEncryptor_EncryptModel_EmptyFields(t *testing.T) {
	type TestModel struct {
		Username string `json:"username" encrypt:"true"`
		Password string `json:"password" encrypt:"true"`
	}

	enc, err := NewEncryptor("test-passphrase")
	require.NoError(t, err)

	model := &TestModel{
		Username: "",
		Password: "only-password",
	}

	err = enc.EncryptModel(model, "")
	require.NoError(t, err)

	assert.Equal(t, "", model.Username) // Empty should stay empty
	assert.NotEqual(t, "only-password", model.Password)
}

func TestEncryptor_EncryptModel_InvalidInput(t *testing.T) {
	enc, err := NewEncryptor("test-passphrase")
	require.NoError(t, err)

	// Not a pointer
	notPointer := struct{ Field string }{}
	err = enc.EncryptModel(notPointer, "")
	assert.Error(t, err)

	// Not a struct
	notStruct := "string"
	err = enc.EncryptModel(&notStruct, "")
	assert.Error(t, err)
}

func TestEncryptor_CustomPassphrase(t *testing.T) {
	enc, err := NewEncryptor("default-passphrase")
	require.NoError(t, err)

	plaintext := "test-data"
	customPass := "custom-passphrase"

	// Encrypt with custom passphrase
	encrypted, err := enc.Encrypt(plaintext, customPass)
	require.NoError(t, err)

	// Decrypt with custom passphrase
	decrypted, err := enc.Decrypt(encrypted, customPass)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Decrypt with wrong passphrase should fail
	_, err = enc.Decrypt(encrypted, "wrong-passphrase")
	assert.Error(t, err)
}
