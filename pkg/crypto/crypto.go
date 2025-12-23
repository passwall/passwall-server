package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"reflect"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrInvalidKey        = errors.New("invalid encryption key")
)

// Encryptor handles AES-GCM encryption (ORIGINAL IMPLEMENTATION)
// This matches the original server-side encryption
type Encryptor struct {
	passphrase string
}

// NewEncryptor creates a new encryptor
func NewEncryptor(passphrase string) (*Encryptor, error) {
	if passphrase == "" {
		return nil, ErrInvalidKey
	}

	return &Encryptor{
		passphrase: passphrase,
	}, nil
}

// createHash creates MD5 hash (original implementation)
func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Encrypt encrypts using AES-GCM (ORIGINAL)
func (e *Encryptor) Encrypt(plaintext, passphrase string) (string, error) {
	if passphrase == "" {
		passphrase = e.passphrase
	}

	if plaintext == "" {
		return "", nil
	}

	dataByte := []byte(plaintext)
	block, err := aes.NewCipher([]byte(createHash(passphrase)))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherByte := gcm.Seal(nonce, nonce, dataByte, nil)
	return base64.StdEncoding.EncodeToString(cipherByte), nil
}

// Decrypt decrypts using AES-GCM (ORIGINAL)
func (e *Encryptor) Decrypt(ciphertext, passphrase string) (string, error) {
	if passphrase == "" {
		passphrase = e.passphrase
	}

	if ciphertext == "" {
		return "", nil
	}

	dataByte, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(dataByte) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := dataByte[:nonceSize], dataByte[nonceSize:]
	plainByte, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plainByte), nil
}

// EncryptModel encrypts struct fields with encrypt:"true" tag
func (e *Encryptor) EncryptModel(model interface{}, passphrase string) error {
	if passphrase == "" {
		passphrase = e.passphrase
	}

	v := reflect.ValueOf(model)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return errors.New("model must be a struct pointer")
	}

	v = v.Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		typeField := t.Field(i)

		if typeField.Tag.Get("encrypt") == "true" && field.Kind() == reflect.String {
			value := field.String()
			if value != "" {
				encrypted, err := e.Encrypt(value, passphrase)
				if err != nil {
					return err
				}
				field.SetString(encrypted)
			}
		}
	}

	return nil
}

// DecryptModel decrypts struct fields with encrypt:"true" tag
func (e *Encryptor) DecryptModel(model interface{}, passphrase string) error {
	if passphrase == "" {
		passphrase = e.passphrase
	}

	v := reflect.ValueOf(model)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return errors.New("model must be a struct pointer")
	}

	v = v.Elem()
	t := v.Type()

	var lastErr error
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		typeField := t.Field(i)

		if typeField.Tag.Get("encrypt") == "true" && field.Kind() == reflect.String {
			value := field.String()
			if value != "" {
				decrypted, err := e.Decrypt(value, passphrase)
				if err != nil {
					lastErr = err
					continue
				}
				field.SetString(decrypted)
			}
		}
	}

	return lastErr
}
