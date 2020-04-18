package login

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/pkg/database"
	"github.com/spf13/viper"
)

// AddValues ...
func AddValues(url, username, password string, file *os.File) error {
	db := database.GetDB()
	var urlIndex, usernameIndex, passwordIndex int

	scanner := bufio.NewScanner(file)
	counter := 0
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")

		// Ignore first line (field names)
		counter++
		if counter == 1 {
			// Match user's fieldnames to passwall's field names (URL, Username, Password)
			urlIndex = FindIndex(fields, url)
			usernameIndex = FindIndex(fields, username)
			passwordIndex = FindIndex(fields, password)

			// Check if fields match
			if urlIndex == -1 || usernameIndex == -1 || passwordIndex == -1 {
				errorText := fmt.Sprintf("%s, %s or %s field couldn't found in %s file", url, username, password, filepath.Base(file.Name()))
				err := errors.New(errorText)
				return err
			}
			continue
		}

		// if isRecordNotFound(fields[urlIndex], fields[usernameIndex], fields[passwordIndex]) {
		// Fill login struct with csv file content
		login := Login{
			URL:      fields[urlIndex],
			Username: fields[usernameIndex],
			Password: base64.StdEncoding.EncodeToString(Encrypt(fields[passwordIndex], viper.GetString("server.passphrase"))),
		}

		// Add to database
		db.Create(&login)
		// }
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func FindIndex(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

// CheckErr ...
func CheckErr(err error) {
	if err != nil {
		log.Println(err)
	}
}

// Include ...
func Include(vs []string, t string) bool {
	return FindIndex(vs, t) >= 0
}

// SetArgs ...
func SetArgs(c *gin.Context) (map[string]string, map[string]int) {

	// String type query params
	search := c.DefaultQuery("Search", "")
	sort := c.DefaultQuery("Sort", "updated_at")
	order := c.DefaultQuery("Order", "DESC")
	argsStr := map[string]string{
		"search": search,
		"order":  setOrder(sort, order),
	}

	// Integer type query params
	offset := c.DefaultQuery("Offset", "")
	limit := c.DefaultQuery("Limit", "")
	argsInt := map[string]int{
		"offset": setOffset(offset),
		"limit":  setLimit(limit),
	}

	return argsStr, argsInt
}

// Offset returns the starting number of result for pagination
func setOffset(offset string) int {
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		return -1
	}

	// don't allow negative values
	// except -1 which cancels offset condition
	if offsetInt < 0 {
		offsetInt = -1
	}
	return offsetInt
}

// Limit returns the number of result for pagination
func setLimit(limit string) int {
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		// -1 cancels limit condition
		return -1
	}

	// min limit should be 1
	if limitInt < 1 {
		limitInt = 1
	}
	return limitInt
}

// SortOrder returns the string for sorting and orderin data
func setOrder(sort, order string) string {
	sortValues := []string{"id", "created_at", "updated_at", "url", "username"}
	orderValues := []string{"desc", "asc"}

	if Include(sortValues, strings.ToLower(sort)) && Include(orderValues, strings.ToLower(order)) {
		return ToSnakeCase(sort) + " " + ToSnakeCase(order)
	}

	return "updated_at desc"
}

// Search adds where to search keywords
func setSearch(search string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if search != "" {

			// Case insensitive is different in postgres and others (mysql,sqlite)
			if viper.GetString("database.driver") == "postgres" {
				db = db.Where("url ILIKE ?", "%"+search+"%")
				db = db.Or("username ILIKE ?", "%"+search+"%")
			} else {
				db = db.Where("url LIKE ?", "%"+search+"%")
				db = db.Or("username LIKE ?", "%"+search+"%")
			}

		}
		return db
	}
}

// ToSnakeCase changes string to database table
func ToSnakeCase(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}

// Password ..
func Password() string {
	mathrand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789" +
		"=+%*/()[]{}/!@#$?|")
	length := 16
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
func Decrypt(dataStr string, passphrase string) string {
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
	return string(plainByte[:])
}

// DecryptLoginPasswords ...
func DecryptLoginPasswords(logins []Login) []Login {
	for i := range logins {
		if logins[i].Password == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(logins[i].Password)
		passB64 := Decrypt(string(passByte[:]), viper.GetString("server.passphrase"))
		logins[i].Password = passB64
	}
	return logins
}
