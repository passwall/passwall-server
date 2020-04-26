package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/model"
)

// GeneratePassword generates new password
func GeneratePassword(c *gin.Context) {
	password := encryption.Password()
	response := model.LoginResponse{"Success", password}
	c.JSON(http.StatusOK, response)
}
