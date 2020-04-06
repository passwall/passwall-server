package login

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yakuter/gpass/controller/helper"
	"github.com/yakuter/gpass/model"
	"github.com/yakuter/gpass/pkg/config"
	"github.com/yakuter/gpass/pkg/database"
)

func PostHandler(c *gin.Context) {
	action := c.Param("action")

	switch action {
	case "import":
		Import(c)
	case "export":
		Import(c)
	default:
		err = errors.New("Route not found")
		c.AbortWithError(404, err)
	}
}

func Import(c *gin.Context) {
	url := c.DefaultPostForm("URL", "URL")
	username := c.DefaultPostForm("Username", "Username")
	password := c.DefaultPostForm("Password", "Password")
	path := "./tmp/import/"

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
	file, err := readFile(path + filename)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"Status":  "Error",
			"Message": err.Error(),
		})
		return
	}

	// Match form fields with login fields
	urlIndex, usernameIndex, passwordIndex, err := matchIndex(url, username, password, file)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"Status":  "Error",
			"Message": err.Error(),
		})
		return
	}

	// Read file content and add logins to db
	err = addValues(urlIndex, usernameIndex, passwordIndex, file)
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

func matchIndex(url, username, password string, file *os.File) (int, int, int, error) {
	var urlIndex, usernameIndex, passwordIndex int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")
		urlIndex = findIndex(fields, url)
		usernameIndex = findIndex(fields, username)
		passwordIndex = findIndex(fields, password)

		if urlIndex == -1 || usernameIndex == -1 || passwordIndex == -1 {
			errorText := fmt.Sprintf("%s, %s or %s field couldn't found in %s file", url, username, password, filepath.Base(file.Name()))
			err := errors.New(errorText)
			return -1, -1, -1, err
		}
		break
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
		return -1, -1, -1, err
	}

	return urlIndex, usernameIndex, passwordIndex, nil
}

func addValues(urlIndex, usernameIndex, passwordIndex int, file *os.File) error {
	db = database.GetDB()
	config := config.GetConfig()

	scanner := bufio.NewScanner(file)
	counter := 0
	for scanner.Scan() {
		// Don't add field names to db
		counter++
		if counter == 1 {
			continue
		}

		dizi := strings.Split(scanner.Text(), ",")
		login := model.Login{
			URL:      dizi[urlIndex],
			Username: dizi[usernameIndex],
			Password: helper.Encrypt(dizi[passwordIndex], config.Server.Passphrase),
		}
		db.Create(&login)
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func readFile(filepath string) (*os.File, error) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
	}
	return file, err
}

func findIndex(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}
