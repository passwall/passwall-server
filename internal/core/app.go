package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	cleanupCtx   context.Context
	cleanupStop  context.CancelFunc
}

// New creates a new application instance
func New() (*App, error) {
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

	return &App{
		config: cfg,
		db:     db,
	}, nil
}

// Run starts the application
func (a *App) Run() error {
	// Configure Gin to use our logger
	gin.DefaultWriter = logger.GetWriter()
	gin.DefaultErrorWriter = logger.GetWriter()

	// Set Gin mode
	if a.config.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize repositories
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
	a.cleanupCtx, a.cleanupStop = context.WithCancel(context.Background())

	// Start token cleanup in background
	go a.tokenCleanup.Start(a.cleanupCtx)

	// Start server in a goroutine
	go func() {
		logger.Infof("Server starting on %s", addr)
		fmt.Printf("🚀 Passwall Server is starting at %s in '%s' mode\n", addr, a.config.Server.Env)

		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	return a.waitForShutdown()
}

// waitForShutdown waits for interrupt signal and performs graceful shutdown
func (a *App) waitForShutdown() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Infof("Shutting down server...")
	fmt.Println("\n⏳ Shutting down gracefully...")

	// Stop token cleanup
	if a.cleanupStop != nil {
		a.cleanupStop()
		logger.Infof("Token cleanup stopped")
	}

	// Give outstanding requests 5 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	// Close database connection
	if err := a.db.Close(); err != nil {
		logger.Errorf("Failed to close database: %v", err)
	}

	logger.Infof("Server exited successfully")
	fmt.Println("✅ Server stopped")
	return nil
}
