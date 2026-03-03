package core

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/cleanup"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/email"
	httpHandler "github.com/passwall/passwall-server/internal/handler/http"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/database"
	"github.com/passwall/passwall-server/pkg/logger"
	stripeClient "github.com/passwall/passwall-server/pkg/stripe"
)

// App represents the application
type App struct {
	config          *config.Config
	db              database.Database
	server          *http.Server
	tokenCleanup    *cleanup.TokenCleanup
	activityCleanup *cleanup.ActivityCleanup
	logCleanup      *cleanup.LogCleanup
	sendCleanup     *cleanup.SendCleanup
	emailSender     email.Sender
}

// New creates a new application instance with the given context
func New(ctx context.Context) (*App, error) {
	// Load configuration
	cfg, err := config.Load(config.LoaderOptions{
		ConfigFile: constants.ConfigFilePath,
		EnvPrefix:  constants.EnvPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	db, err := InitDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Auto migrate - creates all tables with their final structure
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Seed database - idempotent, safe to run multiple times
	if err := SeedDatabase(ctx, db, cfg); err != nil {
		return nil, fmt.Errorf("failed to seed database: %w", err)
	}

	return &App{
		config: cfg,
		db:     db,
	}, nil
}

// Run starts the application with the given context
func (a *App) Run(ctx context.Context) error {
	// Configure Gin to use our logger
	gin.DefaultWriter = logger.GetWriter()
	gin.DefaultErrorWriter = logger.GetWriter()

	// Set Gin mode
	if a.config.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize repositories
	// Role and Permission repos (for future use)
	_ = gormrepo.NewRoleRepository(a.db.DB())
	_ = gormrepo.NewPermissionRepository(a.db.DB())

	// Modern flexible items repository
	itemRepo := gormrepo.NewItemRepository(a.db.DB())

	// User and auth repos
	userRepo := gormrepo.NewUserRepository(a.db.DB())
	tokenRepo := gormrepo.NewTokenRepository(a.db.DB())
	verificationRepo := gormrepo.NewVerificationRepository(a.db.DB())
	userActivityRepo := gormrepo.NewUserActivityRepository(a.db.DB())
	excludedDomainRepo := gormrepo.NewExcludedDomainRepository(a.db.DB())
	compatTelemetryRepo := gormrepo.NewCompatTelemetryRepository(a.db.DB())
	preferencesRepo := gormrepo.NewPreferencesRepository(a.db.DB())
	invitationRepo := gormrepo.NewInvitationRepository(a.db.DB())

	// Organization repos
	orgRepo := gormrepo.NewOrganizationRepository(a.db.DB())
	orgUserRepo := gormrepo.NewOrganizationUserRepository(a.db.DB())
	teamRepo := gormrepo.NewTeamRepository(a.db.DB())
	teamUserRepo := gormrepo.NewTeamUserRepository(a.db.DB())
	collectionRepo := gormrepo.NewCollectionRepository(a.db.DB())
	collectionUserRepo := gormrepo.NewCollectionUserRepository(a.db.DB())
	collectionTeamRepo := gormrepo.NewCollectionTeamRepository(a.db.DB())
	orgItemRepo := gormrepo.NewOrganizationItemRepository(a.db.DB())
	orgFolderRepo := gormrepo.NewOrganizationFolderRepository(a.db.DB())
	// Item share repo (personal sharing)
	itemShareRepo := gormrepo.NewItemShareRepository(a.db.DB())
	// Emergency access repo
	emergencyAccessRepo := gormrepo.NewEmergencyAccessRepository(a.db.DB())
	// Send repo
	sendRepo := gormrepo.NewSendRepository(a.db.DB())

	// NOTE: Legacy repos removed - all item types now use ItemRepository with type field

	// Initialize logger adapter for services
	serviceLogger := logger.NewAdapter()

	// Initialize email sender
	emailSender, err := email.NewSender(email.Config{
		EmailConfig: &a.config.Email,
		FrontendURL: a.config.Server.FrontendURL,
		Logger:      serviceLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize email sender: %w", err)
	}

	// Store email sender for cleanup
	a.emailSender = emailSender

	// Initialize email builder for preparing email messages
	emailBuilder, err := email.NewEmailBuilder(
		a.config.Server.FrontendURL,
		a.config.Email.FromEmail,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize email builder: %w", err)
	}

	// Initialize services
	authConfig := &service.AuthConfig{
		JWTSecret:            a.config.Server.Secret,
		AccessTokenDuration:  a.config.Server.AccessTokenExpireDuration,
		RefreshTokenDuration: a.config.Server.RefreshTokenExpireDuration,
	}

	// Initialize services
	userActivityService := service.NewUserActivityService(userActivityRepo, serviceLogger)
	excludedDomainService := service.NewExcludedDomainService(excludedDomainRepo, serviceLogger)
	compatTelemetryService := service.NewCompatTelemetryService(compatTelemetryRepo, serviceLogger)
	preferencesService := service.NewPreferencesService(preferencesRepo, serviceLogger)
	verificationService := service.NewVerificationService(verificationRepo, userRepo, serviceLogger)

	// Initialize subscription repos before auth service (used for plan-based device limits)
	subscriptionRepo := gormrepo.NewSubscriptionRepository(a.db.DB())
	planRepo := gormrepo.NewPlanRepository(a.db.DB())

	// Organization policy repo (needed by auth service for policy requirements on sign-in)
	orgPolicyRepo := gormrepo.NewOrganizationPolicyRepository(a.db.DB())

	// Organization policy service (created early so failedLoginTracker can use it in authService)
	organizationPolicyService := service.NewOrganizationPolicyService(orgPolicyRepo, orgUserRepo, subscriptionRepo, serviceLogger)
	failedLoginTracker := service.NewFailedLoginTracker(organizationPolicyService)

	authService := service.NewAuthService(userRepo, tokenRepo, verificationRepo, orgRepo, orgUserRepo, orgFolderRepo, invitationRepo, subscriptionRepo, orgPolicyRepo, failedLoginTracker, userActivityService, emailSender, emailBuilder, authConfig, serviceLogger)
	userService := service.NewUserService(
		userRepo,
		orgRepo,
		orgUserRepo,
		orgFolderRepo,
		teamUserRepo,
		collectionUserRepo,
		itemShareRepo,
		invitationRepo,
		userActivityRepo,
		serviceLogger,
	)
	userNotificationPreferencesService := service.NewUserNotificationPreferencesService(preferencesRepo, serviceLogger)
	userAppearancePreferencesService := service.NewUserAppearancePreferencesService(preferencesRepo, serviceLogger)
	invitationService := service.NewInvitationService(invitationRepo, userRepo, orgRepo, emailSender, emailBuilder, serviceLogger)

	// Modern flexible items service (handles all item types)
	itemService := service.NewItemService(itemRepo, serviceLogger)
	itemShareService := service.NewItemShareService(
		itemShareRepo,
		orgItemRepo,
		userRepo,
		emailSender,
		emailBuilder,
		serviceLogger,
	)

	// Initialize Stripe client
	stripeClientInstance := stripeClient.NewClient(a.config.Stripe.SecretKey, a.config.Stripe.WebhookSecret)

	// Create a placeholder payment service for organizationService (will be updated later)
	var paymentService service.PaymentService

	// Organization service
	organizationService := service.NewOrganizationService(
		orgRepo,
		orgUserRepo,
		userRepo,
		teamRepo,
		teamUserRepo,
		collectionRepo,
		collectionUserRepo,
		collectionTeamRepo,
		orgPolicyRepo,
		paymentService,
		invitationService,
		subscriptionRepo,
		planRepo,
		serviceLogger,
	)

	// Subscription service (needs organizationService, stripe client, email service optional, logger)
	subscriptionService := service.NewSubscriptionService(subscriptionRepo, planRepo, organizationService, nil, stripeClientInstance, serviceLogger)

	// Payment service - handles org subscriptions via Stripe webhooks
	paymentService = service.NewPaymentService(stripeClientInstance, orgRepo, orgUserRepo, userRepo, subscriptionService, planRepo, userActivityService, a.config, serviceLogger)

	// RevenueCat service - handles mobile in-app purchases via webhooks (org-level subscriptions)
	revenueCatService := service.NewRevenueCatService(userRepo, orgRepo, subscriptionService, planRepo, userActivityService, a.config, serviceLogger)

	teamService := service.NewTeamService(teamRepo, teamUserRepo, orgUserRepo, orgRepo, serviceLogger)
	collectionService := service.NewCollectionService(
		collectionRepo,
		collectionUserRepo,
		collectionTeamRepo,
		orgUserRepo,
		teamRepo,
		teamUserRepo,
		orgRepo,
		orgItemRepo,
		subscriptionRepo,
		serviceLogger,
	)

	// Organization items service (shared vault)
	organizationItemService := service.NewOrganizationItemService(
		orgItemRepo,
		collectionRepo,
		collectionUserRepo,
		collectionTeamRepo,
		teamUserRepo,
		orgUserRepo,
		serviceLogger,
	)
	organizationFolderService := service.NewOrganizationFolderService(orgFolderRepo, orgItemRepo, orgUserRepo, serviceLogger)

	// Emergency access service
	emergencyAccessService := service.NewEmergencyAccessService(
		emergencyAccessRepo,
		userRepo,
		orgItemRepo,
		emailSender,
		emailBuilder,
		serviceLogger,
	)

	// Send service
	sendService := service.NewSendService(sendRepo, userRepo, orgUserRepo, orgPolicyRepo, serviceLogger)

	// Organization policy enforcement services
	policyEnforcementService := service.NewPolicyEnforcementService(organizationPolicyService)
	_ = policyEnforcementService // Available for injection into other services
	policyFirewallService := service.NewPolicyFirewallService(organizationPolicyService)

	// Organization settings service (uses existing preferences repo)
	organizationSettingsService := service.NewOrganizationSettingsService(preferencesRepo, orgUserRepo, serviceLogger)

	// SSO & SCIM repos
	ssoConnRepo := gormrepo.NewSSOConnectionRepository(a.db.DB())
	ssoStateRepo := gormrepo.NewSSOStateRepository(a.db.DB())
	scimTokenRepo := gormrepo.NewSCIMTokenRepository(a.db.DB())

	// Key Escrow repos and service
	keyEscrowRepo := gormrepo.NewKeyEscrowRepository(a.db.DB())
	orgEscrowKeyRepo := gormrepo.NewOrgEscrowKeyRepository(a.db.DB())
	keyEscrowService := service.NewKeyEscrowService(
		keyEscrowRepo, orgEscrowKeyRepo, ssoConnRepo,
		a.config.Server.EscrowMasterKey, serviceLogger,
	)

	// SSO service
	serverBaseURL := a.config.Server.Domain
	ssoService := service.NewSSOService(
		ssoConnRepo, ssoStateRepo, userRepo, orgUserRepo, orgRepo,
		authService, keyEscrowService, serviceLogger, serverBaseURL,
	)

	// SCIM service
	scimService := service.NewSCIMService(
		scimTokenRepo, userRepo, orgUserRepo, teamRepo, teamUserRepo,
		serviceLogger, serverBaseURL,
	)

	// Initialize handlers
	activityHandler := httpHandler.NewActivityHandler(userActivityService)
	organizationActivityHandler := httpHandler.NewOrganizationActivityHandler(userActivityService, orgUserRepo)
	authHandler := httpHandler.NewAuthHandler(authService, verificationService, userActivityService, emailSender, emailBuilder)
	userHandler := httpHandler.NewUserHandler(userService, userActivityService)
	userNotificationPreferencesHandler := httpHandler.NewUserNotificationPreferencesHandler(userNotificationPreferencesService)
	userAppearancePreferencesHandler := httpHandler.NewUserAppearancePreferencesHandler(userAppearancePreferencesService)
	userPreferencesHandler := httpHandler.NewUserPreferencesHandler(preferencesService)
	invitationHandler := httpHandler.NewInvitationHandler(invitationService, userService, organizationService, userActivityService)

	// Modern handlers (all item types use ItemHandler now)
	itemHandler := httpHandler.NewItemHandler(itemService)
	itemShareHandler := httpHandler.NewItemShareHandler(itemShareService)
	excludedDomainHandler := httpHandler.NewExcludedDomainHandler(excludedDomainService)
	compatTelemetryHandler := httpHandler.NewCompatTelemetryHandler(compatTelemetryService)
	// Organization handlers
	organizationHandler := httpHandler.NewOrganizationHandler(organizationService, organizationPolicyService, subscriptionRepo, userActivityService)
	teamHandler := httpHandler.NewTeamHandler(teamService, userActivityService, organizationService)
	collectionHandler := httpHandler.NewCollectionHandler(collectionService, userActivityService, organizationService)
	organizationItemHandler := httpHandler.NewOrganizationItemHandler(organizationItemService, userActivityService)
	organizationFolderHandler := httpHandler.NewOrganizationFolderHandler(organizationFolderService)

	// Payment handlers
	paymentHandler := httpHandler.NewPaymentHandler(paymentService, subscriptionService, orgRepo, orgUserRepo)
	webhookHandler := httpHandler.NewWebhookHandler(paymentService, revenueCatService)

	// Support handler
	supportHandler := httpHandler.NewSupportHandler(emailSender, serviceLogger)

	// Plans + Admin subscription management handlers
	plansHandler := httpHandler.NewPlansHandler(planRepo, serviceLogger)
	adminSubscriptionsHandler := httpHandler.NewAdminSubscriptionsHandler(
		orgRepo,
		orgUserRepo,
		subscriptionRepo,
		planRepo,
		paymentService,
		userActivityService,
		serviceLogger,
	)
	adminMailHandler := httpHandler.NewAdminMailHandler(emailSender, userRepo, serviceLogger)
	adminLogsHandler := httpHandler.NewAdminLogsHandler()

	// Emergency access handler
	emergencyAccessHandler := httpHandler.NewEmergencyAccessHandler(emergencyAccessService, userRepo)

	// Send handler
	sendHandler := httpHandler.NewSendHandler(sendService)

	// Organization policy & settings handlers
	organizationPolicyHandler := httpHandler.NewOrganizationPolicyHandler(organizationPolicyService)
	organizationSettingsHandler := httpHandler.NewOrganizationSettingsHandler(organizationSettingsService)

	// SSO, SCIM & Key Escrow handlers
	ssoHandler := httpHandler.NewSSOHandler(ssoService, organizationService)
	scimHandler := httpHandler.NewSCIMHandler(scimService, organizationService)
	keyEscrowHandler := httpHandler.NewKeyEscrowHandler(keyEscrowService, organizationService)

	// Icons handler (public favicon service with protection)
	iconsHandler := httpHandler.NewIconsHandler(serviceLogger)

	// Setup router
	router := SetupRouter(
		&a.config.Server,
		authService,
		policyFirewallService,
		authHandler,
		activityHandler,
		organizationActivityHandler,
		itemHandler,
		itemShareHandler,
		excludedDomainHandler,
		userHandler,
		userNotificationPreferencesHandler,
		userAppearancePreferencesHandler,
		userPreferencesHandler,
		invitationHandler,
		organizationHandler,
		organizationPolicyHandler,
		organizationSettingsHandler,
		teamHandler,
		collectionHandler,
		organizationItemHandler,
		organizationFolderHandler,
		emergencyAccessHandler,
		sendHandler,
		paymentHandler,
		webhookHandler,
		supportHandler,
		plansHandler,
		adminSubscriptionsHandler,
		adminMailHandler,
		adminLogsHandler,
		iconsHandler,
		ssoHandler,
		scimHandler,
		scimService,
		keyEscrowHandler,
		compatTelemetryHandler,
	)

	// Create server
	addr := fmt.Sprintf("%s:%s", a.config.Server.Host, a.config.Server.Port)
	a.server = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Second * time.Duration(a.config.Server.Timeout),
		WriteTimeout: time.Second * time.Duration(a.config.Server.Timeout),
		IdleTimeout:  60 * time.Second,
	}

	// Initialize token cleanup service (runs every hour)
	a.tokenCleanup = cleanup.NewTokenCleanup(tokenRepo, 1*time.Hour)

	// Initialize activity cleanup service (runs every 24 hours, keeps 90 days)
	a.activityCleanup = cleanup.NewActivityCleanup(userActivityService, 24*time.Hour, 90*24*time.Hour)

	// Initialize log cleanup service (runs every 15 days, truncates log files in place)
	a.logCleanup = cleanup.NewLogCleanup(adminLogsHandler.LogPaths(), 15*24*time.Hour)

	// Initialize send cleanup service (runs every 6 hours)
	a.sendCleanup = cleanup.NewSendCleanup(sendRepo, 6*time.Hour)

	// Start cleanup services in background (using application context)
	go a.tokenCleanup.Start(ctx)
	go a.activityCleanup.Start(ctx)
	go a.logCleanup.Start(ctx)
	go a.sendCleanup.Start(ctx)

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		logger.Infof("🚀 Passwall Server is starting at %s in '%s' mode", addr, a.config.Server.Env)

		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Server failed: %v", err)
			serverErrChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		// Context canceled (signal received)
		return a.gracefulShutdown()
	case err := <-serverErrChan:
		// Server failed to start
		return fmt.Errorf("server error: %w", err)
	}
}

// gracefulShutdown performs graceful shutdown of all app components
func (a *App) gracefulShutdown() error {
	logger.Infof("Initiating graceful shutdown...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server (stops accepting new connections, waits for existing)
	logger.Infof("Shutting down HTTP server...")
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("HTTP server forced to shutdown: %v", err)
		return fmt.Errorf("server forced to shutdown: %w", err)
	}
	logger.Infof("HTTP server stopped gracefully")

	// Cleanup services already stopped via context cancellation
	logger.Infof("Token cleanup stopped")
	logger.Infof("Activity cleanup stopped")
	logger.Infof("Log cleanup stopped")

	// Close email sender
	logger.Infof("Closing email sender...")
	if err := a.emailSender.Close(); err != nil {
		logger.Errorf("Failed to close email sender: %v", err)
		// Don't return error, continue shutdown
	} else {
		logger.Infof("Email sender closed")
	}

	// Close database connection
	logger.Infof("Closing database connection...")
	if err := a.db.Close(); err != nil {
		logger.Errorf("Failed to close database: %v", err)
		// Don't return error, continue shutdown
	} else {
		logger.Infof("Database connection closed")
	}

	logger.Infof("Graceful shutdown completed")
	return nil
}
