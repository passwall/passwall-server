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
)

// App represents the application
type App struct {
	config          *config.Config
	db              database.Database
	server          *http.Server
	tokenCleanup    *cleanup.TokenCleanup
	activityCleanup *cleanup.ActivityCleanup
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

	// Auto migrate
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Seed roles and permissions (using application context)
	if err := gormrepo.SeedRolesAndPermissions(ctx, db.DB()); err != nil {
		fmt.Printf("Note: roles and permissions might already exist: %v\n", err)
	} else {
		fmt.Println("✓ Roles and permissions seeded successfully")
	}

	// Seed super admin (using application context)
	if err := gormrepo.SeedSuperAdmin(ctx, db.DB(), &cfg.SuperAdmin); err != nil {
		fmt.Printf("Warning: failed to seed super admin: %v\n", err)
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
	folderRepo := gormrepo.NewFolderRepository(a.db.DB())

	// NOTE: Legacy repos removed - all item types now use ItemRepository with type field

	// Initialize logger adapter for services
	serviceLogger := logger.NewAdapter()

	// NOTE: Legacy encryption service removed - modern items use client-side encryption only

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

	// Initialize services
	authConfig := &service.AuthConfig{
		JWTSecret:            a.config.Server.Secret,
		AccessTokenDuration:  a.config.Server.AccessTokenExpireDuration,
		RefreshTokenDuration: a.config.Server.RefreshTokenExpireDuration,
	}

	// Initialize services
	userActivityService := service.NewUserActivityService(userActivityRepo, serviceLogger)
	excludedDomainService := service.NewExcludedDomainService(excludedDomainRepo, serviceLogger)
	folderService := service.NewFolderService(folderRepo, serviceLogger)
	verificationService := service.NewVerificationService(verificationRepo, userRepo, serviceLogger)
	authService := service.NewAuthService(userRepo, tokenRepo, verificationRepo, userActivityService, emailSender, authConfig, serviceLogger)
	userService := service.NewUserService(userRepo, serviceLogger)

	// Modern flexible items service (handles all item types)
	itemService := service.NewItemService(itemRepo, serviceLogger)

	// Initialize handlers
	activityHandler := httpHandler.NewActivityHandler(userActivityService)
	authHandler := httpHandler.NewAuthHandler(authService, verificationService, userActivityService, emailSender)
	userHandler := httpHandler.NewUserHandler(userService)

	// Modern handlers (all item types use ItemHandler now)
	itemHandler := httpHandler.NewItemHandler(itemService)
	excludedDomainHandler := httpHandler.NewExcludedDomainHandler(excludedDomainService)
	folderHandler := httpHandler.NewFolderHandler(folderService)

	// Setup router
	router := SetupRouter(
		&a.config.Server,
		authService,
		authHandler,
		activityHandler,
		itemHandler,
		excludedDomainHandler,
		folderHandler,
		userHandler,
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

	// Start cleanup services in background (using application context)
	go a.tokenCleanup.Start(ctx)
	go a.activityCleanup.Start(ctx)

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		logger.Infof("Server starting on %s", addr)
		fmt.Printf("🚀 Passwall Server is starting at %s in '%s' mode\n", addr, a.config.Server.Env)

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
