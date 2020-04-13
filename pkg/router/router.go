package router

import (
	"log"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-api/login"
	"github.com/pass-wall/passwall-api/pkg/database"
	"github.com/pass-wall/passwall-api/pkg/middleware"

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
		logins.GET("/", loginAPI.FindAll)
		logins.GET("/:id", loginAPI.FindByID)
		logins.POST("/", loginAPI.Create)
		// logins.POST("/:action", loginAPI.PostHandler)
		logins.PUT("/:id", loginAPI.Update)
		logins.DELETE("/:id", loginAPI.Delete)
	}

	return r
}

// InitLoginAPI ..
func InitLoginAPI(db *gorm.DB) login.LoginAPI {
	loginRepository := login.NewLoginRepository(db)
	loginService := login.NewLoginService(loginRepository)
	loginAPI := login.NewLoginAPI(loginService)
	return loginAPI
}
