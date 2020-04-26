package api

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/store"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// LoginAPI ...
type LoginAPI struct {
	LoginService store.LoginService
}

// NewLoginAPI ...
func NewLoginAPI(p store.LoginService) LoginAPI {
	return LoginAPI{LoginService: p}
}

// FindSamePassword ...
func (p *LoginAPI) FindSamePassword(c *gin.Context) {
	var password model.Password

	c.BindJSON(&password)

	urls, err := app.FindSamePassword(&p.LoginService, password)

	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	c.JSON(http.StatusOK, urls)
}

// FindAll ...
func (p *LoginAPI) FindAll(c *gin.Context) {
	var err error
	logins := []model.Login{}

	argsStr, argsInt := SetArgs(c)

	logins, err = p.LoginService.FindAll(argsStr, argsInt)

	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	logins = app.DecryptLoginPasswords(logins)
	c.JSON(http.StatusOK, model.ToLoginDTOs(logins))
}

// FindByID ...
func (p *LoginAPI) FindByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	login, err := p.LoginService.FindByID(uint(id))
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	passByte, _ := base64.StdEncoding.DecodeString(login.Password)
	login.Password = string(encryption.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

	c.JSON(http.StatusOK, model.ToLoginDTO(login))
}

// Create ...
func (p *LoginAPI) Create(c *gin.Context) {
	var loginDTO model.LoginDTO
	err := c.BindJSON(&loginDTO)
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	if loginDTO.Password == "" {
		loginDTO.Password = encryption.Password()
	}

	rawPass := loginDTO.Password
	loginDTO.Password = base64.StdEncoding.EncodeToString(encryption.Encrypt(loginDTO.Password, viper.GetString("server.passphrase")))

	createdLogin, err := p.LoginService.Save(model.ToLogin(loginDTO))
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	createdLogin.Password = rawPass

	c.JSON(http.StatusOK, model.ToLoginDTO(createdLogin))
}

// Update ...
func (p *LoginAPI) Update(c *gin.Context) {
	var loginDTO model.LoginDTO
	err := c.BindJSON(&loginDTO)
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	login, err := p.LoginService.FindByID(uint(id))
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	if loginDTO.Password == "" {
		loginDTO.Password = encryption.Password()
	}
	rawPass := loginDTO.Password
	loginDTO.Password = base64.StdEncoding.EncodeToString(encryption.Encrypt(loginDTO.Password, viper.GetString("server.passphrase")))

	login.URL = loginDTO.URL
	login.Username = loginDTO.Username
	login.Password = loginDTO.Password
	updatedLogin, err := p.LoginService.Save(login)
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}
	updatedLogin.Password = rawPass
	c.JSON(http.StatusOK, model.ToLoginDTO(updatedLogin))
}

// Delete ...
func (p *LoginAPI) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusBadRequest, response)
		return
	}

	login, err := p.LoginService.FindByID(uint(id))
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	err = p.LoginService.Delete(login.ID)
	if err != nil {
		response := model.Response{"Error", err.Error()}
		c.JSON(http.StatusNotFound, response)
		return
	}

	response := model.Response{"Success", "Login deleted successfully!"}
	c.JSON(http.StatusOK, response)
}

// Migrate ...
func (p *LoginAPI) Migrate() {
	p.LoginService.Migrate()
}
