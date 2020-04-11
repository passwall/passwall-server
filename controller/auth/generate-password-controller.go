package login

import (
	"net/http"

	"github.com/yakuter/gpass/controller/helper"
	"github.com/yakuter/gpass/model"

	"github.com/gin-gonic/gin"
)

// GeneratePassword generates new password
func GeneratePassword(c *gin.Context) {
	password := helper.Password()
	result := model.Result{"Success", password}
	c.JSON(http.StatusOK, result)
}
