package core

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	httpHandler "github.com/passwall/passwall-server/internal/handler/http"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/logger"
)

// SetupRouter configures all application routes
func SetupRouter(
	serverConfig *config.ServerConfig,
	authService service.AuthService,
	firewallService service.PolicyFirewallService,
	orgRepo repository.OrganizationRepository,
	userRepo repository.UserRepository,
	authHandler *httpHandler.AuthHandler,
	twoFactorHandler *httpHandler.TwoFactorHandler,
	activityHandler *httpHandler.ActivityHandler,
	organizationActivityHandler *httpHandler.OrganizationActivityHandler,
	itemShareHandler *httpHandler.ItemShareHandler,
	excludedDomainHandler *httpHandler.ExcludedDomainHandler,
	userHandler *httpHandler.UserHandler,
	userNotificationPreferencesHandler *httpHandler.UserNotificationPreferencesHandler,
	userAppearancePreferencesHandler *httpHandler.UserAppearancePreferencesHandler,
	userPreferencesHandler *httpHandler.UserPreferencesHandler,
	invitationHandler *httpHandler.InvitationHandler,
	organizationHandler *httpHandler.OrganizationHandler,
	entitlementHandler *httpHandler.EntitlementHandler,
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
	adminStepUpHandler *httpHandler.AdminStepUpHandler,
	adminAuditLogger *service.ActivityLogger,
	adminDirectoryHandler *httpHandler.AdminDirectoryHandler,
	iconsHandler *httpHandler.IconsHandler,
	ssoHandler *httpHandler.SSOHandler,
	scimHandler *httpHandler.SCIMHandler,
	scimService service.SCIMService,
	keyEscrowHandler *httpHandler.KeyEscrowHandler,
	breachMonitorHandler *httpHandler.BreachMonitorHandler,
	compromisedCheckHandler *httpHandler.CompromisedCheckHandler,
	compatTelemetryHandler *httpHandler.CompatTelemetryHandler,
	aiTelemetryHandler *httpHandler.AITelemetryHandler,
	db databasePinger,
	isReady func() bool,
) *gin.Engine {
	// Create router without default middleware
	router := gin.New()
	ConfigureClientIP(router, serverConfig.TrustedProxies)

	// Use our custom logger middleware
	router.Use(logger.GinLogger())
	router.Use(logger.GinRecovery())

	// Global middleware
	router.Use(httpHandler.CORSMiddleware(serverConfig))
	router.Use(httpHandler.SecurityMiddleware())

	// Liveness and readiness endpoints (no auth required).
	registerHealthRoutes(router, db, isReady)

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
	// Recovery delete request: 2 requests per 10 minutes per IP
	recoveryDeleteRequestLimiter := httpHandler.NewRateLimiter(300*time.Second, 2)
	// Recovery delete confirm: 6 requests per 10 minutes per IP
	recoveryDeleteConfirmLimiter := httpHandler.NewRateLimiter(100*time.Second, 6)
	stepUpRateLimiter := httpHandler.NewRateLimiter(60*time.Second, 5)

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
		authGroup.POST("/recover-delete/request",
			httpHandler.RateLimitMiddleware(recoveryDeleteRequestLimiter),
			recaptchaMiddleware,
			authHandler.RequestRecoveryDelete,
		)
		authGroup.POST("/recover-delete/confirm",
			httpHandler.RateLimitMiddleware(recoveryDeleteConfirmLimiter),
			recaptchaMiddleware,
			authHandler.ConfirmRecoveryDelete,
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

	// Read-only admin directory. Authenticated with the configured service
	// credential, not an end-user session. See PROJECT_CONTEXT.md.
	adminDirectoryLimiter := httpHandler.NewRateLimiter(500*time.Millisecond, 120)
	adminDirectory := router.Group("/api/admin/directory")
	adminDirectory.Use(httpHandler.RateLimitMiddleware(adminDirectoryLimiter))
	adminDirectory.Use(httpHandler.AdminServiceAuthMiddleware(serverConfig.AdminAPIKey))
	{
		adminDirectory.GET("/users", adminDirectoryHandler.ListUsers)
		adminDirectory.GET("/users/:id", adminDirectoryHandler.GetUser)
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

		// Platform administration has two tiers:
		//  - admin: read-only support (lists, details, activities, log tail)
		//  - system admin (admin + is_system_user, DB-checked): every change,
		//    and destructive changes also need a fresh step-up (master password + 2FA).
		systemAdmin := httpHandler.RequireSystemAdminMiddleware(userRepo)
		stepUp := httpHandler.RequireAdminStepUpMiddleware(authService)
		audit := func(activityType domain.ActivityType) gin.HandlerFunc {
			return httpHandler.AdminAuditMiddleware(adminAuditLogger, activityType)
		}

		// User management routes. Accounts are created by self-signup only;
		// an admin never sets another user's master password.
		usersGroup := apiGroup.Group("/users")
		usersGroup.Use(httpHandler.RequireAdminMiddleware())
		{
			usersGroup.GET("", userHandler.List)
			usersGroup.GET("/:id", userHandler.GetByID)
			usersGroup.PUT("/:id", systemAdmin, stepUp, userHandler.Update)
			usersGroup.DELETE("/:id", systemAdmin, stepUp, userHandler.Delete)
			usersGroup.GET("/:id/activities", activityHandler.GetUserActivities)

			// Ownership management for user deletion
			usersGroup.GET("/:id/ownership-check", userHandler.CheckOwnership)
			usersGroup.POST("/:id/transfer-ownership", systemAdmin, stepUp, audit(domain.ActivityTypeAdminOwnershipTransferred), userHandler.TransferOwnership)
			usersGroup.POST("/:id/delete-with-organizations", systemAdmin, stepUp, audit(domain.ActivityTypeAdminUserDeletedWithOrgs), userHandler.DeleteWithOrganizations)
		}

		// Organization invitations addressed to the current user
		invitationsGroup := apiGroup.Group("/invitations")
		{
			invitationsGroup.GET("", invitationHandler.ListReceived)
			invitationsGroup.POST("/:id/accept", invitationHandler.Accept)
			invitationsGroup.POST("/:id/decline", invitationHandler.Decline)
		}

		// Referral invitations ("invite a friend to Passwall")
		referralsGroup := apiGroup.Group("/referrals")
		{
			referralsGroup.GET("", invitationHandler.ListReferrals)
			referralsGroup.POST("", invitationHandler.CreateReferral)
		}

		// Activity management routes - Admin only
		adminActivitiesGroup := apiGroup.Group("/activities")
		adminActivitiesGroup.Use(httpHandler.RequireAdminMiddleware())
		{
			adminActivitiesGroup.GET("", activityHandler.ListActivities)
		}

		// Platform admin API
		adminGroup := apiGroup.Group("/admin")
		adminGroup.Use(httpHandler.RequireAdminMiddleware())
		{
			adminGroup.POST("/step-up", systemAdmin, httpHandler.RateLimitMiddleware(stepUpRateLimiter), audit(domain.ActivityTypeAdminStepUp), adminStepUpHandler.Create)

			adminGroup.GET("/organizations", adminSubscriptionsHandler.ListOrganizations)
			adminGroup.GET("/subscriptions", adminSubscriptionsHandler.List)
			adminGroup.POST("/organizations/:id/subscription/grant", systemAdmin, stepUp, adminSubscriptionsHandler.GrantManual)
			adminGroup.POST("/organizations/:id/subscription/extend", systemAdmin, stepUp, adminSubscriptionsHandler.ExtendManual)
			adminGroup.POST("/organizations/:id/subscription/revoke", systemAdmin, stepUp, adminSubscriptionsHandler.RevokeManual)

			// Mail to registered users
			adminGroup.POST("/mail", systemAdmin, stepUp, audit(domain.ActivityTypeAdminMailSent), adminMailHandler.CreateJob)
			adminGroup.GET("/mail/:jobId", systemAdmin, adminMailHandler.GetJob)

			// Server logs: tail for support; downloads contain PII. Logs are never
			// deleted through the API (the 15-day rotation is automatic).
			adminGroup.GET("/logs", adminLogsHandler.List)
			adminGroup.GET("/logs/download", systemAdmin, stepUp, audit(domain.ActivityTypeAdminLogsDownloaded), adminLogsHandler.Download)
			adminGroup.GET("/logs/download-bundle", systemAdmin, stepUp, audit(domain.ActivityTypeAdminLogsDownloaded), adminLogsHandler.DownloadBundle)

			adminGroup.GET("/telemetry/compat", compatTelemetryHandler.ListAdmin)
			adminGroup.GET("/telemetry/compat/summary", compatTelemetryHandler.ListSummaryAdmin)
			adminGroup.POST("/telemetry/compat/cleanup", systemAdmin, stepUp, audit(domain.ActivityTypeAdminTelemetryCleaned), compatTelemetryHandler.CleanupAdmin)
			adminGroup.GET("/telemetry/compat/analyze", systemAdmin, audit(domain.ActivityTypeAdminTelemetryAnalyzed), aiTelemetryHandler.Analyze)
			adminGroup.GET("/telemetry/compat/analyze/verdicts", aiTelemetryHandler.ListVerdicts)
			adminGroup.DELETE("/telemetry/compat/analyze/verdicts", systemAdmin, stepUp, audit(domain.ActivityTypeAdminTelemetryVerdictsReset), aiTelemetryHandler.ResetVerdicts)

			// Custom icons
			adminGroup.GET("/icons", iconsHandler.ListCustomIcons)
			adminGroup.POST("/icons/:domain", systemAdmin, audit(domain.ActivityTypeAdminIconChanged), iconsHandler.UploadCustomIcon)
			adminGroup.DELETE("/icons/:domain", systemAdmin, audit(domain.ActivityTypeAdminIconChanged), iconsHandler.DeleteCustomIcon)
		}

		// ============================================================
		// ORGANIZATIONS API
		// ============================================================

		// Organizations CRUD
		// OrgPublicIDResolverMiddleware resolves the short public_id in :id to a numeric org ID,
		// then FirewallMiddleware checks IP-based access for that org.
		// Organization policies are not behind the organization firewall: an
		// owner or admin who sets a wrong IP rule must still be able to fix it.
		// The handlers require owner/admin (or membership for /active).
		orgPoliciesGroup := apiGroup.Group("/organizations")
		orgPoliciesGroup.Use(httpHandler.OrgPublicIDResolverMiddleware(orgRepo))
		{
			orgPoliciesGroup.GET("/:id/policies", organizationPolicyHandler.ListPolicies)
			orgPoliciesGroup.GET("/:id/policies/active", organizationPolicyHandler.GetActivePolicies)
			orgPoliciesGroup.GET("/:id/policies/:policyType", organizationPolicyHandler.GetPolicy)
			orgPoliciesGroup.PUT("/:id/policies/:policyType", organizationPolicyHandler.UpdatePolicy)
		}

		orgsGroup := apiGroup.Group("/organizations")
		orgsGroup.Use(httpHandler.OrgPublicIDResolverMiddleware(orgRepo))
		orgsGroup.Use(httpHandler.FirewallMiddleware(firewallService))
		{
			orgsGroup.POST("", organizationHandler.Create)
			orgsGroup.GET("", organizationHandler.List)
			orgsGroup.GET("/:id", organizationHandler.GetByID)
			orgsGroup.GET("/:id/entitlements", entitlementHandler.Get)
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
			orgsGroup.GET("/:id/members", organizationHandler.GetMembers)
			orgsGroup.PUT("/:id/members/:userId", organizationHandler.UpdateMemberRole)
			orgsGroup.DELETE("/:id/members/:userId", organizationHandler.RemoveMember)
			orgsGroup.POST("/:id/members/:userId/confirm", organizationHandler.ConfirmProvisionedMember)
			orgsGroup.PUT("/:id/membership/key", organizationHandler.RewrapOwnOrgKey)

			// Invitations (single source of truth; membership is created on accept)
			orgsGroup.POST("/:id/invitations", invitationHandler.CreateOrgInvitation)
			orgsGroup.GET("/:id/invitations", invitationHandler.ListOrgInvitations)
			orgsGroup.POST("/:id/invitations/:invitationId/resend", invitationHandler.ResendOrgInvitation)
			orgsGroup.DELETE("/:id/invitations/:invitationId", invitationHandler.RevokeOrgInvitation)

			// Teams nested under organization
			orgsGroup.POST("/:id/teams", teamHandler.Create)
			orgsGroup.GET("/:id/teams", teamHandler.List)

			// Collections nested under organization
			orgsGroup.POST("/:id/collections", collectionHandler.Create)
			orgsGroup.GET("/:id/collections", collectionHandler.List)

			// Organization settings (preferences)
			orgsGroup.GET("/:id/settings", organizationSettingsHandler.ListSettings)
			orgsGroup.PUT("/:id/settings", organizationSettingsHandler.UpsertSettings)


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

		// Teams (direct access by ID)
		teamsGroup := apiGroup.Group("/teams")
		teamsGroup.Use(httpHandler.FirewallForResourceMiddleware(firewallService, service.FirewallResourceTeam))
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
		collectionsGroup.Use(httpHandler.FirewallForResourceMiddleware(firewallService, service.FirewallResourceCollection))
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
		orgItemsGroup.Use(httpHandler.FirewallForResourceMiddleware(firewallService, service.FirewallResourceItem))
		{
			orgItemsGroup.GET("/:id", organizationItemHandler.GetByID)
			orgItemsGroup.GET("/:id/autofill-secret", organizationItemHandler.AutofillSecret)
			orgItemsGroup.PUT("/:id", organizationItemHandler.Update)
			orgItemsGroup.DELETE("/:id", organizationItemHandler.Delete)
		}

		// Create organization item (under organization)
		orgsGroup.POST("/:id/items", organizationItemHandler.Create)

		v2Organizations := apiGroup.Group("/v2/organizations")
		v2Organizations.Use(httpHandler.OrgPublicIDResolverMiddleware(orgRepo))
		v2Organizations.Use(httpHandler.FirewallMiddleware(firewallService))
		v2Organizations.GET("/:id/items", organizationItemHandler.ListV2)
	}

	return router
}
