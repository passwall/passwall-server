package login

import (
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-api/pkg/config"
)

// LoginAPI ...
type LoginAPI struct {
	LoginService LoginService
}

// NewLoginAPI ...
func NewLoginAPI(p LoginService) LoginAPI {
	return LoginAPI{LoginService: p}
}

// FindAll ...
func (p *LoginAPI) FindAll(c *gin.Context) {
	logins := p.LoginService.FindAll()
	logins = DecryptLoginPasswords(logins)

	c.JSON(http.StatusOK, ToLoginDTOs(logins))
}

// FindByID ...
func (p *LoginAPI) FindByID(c *gin.Context) {
	config := config.GetConfig()
	id, _ := strconv.Atoi(c.Param("id"))
	login := p.LoginService.FindByID(uint(id))

	passByte, _ := base64.StdEncoding.DecodeString(login.Password)
	login.Password = Decrypt(string(passByte[:]), config.Server.Passphrase)

	c.JSON(http.StatusOK, ToLoginDTO(login))
}

// Create ...
func (p *LoginAPI) Create(c *gin.Context) {
	config := config.GetConfig()
	var loginDTO LoginDTO
	err := c.BindJSON(&loginDTO)
	if err != nil {
		log.Println(err)
		c.Status(http.StatusBadRequest)
		return
	}

	if loginDTO.Password == "" {
		loginDTO.Password = Password()
	}

	rawPass := loginDTO.Password
	loginDTO.Password = base64.StdEncoding.EncodeToString(Encrypt(loginDTO.Password, config.Server.Passphrase))

	createdLogin := p.LoginService.Save(ToLogin(loginDTO))
	createdLogin.Password = rawPass

	c.JSON(http.StatusOK, ToLoginDTO(createdLogin))
}

// Update ...
func (p *LoginAPI) Update(c *gin.Context) {
	var loginDTO LoginDTO
	err := c.BindJSON(&loginDTO)
	if err != nil {
		log.Fatalln(err)
		c.Status(http.StatusBadRequest)
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	login := p.LoginService.FindByID(uint(id))
	if login == (Login{}) {
		c.Status(http.StatusBadRequest)
		return
	}

	login.URL = loginDTO.URL
	login.Username = loginDTO.Username
	login.Password = loginDTO.Password
	p.LoginService.Save(login)

	c.Status(http.StatusOK)
}

// Delete ...
func (p *LoginAPI) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	login := p.LoginService.FindByID(uint(id))
	if login == (Login{}) {
		c.Status(http.StatusBadRequest)
		return
	}

	p.LoginService.Delete(login.ID)

	c.Status(http.StatusOK)
}

// Import ...
func (p *LoginAPI) Import(c *gin.Context) {
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
