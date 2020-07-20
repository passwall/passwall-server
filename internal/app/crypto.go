package app

import (
	"encoding/json"
	"log"

	openssl "github.com/Luzifer/go-openssl/v4"
	"github.com/spf13/viper"
)

// DecryptJSON ...
func DecryptJSON(encrypted []byte, v interface{}) error {

	// 1. Get a openssl object and secret key from configs
	o := openssl.New()
	secret := viper.GetString("server.aesKey")

	// 2. Decrypt string
	dec, err := o.DecryptBytes(secret, encrypted, openssl.BytesToKeyMD5)
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
func EncryptJSON(v interface{}) ([]byte, error) {

	// 1. Get a openssl object and secret key from configs
	o := openssl.New()
	secret := viper.GetString("server.aesKey")

	// 2. Marshall to text
	text, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// 3. Encrypt it
	enc, err := o.EncryptBytes(secret, text, openssl.BytesToKeyMD5)
	if err != nil {
		return nil, err
	}
	log.Println(string(enc))
	return enc, nil
}
