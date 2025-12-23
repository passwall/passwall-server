package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
	httpHandler "github.com/passwall/passwall-server/internal/handler/http"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/logger"
)

// SetupRouter configures all application routes
func SetupRouter(
	authService service.AuthService,
	authHandler *httpHandler.AuthHandler,
	loginHandler *httpHandler.LoginHandler,
	bankAccountHandler *httpHandler.BankAccountHandler,
	creditCardHandler *httpHandler.CreditCardHandler,
	noteHandler *httpHandler.NoteHandler,
	emailHandler *httpHandler.EmailHandler,
	serverHandler *httpHandler.ServerHandler,
	userHandler *httpHandler.UserHandler,
) *gin.Engine {
	// Create router without default middleware
	router := gin.New()

	// Use our custom logger middleware
	router.Use(logger.GinLogger())
	router.Use(logger.GinRecovery())

	// Global middleware
	router.Use(httpHandler.CORSMiddleware())
	router.Use(httpHandler.SecurityMiddleware())

	// Health check endpoint (no auth required)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Auth routes (no auth middleware)
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/signup", authHandler.SignUp)
		authGroup.POST("/signin", authHandler.SignIn)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/check", authHandler.CheckToken)
	}

	// API routes (require authentication)
	apiGroup := router.Group("/api")
	apiGroup.Use(httpHandler.AuthMiddleware(authService))
	{
		// Auth protected routes
		apiGroup.POST("/signout", authHandler.SignOut)

		// Login routes
		apiGroup.GET("/logins", loginHandler.List)
		apiGroup.GET("/logins/:id", loginHandler.GetByID)
		apiGroup.POST("/logins", loginHandler.Create)
		apiGroup.PUT("/logins/:id", loginHandler.Update)
		apiGroup.DELETE("/logins/:id", loginHandler.Delete)
		apiGroup.PUT("/logins/bulk-update", loginHandler.BulkUpdate)

		// Bank account routes
		apiGroup.GET("/bank-accounts", bankAccountHandler.List)
		apiGroup.GET("/bank-accounts/:id", bankAccountHandler.GetByID)
		apiGroup.POST("/bank-accounts", bankAccountHandler.Create)
		apiGroup.PUT("/bank-accounts/:id", bankAccountHandler.Update)
		apiGroup.DELETE("/bank-accounts/:id", bankAccountHandler.Delete)
		apiGroup.PUT("/bank-accounts/bulk-update", bankAccountHandler.BulkUpdate)

		// Credit card routes
		apiGroup.GET("/credit-cards", creditCardHandler.List)
		apiGroup.GET("/credit-cards/:id", creditCardHandler.GetByID)
		apiGroup.POST("/credit-cards", creditCardHandler.Create)
		apiGroup.PUT("/credit-cards/:id", creditCardHandler.Update)
		apiGroup.DELETE("/credit-cards/:id", creditCardHandler.Delete)
		apiGroup.PUT("/credit-cards/bulk-update", creditCardHandler.BulkUpdate)

		// Note routes
		apiGroup.GET("/notes", noteHandler.List)
		apiGroup.GET("/notes/:id", noteHandler.GetByID)
		apiGroup.POST("/notes", noteHandler.Create)
		apiGroup.PUT("/notes/:id", noteHandler.Update)
		apiGroup.DELETE("/notes/:id", noteHandler.Delete)
		apiGroup.PUT("/notes/bulk-update", noteHandler.BulkUpdate)

		// Email routes
		apiGroup.GET("/emails", emailHandler.List)
		apiGroup.GET("/emails/:id", emailHandler.GetByID)
		apiGroup.POST("/emails", emailHandler.Create)
		apiGroup.PUT("/emails/:id", emailHandler.Update)
		apiGroup.DELETE("/emails/:id", emailHandler.Delete)
		apiGroup.PUT("/emails/bulk-update", emailHandler.BulkUpdate)

		// Server routes
		apiGroup.GET("/servers", serverHandler.List)
		apiGroup.GET("/servers/:id", serverHandler.GetByID)
		apiGroup.POST("/servers", serverHandler.Create)
		apiGroup.PUT("/servers/:id", serverHandler.Update)
		apiGroup.DELETE("/servers/:id", serverHandler.Delete)
		apiGroup.PUT("/servers/bulk-update", serverHandler.BulkUpdate)

		// User routes
		apiGroup.GET("/users/:id", userHandler.GetByID)
		apiGroup.PUT("/users/:id", userHandler.Update)
		apiGroup.DELETE("/users/:id", userHandler.Delete)
		apiGroup.POST("/users/change-master-password", userHandler.ChangeMasterPassword)
	}

	return router
}
