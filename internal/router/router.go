package router

import (
	"github.com/gin-contrib/secure"
	"github.com/gorilla/mux"
	// "github.com/jinzhu/gorm"
)

// Setup initializes the gin engine and router
func Setup() *mux.Router {
	// r := gin.New()
	r := mux.NewRouter().StrictSlash(true)

	// Middlewares
	// r.Use(gin.Logger())
	// r.Use(gin.Recovery())
	// r.Use(middleware.CORS())
	// r.Use(secure.New(secureConfig()))

	// Serve static files in public folder
	// r.Use(static.Serve("/", static.LocalFile("./public", true)))
	// r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(dir))))

	// db := store.GetDB()
	// loginAPI := InitLoginAPI(db)

	// JWT middleware
	// authMW := middleware.AuthMiddleware()

	// auth := r.Group("/auth")
	// {
	// 	auth.POST("/signin", middleware.LimiterMW(), authMW.LoginHandler)
	// 	auth.POST("/check", authMW.MiddlewareFunc(), middleware.TokenCheck)
	// 	auth.POST("/refresh", authMW.MiddlewareFunc(), authMW.RefreshHandler)
	// }

	// auth := r.PathPrefix("/auth").Subrouter()
	// auth.HandleFunc("/signin", createPost).Methods("POST")
	// auth.HandleFunc("/check", createPost).Methods("POST")
	// auth.HandleFunc("/refresh", createPost).Methods("POST")

	// r.HandleFunc("/logins", loginAPI.Test).Methods("GET")

	// Endpoints for logins protected with JWT
	/* logins := r.Group("/logins", authMW.MiddlewareFunc())
	{
		logins.GET("/", loginAPI.FindAll)
		logins.GET("/:id", loginAPI.FindByID)
		logins.POST("/", loginAPI.Create)
		logins.POST("/:action", func(w http.ResponseWriter, r *http.Request) {
			path := c.Param("action")
			if path == "check-password" {
				loginAPI.FindSamePassword(c)
			} else {
				postHandler(c)
			}
		})

		logins.PUT("/:id", loginAPI.Update)
		logins.DELETE("/:id", loginAPI.Delete)

	}

	r.NoRoute(func(w http.ResponseWriter, r *http.Request) {
		c.File("./public/index.html")
	}) */

	return r
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
