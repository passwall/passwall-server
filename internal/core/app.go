package core

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/cleanup"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	httpHandler "github.com/passwall/passwall-server/internal/handler/http"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/database"
	"github.com/passwall/passwall-server/pkg/hibp"
	"github.com/passwall/passwall-server/pkg/logger"
	stripeClient "github.com/passwall/passwall-server/pkg/stripe"
)

// App represents the application
type App struct {
	config              *config.Config
	db                  database.Database
	server              *http.Server
	tokenCleanup        *cleanup.TokenCleanup
	activityCleanup     *cleanup.ActivityCleanup
	logCleanup          *cleanup.LogCleanup
	sendCleanup         *cleanup.SendCleanup
	breachMonitorWorker *cleanup.BreachMonitorWorker
	subscriptionWorker  *cleanup.SubscriptionWorker
	draftOrgCleanup     *cleanup.DraftOrganizationCleanup
	invitationExpiry    *cleanup.InvitationExpiry
	emailSender         email.Sender
	readiness           atomic.Bool
	workerWG            sync.WaitGroup
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

	if cfg.Database.AutoMigrate {
		if err := MigrateDatabase(ctx, db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to migrate database: %w", err)
		}
	}

	if cfg.Database.AutoSeed {
		if err := SeedDatabase(ctx, db, cfg); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to seed database: %w", err)
		}
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
	if config.IsProduction(a.config.Server.Env) {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize repositories
	// Role and Permission repos (for future use)
	_ = gormrepo.NewRoleRepository(a.db.DB())
	_ = gormrepo.NewPermissionRepository(a.db.DB())

	// User and auth repos
	userRepo := gormrepo.NewUserRepository(a.db.DB())
	tokenRepo := gormrepo.NewTokenRepository(a.db.DB())
	verificationRepo := gormrepo.NewVerificationRepository(a.db.DB())
	accountDeletionTokenRepo := gormrepo.NewAccountDeletionTokenRepository(a.db.DB())
	userActivityRepo := gormrepo.NewUserActivityRepository(a.db.DB())
	excludedDomainRepo := gormrepo.NewExcludedDomainRepository(a.db.DB())
	compatTelemetryRepo := gormrepo.NewCompatTelemetryRepository(a.db.DB())
	verdictRepo := gormrepo.NewTelemetryAIVerdictRepository(a.db.DB())
	preferencesRepo := gormrepo.NewPreferencesRepository(a.db.DB())
	invitationRepo := gormrepo.NewInvitationRepository(a.db.DB())
	orgInvitationRepo := gormrepo.NewOrganizationInvitationRepository(a.db.DB())

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
	entitlementOverrideRepo := gormrepo.NewOrganizationEntitlementOverrideRepository(a.db.DB())
	// Item share repo (personal sharing)
	itemShareRepo := gormrepo.NewItemShareRepository(a.db.DB())
	// Emergency access repo
	emergencyAccessRepo := gormrepo.NewEmergencyAccessRepository(a.db.DB())
	// Send repo
	sendRepo := gormrepo.NewSendRepository(a.db.DB())

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
	webhookEventRepo := gormrepo.NewWebhookEventRepository(a.db.DB())
	txManager := gormrepo.NewTxManager(a.db.DB())
	entitlementService := service.NewOrganizationEntitlementService(
		subscriptionRepo,
		planRepo,
		orgRepo,
		orgItemRepo,
		entitlementOverrideRepo,
		a.config.Server.FrontendURL,
		service.WithEntitlementEnforcementModes(
			os.Getenv("ENTITLEMENT_ENFORCEMENT_MODES"),
			serviceLogger,
		),
	)
	personalVaultProvisioner := service.NewPersonalVaultProvisioner(
		txManager,
		userRepo,
		orgRepo,
		orgUserRepo,
		collectionRepo,
		orgFolderRepo,
		subscriptionRepo,
		planRepo,
	)

	// Organization policy repo (needed by auth service for policy requirements on sign-in)
	orgPolicyRepo := gormrepo.NewOrganizationPolicyRepository(a.db.DB())

	// Organization policy service (created early so failedLoginTracker can use it in authService)
	organizationPolicyService := service.NewOrganizationPolicyService(orgPolicyRepo, orgUserRepo, subscriptionRepo, serviceLogger, entitlementService)
	failedLoginTracker := service.NewFailedLoginTracker(organizationPolicyService)

	userService := service.NewUserService(
		userRepo,
		tokenRepo,
		orgRepo,
		orgUserRepo,
		orgItemRepo,
		teamUserRepo,
		collectionUserRepo,
		itemShareRepo,
		invitationRepo,
		userActivityRepo,
		serviceLogger,
		txManager,
		personalVaultProvisioner,
	)
	authService := service.NewAuthService(userRepo, tokenRepo, verificationRepo, accountDeletionTokenRepo, orgRepo, orgUserRepo, invitationRepo, subscriptionRepo, orgPolicyRepo, failedLoginTracker, userActivityService, userService, emailSender, emailBuilder, authConfig, serviceLogger, personalVaultProvisioner, entitlementService)
	userNotificationPreferencesService := service.NewUserNotificationPreferencesService(preferencesRepo, serviceLogger)
	userAppearancePreferencesService := service.NewUserAppearancePreferencesService(preferencesRepo, serviceLogger)
	invitationService := service.NewInvitationService(invitationRepo, userRepo, emailSender, emailBuilder, serviceLogger)

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
		subscriptionRepo,
		planRepo,
		serviceLogger,
		service.WithOrganizationEntitlements(entitlementService),
		service.WithOrganizationInvitations(service.OrgInvitationDeps{
			Invitations:  orgInvitationRepo,
			Preferences:  preferencesRepo,
			TxManager:    txManager,
			EmailSender:  emailSender,
			EmailBuilder: emailBuilder,
		}),
	)

	// Subscription service (needs organizationService, stripe client, email service optional, logger)
	subscriptionService := service.NewSubscriptionService(
		subscriptionRepo,
		planRepo,
		orgRepo,
		organizationService,
		nil,
		stripeClientInstance,
		serviceLogger,
		txManager,
		service.WithManualSubscriptionEmails(emailSender, emailBuilder, orgUserRepo, a.config.Server.FrontendURL),
	)

	// Payment service - handles org subscriptions via Stripe webhooks
	paymentService = service.NewPaymentService(
		stripeClientInstance, orgRepo, orgUserRepo, userRepo, subscriptionService, webhookEventRepo, planRepo, userActivityService, a.config, serviceLogger,
		service.WithTrialHistory(subscriptionRepo),
		service.WithTrialEndingEmails(emailSender, emailBuilder),
	)

	// RevenueCat service - handles mobile in-app purchases via webhooks (org-level subscriptions)
	revenueCatService := service.NewRevenueCatService(userRepo, orgRepo, subscriptionService, webhookEventRepo, planRepo, userActivityService, a.config, serviceLogger)

	featureService := service.NewFeatureService(entitlementService)
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
		entitlementService,
	)
	teamService := service.NewTeamService(
		teamRepo,
		teamUserRepo,
		orgUserRepo,
		orgRepo,
		serviceLogger,
		entitlementService,
	)
	itemShareService := service.NewItemShareService(
		itemShareRepo,
		orgItemRepo,
		orgUserRepo,
		collectionUserRepo,
		collectionTeamRepo,
		teamUserRepo,
		userRepo,
		emailSender,
		emailBuilder,
		serviceLogger,
		entitlementService,
	)

	// Organization items service (shared vault)
	organizationItemService := service.NewOrganizationItemService(
		orgItemRepo,
		collectionRepo,
		collectionUserRepo,
		collectionTeamRepo,
		teamUserRepo,
		orgUserRepo,
		featureService,
		serviceLogger,
	)
	organizationFolderService := service.NewOrganizationFolderService(
		orgFolderRepo,
		orgItemRepo,
		orgUserRepo,
		serviceLogger,
		entitlementService,
	)

	// Emergency access service
	emergencyAccessService := service.NewEmergencyAccessService(
		emergencyAccessRepo,
		userRepo,
		orgItemRepo,
		emailSender,
		emailBuilder,
		serviceLogger,
		entitlementService,
	)

	// Send service
	sendService := service.NewSendService(sendRepo, userRepo, orgUserRepo, orgPolicyRepo, emailSender, emailBuilder, serviceLogger, entitlementService)

	// HIBP clients
	hibpClient := hibp.NewClient(a.config.HIBP.APIKey, a.config.HIBP.RateLimitMs, a.config.HIBP.MaxRetries)
	pwnedPasswordsClient := hibp.NewPwnedPasswordsClient(0, 0) // defaults: 60 min TTL, 10k entries

	// Breach monitoring
	breachMonitorRepo := gormrepo.NewBreachMonitorRepository(a.db.DB())
	breachMonitorService := service.NewBreachMonitorService(breachMonitorRepo, hibpClient, featureService, orgUserRepo)

	// Organization policy enforcement services
	policyEnforcementService := service.NewPolicyEnforcementService(organizationPolicyService)
	policyFirewallService := service.NewPolicyFirewallService(organizationPolicyService)

	// Organization settings service (uses existing preferences repo)
	organizationSettingsService := service.NewOrganizationSettingsService(
		preferencesRepo,
		orgUserRepo,
		serviceLogger,
		entitlementService,
	)

	// SSO & SCIM repos
	ssoConnRepo := gormrepo.NewSSOConnectionRepository(a.db.DB())
	ssoStateRepo := gormrepo.NewSSOStateRepository(a.db.DB())
	scimTokenRepo := gormrepo.NewSCIMTokenRepository(a.db.DB())

	// SSO service
	serverBaseURL := a.config.Server.Domain
	ssoService := service.NewSSOService(service.SSOServiceDeps{
		ConnRepo:               ssoConnRepo,
		StateRepo:              ssoStateRepo,
		LoginCodeRepo:          gormrepo.NewSSOLoginCodeRepository(a.db.DB()),
		UserRepo:               userRepo,
		OrgUserRepo:            orgUserRepo,
		OrgRepo:                orgRepo,
		AuthService:            authService,
		Logger:                 serviceLogger,
		BaseURL:                serverBaseURL,
		RedirectOrigins:        append([]string{a.config.Server.FrontendURL}, a.config.Server.AllowedOrigins...),
		AllowLocalhostRedirect: !config.IsProduction(a.config.Server.Env),
		Entitlements:           entitlementService,
		JoinPolicies:           organizationService,
	})

	// SCIM service
	scimService := service.WithProvisioningGuards(service.NewSCIMService(
		scimTokenRepo, userRepo, orgUserRepo, teamRepo, teamUserRepo,
		serviceLogger, serverBaseURL,
		entitlementService,
	), ssoConnRepo, organizationService)

	// Initialize handlers
	activityHandler := httpHandler.NewActivityHandler(userActivityService)
	organizationActivityHandler := httpHandler.NewOrganizationActivityHandler(userActivityService, orgUserRepo, entitlementService)
	authHandler := httpHandler.NewAuthHandler(authService, verificationService, userActivityService, emailSender, emailBuilder)
	userHandler := httpHandler.NewUserHandler(userService, userActivityService, userActivityRepo)
	userNotificationPreferencesHandler := httpHandler.NewUserNotificationPreferencesHandler(userNotificationPreferencesService)
	userAppearancePreferencesHandler := httpHandler.NewUserAppearancePreferencesHandler(userAppearancePreferencesService)
	userPreferencesHandler := httpHandler.NewUserPreferencesHandler(preferencesService)
	invitationHandler := httpHandler.NewInvitationHandler(invitationService, userService, organizationService, userActivityService)

	itemShareHandler := httpHandler.NewItemShareHandler(itemShareService)
	excludedDomainHandler := httpHandler.NewExcludedDomainHandler(excludedDomainService)
	compatTelemetryHandler := httpHandler.NewCompatTelemetryHandler(compatTelemetryService)

	// AI telemetry analysis handler (optional — requires ai.enabled + ai.api_key)
	aiTelemetryAnalysisService := service.NewAITelemetryAnalysisService(&a.config.AI, compatTelemetryRepo, verdictRepo, serviceLogger)
	aiTelemetryHandler := httpHandler.NewAITelemetryHandler(aiTelemetryAnalysisService)
	// Organization handlers
	organizationHandler := httpHandler.NewOrganizationHandler(organizationService, organizationPolicyService, subscriptionRepo, userActivityService)
	entitlementHandler := httpHandler.NewEntitlementHandler(entitlementService, orgUserRepo)
	teamHandler := httpHandler.NewTeamHandler(teamService, userActivityService, organizationService)
	collectionHandler := httpHandler.NewCollectionHandler(collectionService, userActivityService, organizationService)
	organizationItemHandler := httpHandler.NewOrganizationItemHandler(organizationItemService, userActivityService, policyEnforcementService)
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
		subscriptionService,
		paymentService,
		userActivityService,
		serviceLogger,
	)
	adminMailHandler := httpHandler.NewAdminMailHandler(emailSender, userRepo, serviceLogger)
	adminLogsHandler := httpHandler.NewAdminLogsHandler()
	adminDirectoryStats := gormrepo.NewAdminDirectoryStatsRepository(a.db.DB())
	adminDirectoryHandler := httpHandler.NewAdminDirectoryHandler(userRepo, userActivityRepo, subscriptionRepo, adminDirectoryStats, serviceLogger)

	// Emergency access handler
	emergencyAccessHandler := httpHandler.NewEmergencyAccessHandler(emergencyAccessService, userRepo)

	// Send handler
	sendHandler := httpHandler.NewSendHandler(sendService)

	// Two-Factor Authentication handler
	twoFactorHandler := httpHandler.NewTwoFactorHandler(authService)

	// Organization policy & settings handlers
	organizationPolicyHandler := httpHandler.NewOrganizationPolicyHandler(organizationPolicyService)
	organizationSettingsHandler := httpHandler.NewOrganizationSettingsHandler(organizationSettingsService)

	// SSO & SCIM handlers
	ssoHandler := httpHandler.NewSSOHandler(ssoService, organizationService, service.NewActivityLogger(userActivityService), a.config.Server.FrontendURL)
	scimHandler := httpHandler.NewSCIMHandler(scimService, organizationService)

	// Breach monitor handler
	breachMonitorHandler := httpHandler.NewBreachMonitorHandler(breachMonitorService)

	// Compromised password check handler (batch HIBP Pwned Passwords)
	compromisedCheckHandler := httpHandler.NewCompromisedCheckHandler(
		pwnedPasswordsClient,
		userRepo,
		orgUserRepo,
		entitlementService,
	)

	// Icons handler (public favicon service with protection)
	iconsHandler := httpHandler.NewIconsHandler(serviceLogger)

	// Setup router
	router := SetupRouter(
		&a.config.Server,
		authService,
		policyFirewallService,
		orgRepo,
		userRepo,
		authHandler,
		twoFactorHandler,
		activityHandler,
		organizationActivityHandler,
		itemShareHandler,
		excludedDomainHandler,
		userHandler,
		userNotificationPreferencesHandler,
		userAppearancePreferencesHandler,
		userPreferencesHandler,
		invitationHandler,
		organizationHandler,
		entitlementHandler,
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
		httpHandler.NewAdminStepUpHandler(authService, userActivityService),
		service.NewActivityLogger(userActivityService),
		adminDirectoryHandler,
		iconsHandler,
		ssoHandler,
		scimHandler,
		scimService,
		breachMonitorHandler,
		compromisedCheckHandler,
		compatTelemetryHandler,
		aiTelemetryHandler,
		a.db,
		a.readiness.Load,
	)

	// Create server
	addr := fmt.Sprintf("%s:%s", a.config.Server.Host, a.config.Server.Port)
	a.server = &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadTimeout:       time.Second * time.Duration(a.config.Server.Timeout),
		ReadHeaderTimeout: time.Second * time.Duration(a.config.Server.ReadHeaderTimeout),
		WriteTimeout:      time.Second * time.Duration(a.config.Server.Timeout),
		IdleTimeout:       time.Second * time.Duration(a.config.Server.IdleTimeout),
	}

	// Initialize token cleanup service (runs every hour)
	a.tokenCleanup = cleanup.NewTokenCleanup(tokenRepo, 1*time.Hour)

	// Initialize activity cleanup service (runs every 24 hours, keeps UserActivityRetention).
	a.activityCleanup = cleanup.NewActivityCleanup(userActivityService, 24*time.Hour, domain.UserActivityRetention)

	// Initialize log cleanup service (runs every 15 days, truncates log files in place)
	a.logCleanup = cleanup.NewLogCleanup(adminLogsHandler.LogPaths(), 15*24*time.Hour)

	// Initialize send cleanup service (runs every 6 hours)
	a.sendCleanup = cleanup.NewSendCleanup(sendRepo, 6*time.Hour)

	// Initialize breach monitor worker (runs at configured interval, default 24h)
	breachCheckInterval := time.Duration(a.config.HIBP.CheckIntervalHours) * time.Hour
	if breachCheckInterval < 1*time.Hour {
		breachCheckInterval = 24 * time.Hour
	}
	a.breachMonitorWorker = cleanup.NewBreachMonitorWorker(
		breachMonitorRepo,
		breachMonitorService,
		featureService,
		breachCheckInterval,
	)

	// Initialize subscription expiry worker (runs every 6 hours)
	a.subscriptionWorker = cleanup.NewSubscriptionWorker(subscriptionService, serviceLogger, 6*time.Hour)

	// Remove plan-first organizations whose checkout was never completed (daily, after 7 days)
	a.draftOrgCleanup = cleanup.NewDraftOrganizationCleanup(subscriptionRepo, orgRepo, serviceLogger, 7*24*time.Hour, 24*time.Hour)
	a.invitationExpiry = cleanup.NewInvitationExpiry(organizationService, serviceLogger, time.Hour)

	runCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()

	a.startWorker(func() { a.tokenCleanup.Start(runCtx) })
	a.startWorker(func() { a.activityCleanup.Start(runCtx) })
	a.startWorker(func() { a.logCleanup.Start(runCtx) })
	a.startWorker(func() { a.sendCleanup.Start(runCtx) })
	a.startWorker(func() { a.breachMonitorWorker.Start(runCtx) })
	a.startWorker(func() { a.subscriptionWorker.Run(runCtx) })
	a.startWorker(func() { a.draftOrgCleanup.Run(runCtx) })
	a.startWorker(func() { a.invitationExpiry.Run(runCtx) })

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		cancelWorkers()
		_ = a.gracefulShutdown()
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	serverErrChan := make(chan error, 1)
	a.readiness.Store(true)
	go func() {
		logger.Infof("🚀 Passwall Server is starting at %s in '%s' mode", addr, a.config.Server.Env)

		if err := a.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Server failed: %v", err)
			serverErrChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		cancelWorkers()
		return a.gracefulShutdown()
	case err := <-serverErrChan:
		cancelWorkers()
		if shutdownErr := a.gracefulShutdown(); shutdownErr != nil {
			logger.Errorf("Shutdown after server failure also failed: %v", shutdownErr)
		}
		return fmt.Errorf("server error: %w", err)
	}
}

