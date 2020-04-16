package login

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
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
	var err error
	logins := []Login{}
	search := c.DefaultQuery("Search", "")

	if search != "" {
		logins, err = p.LoginService.Search(search)
	} else {
		logins, err = p.LoginService.FindAll()
	}

	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	logins = DecryptLoginPasswords(logins)
	c.JSON(http.StatusOK, ToLoginDTOs(logins))
}

// FindByID ...
func (p *LoginAPI) FindByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	login, err := p.LoginService.FindByID(uint(id))
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	passByte, _ := base64.StdEncoding.DecodeString(login.Password)
	login.Password = Decrypt(string(passByte[:]), viper.GetString("server.passphrase"))

	c.JSON(http.StatusOK, ToLoginDTO(login))
}

// Create ...
func (p *LoginAPI) Create(c *gin.Context) {
	var loginDTO LoginDTO
	err := c.BindJSON(&loginDTO)
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	if loginDTO.Password == "" {
		loginDTO.Password = Password()
	}

	rawPass := loginDTO.Password
	loginDTO.Password = base64.StdEncoding.EncodeToString(Encrypt(loginDTO.Password, viper.GetString("server.passphrase")))

	createdLogin, err := p.LoginService.Save(ToLogin(loginDTO))
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	createdLogin.Password = rawPass

	c.JSON(http.StatusOK, ToLoginDTO(createdLogin))
}

// Update ...
func (p *LoginAPI) Update(c *gin.Context) {
	var loginDTO LoginDTO
	err := c.BindJSON(&loginDTO)
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	login, err := p.LoginService.FindByID(uint(id))
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	if loginDTO.Password == "" {
		loginDTO.Password = Password()
	}
	rawPass := loginDTO.Password
	loginDTO.Password = base64.StdEncoding.EncodeToString(Encrypt(loginDTO.Password, viper.GetString("server.passphrase")))

	login.URL = loginDTO.URL
	login.Username = loginDTO.Username
	login.Password = loginDTO.Password
	updatedLogin, err := p.LoginService.Save(login)
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}
	updatedLogin.Password = rawPass
	c.JSON(http.StatusOK, ToLoginDTO(updatedLogin))
}

// Delete ...
func (p *LoginAPI) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	login, err := p.LoginService.FindByID(uint(id))
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	err = p.LoginService.Delete(login.ID)
	if err != nil {
		response := LoginResponse{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	response := LoginResponse{"Success", "Login deleted successfully!"}
	c.JSON(http.StatusOK, response)
}

// Migrate ...
func (p *LoginAPI) Migrate() {
	p.LoginService.Migrate()
}
