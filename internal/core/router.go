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
	organizationActivityHandler *httpHandler.OrganizationActivityHandler,
	itemHandler *httpHandler.ItemHandler,
	excludedDomainHandler *httpHandler.ExcludedDomainHandler,
	folderHandler *httpHandler.FolderHandler,
	userHandler *httpHandler.UserHandler,
	invitationHandler *httpHandler.InvitationHandler,
	organizationHandler *httpHandler.OrganizationHandler,
	teamHandler *httpHandler.TeamHandler,
	collectionHandler *httpHandler.CollectionHandler,
	organizationItemHandler *httpHandler.OrganizationItemHandler,
	paymentHandler *httpHandler.PaymentHandler,
	webhookHandler *httpHandler.WebhookHandler,
	supportHandler *httpHandler.SupportHandler,
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

	// Stripe webhook endpoint (no auth - verified by Stripe signature)
	router.POST("/webhooks/stripe", webhookHandler.HandleStripeWebhook)

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

		// Support endpoint (authenticated users only)
		apiGroup.POST("/support", supportHandler.SendSupportEmail)

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
		apiGroup.GET("/users/me/rsa-keys", userHandler.CheckRSAKeys)
		apiGroup.POST("/users/me/rsa-keys", userHandler.StoreRSAKeys)
		apiGroup.GET("/users/public-key", userHandler.GetPublicKey) // For org key wrapping

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
			
			// Ownership management for user deletion
			usersGroup.GET("/:id/ownership-check", userHandler.CheckOwnership)
			usersGroup.POST("/:id/transfer-ownership", userHandler.TransferOwnership)
			usersGroup.POST("/:id/delete-with-organizations", userHandler.DeleteWithOrganizations)
		}

		// Invitations - Any authenticated user
		invitationsGroup := apiGroup.Group("/invitations")
		{
			invitationsGroup.POST("", invitationHandler.Invite)           // Create invitation (old /invite endpoint)
			invitationsGroup.GET("/pending", invitationHandler.GetPending) // Get my pending invitations
			invitationsGroup.GET("/sent", invitationHandler.GetSent)       // Get invitations I sent
			invitationsGroup.POST("/:id/accept", invitationHandler.Accept) // Accept invitation
			invitationsGroup.POST("/:id/decline", invitationHandler.Decline) // Decline invitation
		}
		
		// Legacy endpoint (backward compatibility)
		apiGroup.POST("/invite", invitationHandler.Invite)

		// Activity management routes - Admin only
		adminActivitiesGroup := apiGroup.Group("/activities")
		adminActivitiesGroup.Use(httpHandler.RequireAdminMiddleware())
		{
			adminActivitiesGroup.GET("", activityHandler.ListActivities)
		}

		// ============================================================
		// ORGANIZATIONS API
		// ============================================================

		// Organizations CRUD
		orgsGroup := apiGroup.Group("/organizations")
		{
			orgsGroup.POST("", organizationHandler.Create)
			orgsGroup.GET("", organizationHandler.List)
			orgsGroup.GET("/:id", organizationHandler.GetByID)
			orgsGroup.PUT("/:id", organizationHandler.Update)
			orgsGroup.DELETE("/:id", organizationHandler.Delete)

			// Organization activities (visible to org members)
			orgsGroup.GET("/:id/activities", organizationActivityHandler.ListOrganizationActivities)

			// Member management (nested under organization)
			orgsGroup.POST("/:id/members", organizationHandler.InviteUser)
			orgsGroup.GET("/:id/members", organizationHandler.GetMembers)
			orgsGroup.PUT("/:id/members/:userId", organizationHandler.UpdateMemberRole)
			orgsGroup.DELETE("/:id/members/:userId", organizationHandler.RemoveMember)

			// Teams nested under organization
			orgsGroup.POST("/:id/teams", teamHandler.Create)
			orgsGroup.GET("/:id/teams", teamHandler.List)

			// Collections nested under organization
			orgsGroup.POST("/:id/collections", collectionHandler.Create)
			orgsGroup.GET("/:id/collections", collectionHandler.List)

			// Payment & Billing routes
			orgsGroup.POST("/:id/checkout", paymentHandler.CreateCheckoutSession)
			orgsGroup.GET("/:id/billing", paymentHandler.GetBillingInfo)
			orgsGroup.POST("/:id/subscription/cancel", paymentHandler.CancelSubscription)
			orgsGroup.POST("/:id/subscription/reactivate", paymentHandler.ReactivateSubscription)
			orgsGroup.POST("/:id/subscription/sync", paymentHandler.SyncSubscription)
		}

		// Invitation acceptance (not nested)
		apiGroup.POST("/org-invitations/:id/accept", organizationHandler.AcceptInvitation)

		// Teams (direct access by ID)
		teamsGroup := apiGroup.Group("/teams")
		{
			teamsGroup.GET("/:id", teamHandler.GetByID)
			teamsGroup.PUT("/:id", teamHandler.Update)
			teamsGroup.DELETE("/:id", teamHandler.Delete)

			// Team members
			teamsGroup.POST("/:id/members", teamHandler.AddMember)
			teamsGroup.GET("/:id/members", teamHandler.GetMembers)
			teamsGroup.PUT("/:id/members/:memberId", teamHandler.UpdateMember)
			teamsGroup.DELETE("/:id/members/:memberId", teamHandler.RemoveMember)
		}

		// Collections (direct access by ID)
		collectionsGroup := apiGroup.Group("/collections")
		{
			collectionsGroup.GET("/:id", collectionHandler.GetByID)
			collectionsGroup.PUT("/:id", collectionHandler.Update)
			collectionsGroup.DELETE("/:id", collectionHandler.Delete)

			// User access management
			collectionsGroup.PUT("/:id/users/:orgUserId", collectionHandler.GrantUserAccess)
			collectionsGroup.DELETE("/:id/users/:orgUserId", collectionHandler.RevokeUserAccess)
			collectionsGroup.GET("/:id/users", collectionHandler.GetUserAccess)

			// Team access management
			collectionsGroup.PUT("/:id/teams/:teamId", collectionHandler.GrantTeamAccess)
			collectionsGroup.DELETE("/:id/teams/:teamId", collectionHandler.RevokeTeamAccess)
			collectionsGroup.GET("/:id/teams", collectionHandler.GetTeamAccess)
			
			// Collection items (shared vault) - use :id not :collectionId
			collectionsGroup.GET("/:id/items", organizationItemHandler.ListByCollection)
		}

		// Organization Items (direct access)
		orgItemsGroup := apiGroup.Group("/org-items")
		{
			orgItemsGroup.GET("/:id", organizationItemHandler.GetByID)
			orgItemsGroup.PUT("/:id", organizationItemHandler.Update)
			orgItemsGroup.DELETE("/:id", organizationItemHandler.Delete)
		}

		// Create organization item (under organization)
		orgsGroup.POST("/:id/items", organizationItemHandler.Create)
	}

	return router
}
