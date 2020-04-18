package login

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/pkg/database"
)

// PostHandler ...
func PostHandler(c *gin.Context) {
	action := c.Param("action")

	switch action {
	case "import":
		Import(c)
	case "export":
		Export(c)
	case "generate-password":
		GeneratePassword(c)
	default:
		response := LoginResponse{"Error", "Route not found"}
		c.JSON(http.StatusNotFound, response)
	}
}

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

// Export exports all logins as CSV file
func Export(c *gin.Context) {
	db := database.GetDB()

	var logins []Login
	filepath := "/tmp/passwall_api_export.csv"

	db.Find(&logins)
	logins = DecryptLoginPasswords(logins)

	file, err := os.Create(filepath)
	if err != nil {
		log.Println(err)
	}

	file.WriteString("URL,Username,Password\n")

	for _, login := range logins {
		_, err := file.WriteString(fmt.Sprintf("%s,%s,%s\n", login.URL, login.Username, login.Password))

		if err != nil {
			log.Println(err)
		}

	}

	c.File(filepath)
	c.Status(http.StatusOK)

	file.Close()
}

// GeneratePassword generates new password
func GeneratePassword(c *gin.Context) {
	password := Password()
	response := LoginResponse{"Success", password}
	c.JSON(http.StatusOK, response)
}
