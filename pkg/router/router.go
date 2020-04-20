package router

import (
	"log"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/login"
	"github.com/pass-wall/passwall-server/pkg/database"
	"github.com/pass-wall/passwall-server/pkg/middleware"
	"github.com/pass-wall/passwall-server/util"

	"github.com/gin-gonic/gin"
)

// Setup initializes the gin engine and router
func Setup() *gin.Engine {
	r := gin.New()

	// Middlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	db := database.GetDB()
	loginAPI := InitLoginAPI(db)

	// JWT middleware
	authMW := middleware.AuthMiddleware()

	auth := r.Group("/auth")
	{
		auth.POST("/signin", authMW.LoginHandler)
		auth.POST("/check", authMW.MiddlewareFunc(), middleware.TokenCheck)
		auth.POST("/refresh", authMW.MiddlewareFunc(), authMW.RefreshHandler)
	}

	// Endpoints for logins protected with JWT
	logins := r.Group("/logins", authMW.MiddlewareFunc())
	{
		logins.GET("/", loginAPI.FindAll)
		logins.GET("/:id", loginAPI.FindByID)
		logins.POST("/", loginAPI.Create)
		logins.POST("/:action", util.PostHandler)
		logins.PUT("/:id", loginAPI.Update)
		logins.DELETE("/:id", loginAPI.Delete)
	}

	// Protection for route/endpoint scaning
	r.NoRoute(authMW.MiddlewareFunc(), func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		log.Printf("NoRoute claims: %#v\n", claims)
		c.JSON(404, gin.H{"status": "Error", "message": "Page not found"})
	})

	return r
}

// InitLoginAPI ..
func InitLoginAPI(db *gorm.DB) login.LoginAPI {
	loginRepository := login.NewLoginRepository(db)
	loginService := login.NewLoginService(loginRepository)
	loginAPI := login.NewLoginAPI(loginService)
	loginAPI.Migrate()
	return loginAPI
}
