package router

import (
	"github.com/gin-contrib/secure"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/login"
	"github.com/pass-wall/passwall-server/pkg/database"
	"github.com/pass-wall/passwall-server/pkg/middleware"
	"github.com/pass-wall/passwall-server/util"
)

// Setup initializes the gin engine and router
func Setup() *gin.Engine {
	r := gin.New()

	// Middlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(secure.New(secureConfig()))

	// Serve static files in public folder
	r.Use(static.Serve("/", static.LocalFile("./public", true)))

	db := database.GetDB()
	loginAPI := InitLoginAPI(db)

	// JWT middleware
	authMW := middleware.AuthMiddleware()

	auth := r.Group("/auth")
	{
		auth.POST("/signin", middleware.LimiterMW(), authMW.LoginHandler)
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
	// r.NoRoute(authMW.MiddlewareFunc(), func(c *gin.Context) {
	// 	claims := jwt.ExtractClaims(c)
	// 	log.Printf("NoRoute claims: %#v\n", claims)
	// 	c.JSON(404, gin.H{"Status": "Error", "Message": "Page not found"})
	// })

	r.NoRoute(func(c *gin.Context) {
		c.File("./public/index.html")
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

func secureConfig() secure.Config {
	// Details about this config is here
	// https://github.com/gin-contrib/secure/blob/master/secure.go
	return secure.Config{
		// AllowedHosts:          []string{"example.com", "ssl.example.com"},
		// SSLRedirect:           false,
		// SSLHost:               "ssl.example.com",
		STSSeconds:            315360000,
		STSIncludeSubdomains:  true,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ContentSecurityPolicy: "default-src 'self' 'unsafe-inline' 'unsafe-eval'; connect-src *",
		IENoOpen:              true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
	}
}
