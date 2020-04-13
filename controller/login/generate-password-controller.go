package login

import (
	"net/http"

	"github.com/pass-wall/passwall-api/controller/helper"
	"github.com/pass-wall/passwall-api/model"

	"github.com/gin-gonic/gin"
)

// GeneratePassword generates new password
func GeneratePassword(c *gin.Context) {
	password := helper.Password()
	result := model.Result{"Success", password}
	c.JSON(http.StatusOK, result)
}
