package router

import (
	"github.com/yakuter/gpass/controller/login"
	"github.com/yakuter/gpass/pkg/config"
	"github.com/yakuter/gpass/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func Setup() *gin.Engine {
	config := config.GetConfig()
	r := gin.New()

	// Middlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// gpass uses gin.BasicAuth() middleware to secure routes
	// You can change username and password in config.yml
	// Don't forget to add Basic Auth authorization to your HTTP requests
	usersMap := map[string]string{
		config.Server.Username: config.Server.Password,
	}
	authorized := r.Group("/", gin.BasicAuth(usersMap))
	logins := authorized.Group("/logins")
	{
		logins.GET("/", login.GetLogins)
		logins.GET("/:id", login.GetLogin)
		logins.POST("/", login.CreateLogin)
		logins.PUT("/:id", login.UpdateLogin)
		logins.DELETE("/:id", login.DeleteLogin)
	}

	return r
}
