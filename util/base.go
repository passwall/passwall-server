package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pass-wall/passwall-server/api/login"
)

// PostHandler ...
func PostHandler(c *gin.Context) {
	action := c.Param("action")

	switch action {
	case "import":
		Import(c)
	case "export":
		Export(c)
	case "backup":
		Backup(c)
	case "restore":
		Restore(c)
	case "generate-password":
		GeneratePassword(c)
	default:
		response := login.LoginResponse{"Error", "Route not found"}
		c.JSON(http.StatusNotFound, response)
	}
}
