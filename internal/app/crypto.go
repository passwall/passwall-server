package app

import (
	"encoding/json"

	openssl "github.com/Luzifer/go-openssl/v4"
)

// DecryptJSON ...
func DecryptJSON(key string, encrypted []byte, v interface{}) error {

	// 1. Get a openssl object
	o := openssl.New()

	// 2. Decrypt string
	dec, err := o.DecryptBytes(key, encrypted, openssl.BytesToKeyMD5)
	if err != nil {
		return err
	}

	// 3. Convert string to JSON
	if err := json.Unmarshal(dec, v); err != nil {
		return err
	}

	return nil
}

// EncryptJSON ...
func EncryptJSON(key string, v interface{}) ([]byte, error) {

	// 1. Get a openssl object
	o := openssl.New()

	// 2. Marshall to text
	text, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// 3. Encrypt it
	enc, err := o.EncryptBytes(key, text, openssl.BytesToKeyMD5)
	if err != nil {
		return nil, err
	}

	return enc, nil
}
