package app

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"log"
	"os"

	"github.com/pass-wall/passwall-server/internal/common"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// Import ...
func Import(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	username := r.FormValue("username")
	password := r.FormValue("password")

	uploadedFile, err := upload(r)
	if err != nil {
		common.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer uploadedFile.Close()

	// Go to first line of file
	uploadedFile.Seek(0, 0)

	// Read file content and add logins to db
	err = InsertValues(url, username, password, uploadedFile)
	if err != nil {
		common.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Delete imported file
	err = os.Remove(uploadedFile.Name())
	if err != nil {
		common.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := model.Response{http.StatusOK, "Success", "Import finished successfully!"}
	common.RespondWithJSON(w, http.StatusOK, response)
}

// InsertValues ...
func InsertValues(url, username, password string, file *os.File) error {
	db := storage.GetDB()
	var urlIndex, usernameIndex, passwordIndex int

	scanner := bufio.NewScanner(file)
	counter := 0
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")

		// Ignore first line (field names)
		counter++
		if counter == 1 {
			// Match user's fieldnames to passwall's field names (URL, Username, Password)
			urlIndex = encryption.FindIndex(fields, url)
			usernameIndex = encryption.FindIndex(fields, username)
			passwordIndex = encryption.FindIndex(fields, password)

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
			Password: base64.StdEncoding.EncodeToString(encryption.Encrypt(fields[passwordIndex], viper.GetString("server.passphrase"))),
		}

		// Add to database
		db.Create(&login)
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func upload(r *http.Request) (*os.File, error) {

	// Max 10 MB
	r.ParseMultipartForm(10 << 20)

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)

	if ext != ".csv" {
		return nil, fmt.Errorf("%s unsupported filetype", ext)
	}

	tempFile, err := ioutil.TempFile("/tmp", "passwall-import-*.csv")
	if err != nil {
		return nil, err
	}

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	tempFile.Write(fileBytes)

	return tempFile, err
}
