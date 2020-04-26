package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
	"io/ioutil"
	mathrand "math/rand"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
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

// Include ...
func Include(vs []string, t string) bool {
	return FindIndex(vs, t) >= 0
}

// Password ..
func Password() string {
	mathrand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789" +
		"=+%*/()[]{}/!@#$?|")
	length := viper.GetInt("server.generatedPasswordLength")
	if length < 4 {
		length = 4
	}
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[mathrand.Intn(len(chars))])
	}
	return b.String()
}

// CreateHash ...
func CreateHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Encrypt ..
func Encrypt(dataStr string, passphrase string) []byte {
	dataByte := []byte(dataStr)
	block, _ := aes.NewCipher([]byte(CreateHash(passphrase)))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	cipherByte := gcm.Seal(nonce, nonce, dataByte, nil)
	return cipherByte
}

// Decrypt ...
func Decrypt(dataStr string, passphrase string) []byte {
	dataByte := []byte(dataStr)
	key := []byte(CreateHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := dataByte[:nonceSize], dataByte[nonceSize:]
	plainByte, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}
	return plainByte
	// return string(plainByte[:])
}

// EncryptFile ...
func EncryptFile(filename string, data []byte, passphrase string) {
	f, _ := os.Create(filename)
	defer f.Close()
	f.Write(Encrypt(string(data[:]), passphrase))
}

// DecryptFile ...
func DecryptFile(filename string, passphrase string) []byte {
	data, _ := ioutil.ReadFile(filename)
	return Decrypt(string(data[:]), passphrase)
}
