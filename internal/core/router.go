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
	firewallService service.PolicyFirewallService,
	authHandler *httpHandler.AuthHandler,
	twoFactorHandler *httpHandler.TwoFactorHandler,
	activityHandler *httpHandler.ActivityHandler,
	organizationActivityHandler *httpHandler.OrganizationActivityHandler,
	itemHandler *httpHandler.ItemHandler,
	itemShareHandler *httpHandler.ItemShareHandler,
	excludedDomainHandler *httpHandler.ExcludedDomainHandler,
	userHandler *httpHandler.UserHandler,
	userNotificationPreferencesHandler *httpHandler.UserNotificationPreferencesHandler,
	userAppearancePreferencesHandler *httpHandler.UserAppearancePreferencesHandler,
	userPreferencesHandler *httpHandler.UserPreferencesHandler,
	invitationHandler *httpHandler.InvitationHandler,
	organizationHandler *httpHandler.OrganizationHandler,
	organizationPolicyHandler *httpHandler.OrganizationPolicyHandler,
	organizationSettingsHandler *httpHandler.OrganizationSettingsHandler,
	teamHandler *httpHandler.TeamHandler,
	collectionHandler *httpHandler.CollectionHandler,
	organizationItemHandler *httpHandler.OrganizationItemHandler,
	organizationFolderHandler *httpHandler.OrganizationFolderHandler,
	emergencyAccessHandler *httpHandler.EmergencyAccessHandler,
	sendHandler *httpHandler.SendHandler,
	paymentHandler *httpHandler.PaymentHandler,
	webhookHandler *httpHandler.WebhookHandler,
	supportHandler *httpHandler.SupportHandler,
	plansHandler *httpHandler.PlansHandler,
	adminSubscriptionsHandler *httpHandler.AdminSubscriptionsHandler,
	adminMailHandler *httpHandler.AdminMailHandler,
	adminLogsHandler *httpHandler.AdminLogsHandler,
	iconsHandler *httpHandler.IconsHandler,
	ssoHandler *httpHandler.SSOHandler,
	scimHandler *httpHandler.SCIMHandler,
	scimService service.SCIMService,
	keyEscrowHandler *httpHandler.KeyEscrowHandler,
	breachMonitorHandler *httpHandler.BreachMonitorHandler,
	compromisedCheckHandler *httpHandler.CompromisedCheckHandler,
	compatTelemetryHandler *httpHandler.CompatTelemetryHandler,
	aiTelemetryHandler *httpHandler.AITelemetryHandler,
) *gin.Engine {
	// Create router without default middleware
	router := gin.New()

	// Use our custom logger middleware
	router.Use(logger.GinLogger())
	router.Use(logger.GinRecovery())

	// Global middleware
	router.Use(httpHandler.CORSMiddleware(serverConfig))
	router.Use(httpHandler.SecurityMiddleware())

	// Health check endpoint (no auth required)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Icons endpoint (protected - only Passwall clients allowed)
	// Rate limited: 60 requests per minute per IP (1 request per second, burst of 60)
	iconRateLimiter := httpHandler.NewRateLimiter(1*time.Second, 60)
	router.GET("/icons/:domain",
		httpHandler.IconProtectionMiddleware(),
		httpHandler.RateLimitMiddleware(iconRateLimiter),
		iconsHandler.GetIcon,
	)

	// ============================================================
	// SSO ENDPOINTS (public — no JWT auth, IdP-driven)
	// ============================================================
	ssoGroup := router.Group("/sso")
	{
		ssoGroup.POST("/login", ssoHandler.InitiateLogin)
		ssoGroup.GET("/callback", ssoHandler.OIDCCallback)
		ssoGroup.POST("/callback", ssoHandler.OIDCCallback)
		ssoGroup.GET("/metadata/:connId", ssoHandler.GetSPMetadata)
	}

	// ============================================================
	// SCIM 2.0 ENDPOINTS (authenticated via SCIM bearer token)
	// ============================================================
	scimGroup := router.Group("/scim/v2")
	scimGroup.Use(httpHandler.SCIMAuthMiddleware(scimService))
	{
		scimGroup.GET("/ServiceProviderConfig", scimHandler.ServiceProviderConfig)
		scimGroup.GET("/ResourceTypes", scimHandler.ResourceTypes)

		scimGroup.GET("/Users", scimHandler.ListUsers)
		scimGroup.GET("/Users/:id", scimHandler.GetUser)
		scimGroup.POST("/Users", scimHandler.CreateUser)
		scimGroup.PUT("/Users/:id", scimHandler.UpdateUser)
		scimGroup.PATCH("/Users/:id", scimHandler.PatchUser)
		scimGroup.DELETE("/Users/:id", scimHandler.DeleteUser)

		scimGroup.GET("/Groups", scimHandler.ListGroups)
		scimGroup.GET("/Groups/:id", scimHandler.GetGroup)
		scimGroup.POST("/Groups", scimHandler.CreateGroup)
		scimGroup.PUT("/Groups/:id", scimHandler.UpdateGroup)
		scimGroup.PATCH("/Groups/:id", scimHandler.PatchGroup)
		scimGroup.DELETE("/Groups/:id", scimHandler.DeleteGroup)
	}

	// Stripe webhook endpoint (no auth - verified by Stripe signature)
	router.POST("/webhooks/stripe", webhookHandler.HandleStripeWebhook)

	// RevenueCat webhook endpoint (no auth - verified by RevenueCat signature)
	// Used for mobile in-app purchases (iOS App Store, Google Play Store)
	router.POST("/webhooks/revenuecat", webhookHandler.HandleRevenueCatWebhook)

	// Rate limiters for auth endpoints
	// SignIn/SignUp: 5 requests per minute per IP (prevents brute force)
	authRateLimiter := httpHandler.NewRateLimiter(12*time.Second, 5)
	// Refresh token: 10 requests per minute per IP
	refreshRateLimiter := httpHandler.NewRateLimiter(6*time.Second, 10)
	// Verification: 3 requests per 5 minutes per IP
	verificationRateLimiter := httpHandler.NewRateLimiter(100*time.Second, 3)
	// Password change: 3 requests per 5 minutes per IP (limits brute-force of old master password hash)
	changePasswordRateLimiter := httpHandler.NewRateLimiter(100*time.Second, 3)

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
			recaptchaMiddleware,
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

		// Two-Factor Authentication verification (during sign-in, no JWT auth)
		authGroup.POST("/2fa/verify",
			httpHandler.RateLimitMiddleware(authRateLimiter),
			twoFactorHandler.Verify,
		)
	}

	// Public Secure Send access (no auth required — recipients don't need accounts)
	publicSendsGroup := router.Group("/api/sends/access")
	{
		publicSendsGroup.GET("/:access_id", sendHandler.Access)
		publicSendsGroup.POST("/:access_id/password", sendHandler.VerifyPassword)
	}

	// Compatibility telemetry ingest — requires authentication so only
	// real Passwall users can submit telemetry data.
	telemetryGroup := router.Group("/api")
	telemetryGroup.Use(httpHandler.AuthMiddleware(authService))
	{
		telemetryGroup.POST("/telemetry/compat", compatTelemetryHandler.Ingest)
	}

	// API routes (require authentication)
	apiGroup := router.Group("/api")
	apiGroup.Use(httpHandler.AuthMiddleware(authService))
	{
		// Plans (authenticated)
		apiGroup.GET("/plans", plansHandler.ListPlans)
		apiGroup.GET("/plans/:code", plansHandler.GetPlan)

		// Policy & settings definitions catalog (authenticated, no org context needed)
		apiGroup.GET("/policies/definitions", organizationPolicyHandler.ListPolicyDefinitions)
		apiGroup.GET("/settings/definitions", organizationSettingsHandler.ListSettingsDefinitions)

		// Compromised password check (batch SHA-1 hash check via HIBP Pwned Passwords)
		apiGroup.POST("/compromised-check", compromisedCheckHandler.BatchCheck)

		// Auth protected routes
		apiGroup.POST("/signout", authHandler.SignOut)

		// Support endpoint (authenticated users only)
		apiGroup.POST("/support", supportHandler.SendSupportEmail)

		// NOTE: telemetry ingest moved above apiGroup (optional auth).

		// Modern Items API (unified endpoint for all types)
		apiGroup.POST("/items", itemHandler.Create)
		apiGroup.GET("/items", itemHandler.List)
		apiGroup.GET("/items/:id", itemHandler.GetByID)
		apiGroup.PUT("/items/:id", itemHandler.Update)
		apiGroup.DELETE("/items/:id", itemHandler.Delete)

		// Personal item sharing (zero-knowledge)
		apiGroup.POST("/item-shares", itemShareHandler.Create)
		apiGroup.GET("/item-shares", itemShareHandler.ListOwned)
		apiGroup.GET("/item-shares/received", itemShareHandler.ListReceived)
		apiGroup.GET("/item-shares/:uuid", itemShareHandler.GetByUUID)
		apiGroup.PUT("/item-shares/:uuid/item", itemShareHandler.UpdateSharedItem)
		apiGroup.PATCH("/item-shares/:uuid/permissions", itemShareHandler.UpdatePermissions)
		apiGroup.POST("/item-shares/:uuid/re-share", itemShareHandler.ReShare)
		apiGroup.DELETE("/item-shares/:id", itemShareHandler.Revoke)

		// Emergency Access
		eaGroup := apiGroup.Group("/emergency-access")
		{
			eaGroup.POST("", emergencyAccessHandler.Invite)
			eaGroup.GET("/granted", emergencyAccessHandler.ListGranted)
			eaGroup.GET("/trusted", emergencyAccessHandler.ListTrusted)
			eaGroup.POST("/:uuid/accept", emergencyAccessHandler.Accept)
			eaGroup.POST("/:uuid/confirm", emergencyAccessHandler.Confirm)
			eaGroup.POST("/:uuid/request", emergencyAccessHandler.RequestRecovery)
			eaGroup.POST("/:uuid/approve", emergencyAccessHandler.ApproveRecovery)
			eaGroup.POST("/:uuid/reject", emergencyAccessHandler.RejectRecovery)
			eaGroup.DELETE("/:uuid", emergencyAccessHandler.RevokeAccess)
			eaGroup.GET("/:uuid/vault", emergencyAccessHandler.GetVault)
		}

		// Secure Send
		sendsGroup := apiGroup.Group("/sends")
		{
			sendsGroup.POST("", sendHandler.Create)
			sendsGroup.GET("", sendHandler.List)
			sendsGroup.GET("/:uuid", sendHandler.GetByUUID)
			sendsGroup.PUT("/:uuid", sendHandler.Update)
			sendsGroup.DELETE("/:uuid", sendHandler.Delete)
			sendsGroup.POST("/:uuid/notify", sendHandler.Notify)
		}

		// Excluded Domains API (for "Turn off Passwall for this site")
		apiGroup.GET("/excluded-domains", excludedDomainHandler.List)
		apiGroup.POST("/excluded-domains", excludedDomainHandler.Create)
		apiGroup.DELETE("/excluded-domains/:id", excludedDomainHandler.Delete)
		apiGroup.DELETE("/excluded-domains/by-domain/:domain", excludedDomainHandler.DeleteByDomain)
		apiGroup.GET("/excluded-domains/check/:domain", excludedDomainHandler.Check)

		// NOTE: All legacy endpoints (logins, credit-cards, bank-accounts, notes, emails, servers)
		// have been migrated to the modern /api/items endpoint.
		// Use /api/items with type parameter: ?type=1 (password), ?type=2 (note), ?type=3 (card), etc.

		// User profile routes - any authenticated user
		apiGroup.PUT("/users/me", userHandler.UpdateProfile)
		apiGroup.GET("/users/me/notification-preferences", userNotificationPreferencesHandler.Get)
		apiGroup.PUT("/users/me/notification-preferences", userNotificationPreferencesHandler.Update)
		apiGroup.GET("/users/me/appearance-preferences", userAppearancePreferencesHandler.Get)
		apiGroup.PUT("/users/me/appearance-preferences", userAppearancePreferencesHandler.Update)
		apiGroup.GET("/users/me/preferences", userPreferencesHandler.List)
		apiGroup.PUT("/users/me/preferences", userPreferencesHandler.Upsert)
		apiGroup.POST("/users/change-master-password",
			httpHandler.RateLimitMiddleware(changePasswordRateLimiter),
			authHandler.ChangeMasterPassword,
		)

		// Two-Factor Authentication management (authenticated)
		twoFactorGroup := apiGroup.Group("/users/me/2fa")
		{
			twoFactorGroup.GET("/status", twoFactorHandler.Status)
			twoFactorGroup.POST("/setup", twoFactorHandler.Setup)
			twoFactorGroup.POST("/confirm", twoFactorHandler.Confirm)
			twoFactorGroup.POST("/disable", twoFactorHandler.Disable)
		}
		apiGroup.GET("/users/me/rsa-keys", userHandler.CheckRSAKeys)
		apiGroup.GET("/users/me/rsa-private-key", userHandler.GetRSAPrivateKeyEnc)
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
			invitationsGroup.POST("", invitationHandler.Invite)              // Create invitation (old /invite endpoint)
			invitationsGroup.GET("/pending", invitationHandler.GetPending)   // Get my pending invitations
			invitationsGroup.GET("/sent", invitationHandler.GetSent)         // Get invitations I sent
			invitationsGroup.POST("/:id/accept", invitationHandler.Accept)   // Accept invitation
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

		// Admin subscription management (Admin only)
		adminGroup := apiGroup.Group("/admin")
		adminGroup.Use(httpHandler.RequireAdminMiddleware())
		{
			adminGroup.GET("/organizations", adminSubscriptionsHandler.ListOrganizations)
			adminGroup.GET("/subscriptions", adminSubscriptionsHandler.List)
			adminGroup.POST("/organizations/:id/subscription/grant", adminSubscriptionsHandler.GrantManual)
			adminGroup.POST("/organizations/:id/subscription/revoke", adminSubscriptionsHandler.RevokeManual)
			// Mail (admin broadcast)
			adminGroup.POST("/mail", adminMailHandler.CreateJob)
			adminGroup.GET("/mail/:jobId", adminMailHandler.GetJob)
			// Server logs (admin)
			adminGroup.GET("/logs", adminLogsHandler.List)
			adminGroup.GET("/logs/download", adminLogsHandler.Download)
			adminGroup.GET("/logs/download-bundle", adminLogsHandler.DownloadBundle)
			adminGroup.POST("/logs/clear", adminLogsHandler.Clear)
			adminGroup.GET("/telemetry/compat", compatTelemetryHandler.ListAdmin)
			adminGroup.GET("/telemetry/compat/summary", compatTelemetryHandler.ListSummaryAdmin)
			adminGroup.POST("/telemetry/compat/cleanup", compatTelemetryHandler.CleanupAdmin)
			adminGroup.GET("/telemetry/compat/analyze", aiTelemetryHandler.Analyze)
			adminGroup.GET("/telemetry/compat/analyze/verdicts", aiTelemetryHandler.ListVerdicts)
			adminGroup.DELETE("/telemetry/compat/analyze/verdicts", aiTelemetryHandler.ResetVerdicts)

			// Custom icons management (admin only)
			adminGroup.GET("/icons", iconsHandler.ListCustomIcons)
			adminGroup.POST("/icons/:domain", iconsHandler.UploadCustomIcon)
			adminGroup.DELETE("/icons/:domain", iconsHandler.DeleteCustomIcon)

			// Legacy endpoint (backward compatibility)
			adminGroup.POST("/bulk-email", adminMailHandler.CreateJob)
			adminGroup.GET("/bulk-email/:jobId", adminMailHandler.GetJob)
		}

		// ============================================================
		// ORGANIZATIONS API
		// ============================================================

		// Organizations CRUD (firewall middleware checks IP-based access per org)
		orgsGroup := apiGroup.Group("/organizations")
		orgsGroup.Use(httpHandler.FirewallMiddleware(firewallService))
		{
			orgsGroup.POST("", organizationHandler.Create)
			orgsGroup.GET("", organizationHandler.List)
			orgsGroup.GET("/:id", organizationHandler.GetByID)
			orgsGroup.PUT("/:id", organizationHandler.Update)
			orgsGroup.DELETE("/:id", organizationHandler.Delete)

			// Organization activities (visible to org members)
			orgsGroup.GET("/:id/activities", organizationActivityHandler.ListOrganizationActivities)

			// Organization items
			orgsGroup.GET("/:id/items", organizationItemHandler.ListByOrganization)

			// Organization folders
			orgsGroup.GET("/:id/folders", organizationFolderHandler.ListByOrganization)
			orgsGroup.POST("/:id/folders", organizationFolderHandler.Create)
			orgsGroup.PUT("/:id/folders/:folderId", organizationFolderHandler.Update)
			orgsGroup.DELETE("/:id/folders/:folderId", organizationFolderHandler.Delete)

			// Member management (nested under organization)
			orgsGroup.POST("/:id/members", organizationHandler.InviteUser)
			orgsGroup.GET("/:id/members", organizationHandler.GetMembers)
			orgsGroup.PUT("/:id/members/:userId", organizationHandler.UpdateMemberRole)
			orgsGroup.DELETE("/:id/members/:userId", organizationHandler.RemoveMember)
			orgsGroup.POST("/:id/members/:userId/confirm", organizationHandler.ConfirmProvisionedMember)

			// Teams nested under organization
			orgsGroup.POST("/:id/teams", teamHandler.Create)
			orgsGroup.GET("/:id/teams", teamHandler.List)

			// Collections nested under organization
			orgsGroup.POST("/:id/collections", collectionHandler.Create)
			orgsGroup.GET("/:id/collections", collectionHandler.List)

			// Organization settings (preferences)
			orgsGroup.GET("/:id/settings", organizationSettingsHandler.ListSettings)
			orgsGroup.PUT("/:id/settings", organizationSettingsHandler.UpsertSettings)

			// Organization policies
			orgsGroup.GET("/:id/policies", organizationPolicyHandler.ListPolicies)
			orgsGroup.GET("/:id/policies/active", organizationPolicyHandler.GetActivePolicies)
			orgsGroup.GET("/:id/policies/:policyType", organizationPolicyHandler.GetPolicy)
			orgsGroup.PUT("/:id/policies/:policyType", organizationPolicyHandler.UpdatePolicy)

			// 2FA compliance (org admin dashboard)
			orgsGroup.GET("/:id/2fa-compliance", twoFactorHandler.Compliance)

			// SSO connection management (org admin)
			orgsGroup.POST("/:id/sso", ssoHandler.CreateConnection)
			orgsGroup.GET("/:id/sso", ssoHandler.ListConnections)
			orgsGroup.GET("/:id/sso/:connId", ssoHandler.GetConnection)
			orgsGroup.PUT("/:id/sso/:connId", ssoHandler.UpdateConnection)
			orgsGroup.DELETE("/:id/sso/:connId", ssoHandler.DeleteConnection)
			orgsGroup.POST("/:id/sso/:connId/activate", ssoHandler.ActivateConnection)

			// Key Escrow (SSO passwordless vault unlock)
			orgsGroup.POST("/:id/key-escrow/enroll", keyEscrowHandler.Enroll)
			orgsGroup.GET("/:id/key-escrow/status", keyEscrowHandler.GetStatus)
			orgsGroup.DELETE("/:id/key-escrow/users/:userId", keyEscrowHandler.Revoke)

			// SCIM token management (org admin)
			orgsGroup.POST("/:id/scim/tokens", scimHandler.CreateToken)
			orgsGroup.GET("/:id/scim/tokens", scimHandler.ListTokens)
			orgsGroup.DELETE("/:id/scim/tokens/:tokenId", scimHandler.RevokeToken)

			// Breach Monitoring (dark web monitoring)
			breachMonitorGroup := orgsGroup.Group("/:id/breach-monitor")
			{
				breachMonitorGroup.POST("/emails", breachMonitorHandler.AddEmail)
				breachMonitorGroup.GET("/emails", breachMonitorHandler.ListEmails)
				breachMonitorGroup.DELETE("/emails/:emailId", breachMonitorHandler.RemoveEmail)
				breachMonitorGroup.POST("/check", breachMonitorHandler.CheckEmails)
				breachMonitorGroup.GET("/breaches", breachMonitorHandler.ListBreaches)
				breachMonitorGroup.PATCH("/breaches/:breachId/dismiss", breachMonitorHandler.DismissBreach)
				breachMonitorGroup.GET("/summary", breachMonitorHandler.GetSummary)
			}

			// Payment & Billing routes
			orgsGroup.POST("/:id/checkout", paymentHandler.CreateCheckoutSession)
			orgsGroup.GET("/:id/billing", paymentHandler.GetBillingInfo)
			orgsGroup.POST("/:id/subscription/seats/preview", paymentHandler.PreviewSeatChange)
			orgsGroup.POST("/:id/subscription/seats", paymentHandler.UpdateSubscriptionSeats)
			orgsGroup.POST("/:id/subscription/change/preview", paymentHandler.PreviewPlanChange)
			orgsGroup.POST("/:id/subscription/change", paymentHandler.ChangePlan)
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
