package app

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/api/login"
	"github.com/pass-wall/passwall-server/helper"
	"github.com/pass-wall/passwall-server/internal/database"
	"github.com/spf13/viper"
)

// Import ...
func Import(c *gin.Context) {
	url := c.DefaultPostForm("URL", "URL")
	username := c.DefaultPostForm("Username", "Username")
	password := c.DefaultPostForm("Password", "Password")
	path := "/tmp/"

	formFile, err := c.FormFile("File")
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, err)
		return
	}

	filename := filepath.Base(formFile.Filename)

	// Save file to ./tmp/import folder
	if err := c.SaveUploadedFile(formFile, path+filename); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"Status":  "Error",
			"Message": err.Error(),
		})
		return
	}

	// get file content
	file, err := os.Open(path + filename)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"Status":  "Error",
			"Message": err.Error(),
		})
		return
	}

	// Read file content and add logins to db
	err = AddValues(url, username, password, file)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"Status":  "Error",
			"Message": err.Error(),
		})
		return
	}

	// Delete imported file
	err = os.Remove(path + filename)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"Status":  "Error",
			"Message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"Status":  "Success",
		"Message": "Import finished successfully",
	})
}

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
			urlIndex = helper.FindIndex(fields, url)
			usernameIndex = helper.FindIndex(fields, username)
			passwordIndex = helper.FindIndex(fields, password)

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
		login := login.Login{
			URL:      fields[urlIndex],
			Username: fields[usernameIndex],
			Password: base64.StdEncoding.EncodeToString(helper.Encrypt(fields[passwordIndex], viper.GetString("server.passphrase"))),
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
