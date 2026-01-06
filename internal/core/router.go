package core

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/config"
	httpHandler "github.com/passwall/passwall-server/internal/handler/http"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/logger"
)

// SetupRouter configures all application routes
func SetupRouter(
	serverConfig *config.ServerConfig,
	authService service.AuthService,
	authHandler *httpHandler.AuthHandler,
	activityHandler *httpHandler.ActivityHandler,
	itemHandler *httpHandler.ItemHandler,
	excludedDomainHandler *httpHandler.ExcludedDomainHandler,
	folderHandler *httpHandler.FolderHandler,
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

	// Rate limiters for auth endpoints
	// SignIn/SignUp: 5 requests per minute per IP (prevents brute force)
	authRateLimiter := httpHandler.NewRateLimiter(12*time.Second, 5)
	// Refresh token: 10 requests per minute per IP
	refreshRateLimiter := httpHandler.NewRateLimiter(6*time.Second, 10)
	// Verification: 3 requests per 5 minutes per IP
	verificationRateLimiter := httpHandler.NewRateLimiter(100*time.Second, 3)

	// Create reCAPTCHA middleware (optional - only applies if token is sent)
	recaptchaMiddleware := httpHandler.OptionalRecaptchaMiddleware(
		serverConfig.RecaptchaSecretKey,
		serverConfig.RecaptchaThreshold,
	)

	// Auth routes (no auth middleware)
	authGroup := router.Group("/auth")
	{
		// PreLogin endpoint (get KDF config before signin)
		// No rate limit - needed for every login attempt
		authGroup.GET("/prelogin", authHandler.PreLogin)

		// Rate-limited endpoints with optional reCAPTCHA
		authGroup.POST("/signup",
			httpHandler.RateLimitMiddleware(authRateLimiter),
			recaptchaMiddleware,
			authHandler.SignUp,
		)
		authGroup.POST("/signin",
			httpHandler.RateLimitMiddleware(authRateLimiter),
			authHandler.SignIn,
		)
		authGroup.POST("/refresh",
			httpHandler.RateLimitMiddleware(refreshRateLimiter),
			authHandler.RefreshToken,
		)

		// Email verification endpoints
		authGroup.GET("/verify/:code", authHandler.VerifyEmail)
		authGroup.POST("/resend-verification",
			httpHandler.RateLimitMiddleware(verificationRateLimiter),
			authHandler.ResendVerificationCode,
		)

		// No rate limit on token check (it's already authenticated)
		authGroup.POST("/check", authHandler.CheckToken)
	}

	// API routes (require authentication)
	apiGroup := router.Group("/api")
	apiGroup.Use(httpHandler.AuthMiddleware(authService))
	{
		// Auth protected routes
		apiGroup.POST("/signout", authHandler.SignOut)

		// Modern Items API (unified endpoint for all types)
		apiGroup.POST("/items", itemHandler.Create)
		apiGroup.GET("/items", itemHandler.List)
		apiGroup.GET("/items/:id", itemHandler.GetByID)
		apiGroup.PUT("/items/:id", itemHandler.Update)
		apiGroup.DELETE("/items/:id", itemHandler.Delete)

		// Excluded Domains API (for "Turn off Passwall for this site")
		apiGroup.GET("/excluded-domains", excludedDomainHandler.List)
		apiGroup.POST("/excluded-domains", excludedDomainHandler.Create)
		apiGroup.DELETE("/excluded-domains/:id", excludedDomainHandler.Delete)
		apiGroup.DELETE("/excluded-domains/by-domain/:domain", excludedDomainHandler.DeleteByDomain)
		apiGroup.GET("/excluded-domains/check/:domain", excludedDomainHandler.Check)

		// Folders API (for organizing vault items)
		apiGroup.GET("/folders", folderHandler.List)
		apiGroup.POST("/folders", folderHandler.Create)
		apiGroup.PUT("/folders/:id", folderHandler.Update)
		apiGroup.DELETE("/folders/:id", folderHandler.Delete)

		// NOTE: All legacy endpoints (logins, credit-cards, bank-accounts, notes, emails, servers)
		// have been migrated to the modern /api/items endpoint.
		// Use /api/items with type parameter: ?type=1 (password), ?type=2 (note), ?type=3 (card), etc.

		// User profile routes - any authenticated user
		apiGroup.PUT("/users/me", userHandler.UpdateProfile)
		apiGroup.POST("/users/change-master-password", authHandler.ChangeMasterPassword)

		// Activity routes - any authenticated user
		apiGroup.GET("/activities/me", activityHandler.GetMyActivities)
		apiGroup.GET("/activities/last-signin", activityHandler.GetLastSignIn)

		// User management routes - Admin only
		usersGroup := apiGroup.Group("/users")
		usersGroup.Use(httpHandler.RequireAdminMiddleware())
		{
			usersGroup.GET("", userHandler.List)
			usersGroup.GET("/:id", userHandler.GetByID)
			usersGroup.POST("", userHandler.Create)
			usersGroup.PUT("/:id", userHandler.Update)
			usersGroup.DELETE("/:id", userHandler.Delete)
			usersGroup.GET("/:id/activities", activityHandler.GetUserActivities)
		}

		// Activity management routes - Admin only
		adminActivitiesGroup := apiGroup.Group("/activities")
		adminActivitiesGroup.Use(httpHandler.RequireAdminMiddleware())
		{
			adminActivitiesGroup.GET("", activityHandler.ListActivities)
		}
	}

	return router
}
