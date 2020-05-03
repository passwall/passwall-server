package app

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// InsertValues ...
func InsertValues(s storage.Store, url, username, password string, file *os.File) error {
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
		login := model.Login{
			URL:      fields[urlIndex],
			Username: fields[usernameIndex],
			Password: base64.StdEncoding.EncodeToString(Encrypt(fields[passwordIndex], viper.GetString("server.passphrase"))),
		}

		// Add to database
		s.Create(&login)
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
		return err
	}

	return nil
}
