package login

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-api/model"
	"github.com/pass-wall/passwall-api/pkg/database"
)

var db *gorm.DB
var err error
var repo *model.Repository

func init() {
	db = database.GetDB()
	repo = model.NewRepository(db)
}

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