func (a *App) startWorker(worker func()) {
	a.workerWG.Add(1)
	go func() {
		defer a.workerWG.Done()
		worker()
	}()
}

// gracefulShutdown performs graceful shutdown of all app components
func (a *App) gracefulShutdown() error {
	logger.Infof("Initiating graceful shutdown...")
	a.readiness.Store(false)
	var shutdownErr error

	httpShutdownCtx, cancelHTTPShutdown := context.WithTimeout(
		context.Background(),
		time.Duration(a.config.Server.ShutdownTimeout)*time.Second,
	)

	// Shutdown HTTP server (stops accepting new connections, waits for existing)
	logger.Infof("Shutting down HTTP server...")
	if err := a.server.Shutdown(httpShutdownCtx); err != nil {
		logger.Errorf("HTTP server forced to shutdown: %v", err)
		_ = a.server.Close()
		shutdownErr = fmt.Errorf("HTTP server shutdown: %w", err)
	}
	cancelHTTPShutdown()
	logger.Infof("HTTP server stopped gracefully")

	workersDone := make(chan struct{})
	go func() {
		a.workerWG.Wait()
		close(workersDone)
	}()
	workerTimer := time.NewTimer(time.Duration(a.config.Server.ShutdownTimeout) * time.Second)
	defer workerTimer.Stop()
	select {
	case <-workersDone:
		logger.Infof("Background workers stopped")
	case <-workerTimer.C:
		logger.Warnf("Timed out waiting for background workers to stop")
		if shutdownErr == nil {
			shutdownErr = fmt.Errorf("background worker shutdown timed out")
		}
		return shutdownErr
	}

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
	return shutdownErr
}
