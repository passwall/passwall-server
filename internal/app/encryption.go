package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	mathRand "math/rand"
	"os"
	"reflect"
	"time"

	"github.com/Luzifer/go-openssl/v4"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

var (
	minSecureKeyLength = 8
	errShortSecureKey  = errors.New("length of secure key does not meet with minimum requirements")
)

// FindIndex ...
func FindIndex(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

func checkSecureKeyLen(length int) error {
	if length < minSecureKeyLength {
		return errShortSecureKey
	}
	return nil
}

//FallbackInsecureKey fallback method for sercure key
func FallbackInsecureKey(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"0123456789" +
		"~!@#$%^&*()_+{}|<>?,./:"

	if err := checkSecureKeyLen(length); err != nil {
		return "", err
	}

	var seededRand *mathRand.Rand = mathRand.New(
		mathRand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b), nil
}

//GenerateSecureKey generates a secure key width a given length
func GenerateSecureKey(length int) (string, error) {
	key := make([]byte, length)

	if err := checkSecureKeyLen(length); err != nil {
		return "", err
	}
	_, err := rand.Read(key)
	if err != nil {
		return FallbackInsecureKey(length)
	}
	// encrypted key length > provided key length
	keyEnc := base64.StdEncoding.EncodeToString(key)
	return keyEnc, nil
}

// NewBcrypt ...
func NewBcrypt(key []byte) string {
	hasher, _ := bcrypt.GenerateFromPassword(key, bcrypt.DefaultCost)
	return string(hasher)
}

// CreateHash ...
func CreateHash(key string) (string, error) {
	hasher := md5.New()
	if _, err := hasher.Write([]byte(key)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Encrypt ..
func Encrypt(dataStr string, passphrase string) ([]byte, error) {
	dataByte := []byte(dataStr)
	hash, err := CreateHash(passphrase)
	if err != nil {
		return nil, err
	}
	block, _ := aes.NewCipher([]byte(hash))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	cipherByte := gcm.Seal(nonce, nonce, dataByte, nil)
	return cipherByte, nil
}

// Decrypt ...
func Decrypt(dataStr string, passphrase string) ([]byte, error) {
	dataByte := []byte(dataStr)
	hash, err := CreateHash(passphrase)
	if err != nil {
		return nil, err
	}
	key := []byte(hash)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := dataByte[:nonceSize], dataByte[nonceSize:]
	plainByte, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}
	return plainByte, nil
	// return string(plainByte[:])
}

// EncryptFile ...
func EncryptFile(filename string, data []byte, passphrase string) error {
	f, _ := os.Create(filename)
	defer f.Close()
	e, err := Encrypt(string(data[:]), passphrase)
	if err != nil {
		return err
	}
	if _, err := f.Write(e); err != nil {
		return err
	}
	return nil
}

// DecryptFile ...
func DecryptFile(filename string, passphrase string) ([]byte, error) {
	data, _ := ioutil.ReadFile(filename)
	d, err := Decrypt(string(data[:]), passphrase)

	if err != nil {
		return nil, err
	}

	return d, nil
}

// EncryptModel encrypts struct pointer according to struct tags
func EncryptModel(rawModel interface{}) (interface{}, error) {
	num := reflect.ValueOf(rawModel).Elem().NumField()

	var tagVal string

	for i := 0; i < num; i++ {
		tagVal = reflect.TypeOf(rawModel).Elem().Field(i).Tag.Get("encrypt")
		value := reflect.ValueOf(rawModel).Elem().Field(i).String()

		if tagVal == "true" {
			e, err := Encrypt(value, viper.GetString("server.passphrase"))
			if err != nil {
				return nil, err
			}
			value = base64.StdEncoding.EncodeToString(e)
			reflect.ValueOf(rawModel).Elem().Field(i).SetString(value)
		}
	}

	return rawModel, nil
}

// DecryptModel decrypts struct pointer according to struct tags
func DecryptModel(rawModel interface{}) (interface{}, error) {
	var err error
	var valueByte []byte
	num := reflect.ValueOf(rawModel).Elem().NumField()

	var tagVal string

	for i := 0; i < num; i++ {
		tagVal = reflect.TypeOf(rawModel).Elem().Field(i).Tag.Get("encrypt")
		value := reflect.ValueOf(rawModel).Elem().Field(i).String()

		if tagVal == "true" {
			valueByte, err = base64.StdEncoding.DecodeString(value)
			d, err := Decrypt(string(valueByte[:]), viper.GetString("server.passphrase"))
			if err != nil {
				return nil, err
			}
			value = string(d)
			reflect.ValueOf(rawModel).Elem().Field(i).SetString(value)
		}
	}

	return rawModel, err
}

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
