package core

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/cleanup"
	"github.com/passwall/passwall-server/internal/config"
	httpHandler "github.com/passwall/passwall-server/internal/handler/http"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/database"
	"github.com/passwall/passwall-server/pkg/logger"
)

// App represents the application
type App struct {
	config       *config.Config
	db           database.Database
	server       *http.Server
	tokenCleanup *cleanup.TokenCleanup
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

	loginRepo := gormrepo.NewLoginRepository(a.db.DB())
	bankAccountRepo := gormrepo.NewBankAccountRepository(a.db.DB())
	creditCardRepo := gormrepo.NewCreditCardRepository(a.db.DB())
	noteRepo := gormrepo.NewNoteRepository(a.db.DB())
	emailRepo := gormrepo.NewEmailRepository(a.db.DB())
	serverRepo := gormrepo.NewServerRepository(a.db.DB())
	userRepo := gormrepo.NewUserRepository(a.db.DB())
	tokenRepo := gormrepo.NewTokenRepository(a.db.DB())

	// Initialize logger adapter for services
	serviceLogger := logger.NewAdapter()

	// Initialize encryption service
	encryptor := service.NewCryptoService(a.config.Server.Passphrase)

	// Initialize services
	authConfig := &service.AuthConfig{
		JWTSecret:            a.config.Server.Secret,
		AccessTokenDuration:  a.config.Server.AccessTokenExpireDuration,
		RefreshTokenDuration: a.config.Server.RefreshTokenExpireDuration,
	}

	authService := service.NewAuthService(userRepo, tokenRepo, authConfig)
	loginService := service.NewLoginService(loginRepo, encryptor, serviceLogger)
	bankAccountService := service.NewBankAccountService(bankAccountRepo, encryptor, serviceLogger)
	creditCardService := service.NewCreditCardService(creditCardRepo, encryptor, serviceLogger)
	noteService := service.NewNoteService(noteRepo, encryptor, serviceLogger)
	emailService := service.NewEmailService(emailRepo, encryptor, serviceLogger)
	serverService := service.NewServerService(serverRepo, encryptor, serviceLogger)
	userService := service.NewUserService(userRepo, serviceLogger)

	// Initialize handlers
	authHandler := httpHandler.NewAuthHandler(authService)
	loginHandler := httpHandler.NewLoginHandler(loginService)
	bankAccountHandler := httpHandler.NewBankAccountHandler(bankAccountService)
	creditCardHandler := httpHandler.NewCreditCardHandler(creditCardService)
	noteHandler := httpHandler.NewNoteHandler(noteService)
	emailHandler := httpHandler.NewEmailHandler(emailService)
	serverHandler := httpHandler.NewServerHandler(serverService)
	userHandler := httpHandler.NewUserHandler(userService)

	// Setup router
	router := SetupRouter(
		authService,
		authHandler,
		loginHandler,
		bankAccountHandler,
		creditCardHandler,
		noteHandler,
		emailHandler,
		serverHandler,
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

	// Start token cleanup in background (using application context)
	go a.tokenCleanup.Start(ctx)

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

	// Token cleanup already stopped via context cancellation
	logger.Infof("Token cleanup stopped")

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
