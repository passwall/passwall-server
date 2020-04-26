package router

import (
	"net/http"

	"github.com/pass-wall/passwall-server/api/login"
	"github.com/pass-wall/passwall-server/app"

	"github.com/gin-gonic/gin"
)

// PostHandler ...
func postHandler(c *gin.Context) {
	action := c.Param("action")

	switch action {
	case "import":
		app.Import(c)
	case "export":
		app.Export(c)
	case "backup":
		app.Backup(c)
	case "restore":
		app.Restore(c)
	case "generate-password":
		app.GeneratePassword(c)
	default:
		response := login.LoginResponse{"Error", "Route not found"}
		c.JSON(http.StatusNotFound, response)
	}
}
