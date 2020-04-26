package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/api/login"
	"github.com/pass-wall/passwall-server/internal/encryption"
)

// GeneratePassword generates new password
func GeneratePassword(c *gin.Context) {
	password := encryption.Password()
	response := login.LoginResponse{"Success", password}
	c.JSON(http.StatusOK, response)
}
