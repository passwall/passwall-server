package router

import (
	"gpass/controller"
	"gpass/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func Setup() *gin.Engine {
	r := gin.New()

	// Middlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// Non-protected routes
	logins := r.Group("/logins")
	{
		logins.GET("/", controller.GetLogins)
		logins.GET("/:id", controller.GetLogin)
		logins.POST("/", controller.CreateLogin)
		logins.PUT("/:id", controller.UpdateLogin)
		logins.DELETE("/:id", controller.DeleteLogin)
	}

	// Protected routes
	// For authorized access, group protected routes using gin.BasicAuth() middleware
	// gin.Accounts is a shortcut for map[string]string
	authorized := r.Group("/admin", gin.BasicAuth(gin.Accounts{
		"username1": "password1",
		"username2": "password2",
		"username3": "password3",
	}))

	// /admin/dashboard endpoint is now protected
	authorized.GET("/dashboard", controller.Dashboard)

	return r
}
