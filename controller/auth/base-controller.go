package login

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/yakuter/gpass/model"
)

var db *gorm.DB
var err error

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
		result := model.Result{"Error", "Route not found"}
		c.JSON(http.StatusNotFound, result)
	}
}
