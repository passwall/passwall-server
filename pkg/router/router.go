package router

import (
	"log"

	jwt "github.com/yakuter/gin-jwt"
	"github.com/yakuter/gpass/controller/login"
	"github.com/yakuter/gpass/pkg/middleware"

	"github.com/gin-gonic/gin"
)

// Setup initializes the gin engine and router
func Setup() *gin.Engine {
	r := gin.New()

	// Middlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// JWT middleware
	authMW := middleware.AuthMiddleware()

	auth := r.Group("/auth")
	{
		auth.POST("/signin", authMW.LoginHandler)
		auth.POST("/check", authMW.MiddlewareFunc(), middleware.TokenCheck)
		auth.POST("/refresh", authMW.MiddlewareFunc(), authMW.RefreshHandler)
	}

	// Refresh time can be longer than token timeout

	// Protection for route/endpoint scaning
	r.NoRoute(authMW.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		log.Printf("NoRoute claims: %#v\n", claims)
		c.JSON(404, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})

	// Endpoints for logins protected with JWT
	logins := r.Group("/logins", authMW.MiddlewareFunc())
	{
		logins.GET("/", login.GetLogins)
		logins.GET("/:id", login.GetLogin)
		logins.POST("/", login.CreateLogin)
		logins.POST("/:action", login.PostHandler)
		logins.PUT("/:id", login.UpdateLogin)
		logins.DELETE("/:id", login.DeleteLogin)
	}

	// gpass uses gin.BasicAuth() middleware to secure routes
	// You can change username and password in config.yml
	// Don't forget to add Basic Auth authorization to your HTTP requests
	// usersMap := map[string]string{
	// 	config.Server.Username: config.Server.Password,
	// }

	// authorized := r.Group("/", gin.BasicAuth(usersMap))
	// logins := authorized.Group("/logins")
	// {
	// 	logins.GET("/", login.GetLogins)
	// 	logins.GET("/:id", login.GetLogin)
	// 	logins.POST("/", login.CreateLogin)
	// 	logins.POST("/:action", login.PostHandler)
	// 	logins.PUT("/:id", login.UpdateLogin)
	// 	logins.DELETE("/:id", login.DeleteLogin)
	// }

	return r
}
