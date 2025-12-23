package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/database"
	"github.com/passwall/passwall-server/pkg/database/postgres"
	"github.com/passwall/passwall-server/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load(config.LoaderOptions{
		ConfigFile: constants.ConfigFilePath,
		EnvPrefix:  constants.EnvPrefix,
	})
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	dbCfg := &database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Username: cfg.Database.Username,
		Password: cfg.Database.Password,
		Database: cfg.Database.Name,
		SSLMode:  cfg.Database.SSLMode,
		LogMode:  cfg.Database.LogMode,
	}

	db, err := postgres.New(dbCfg)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := gormrepo.NewUserRepository(db.DB())
	tokenRepo := gormrepo.NewTokenRepository(db.DB())

	// CLI interface
	c := color.New(color.FgCyan)
	reader := bufio.NewReader(os.Stdin)

	c.Print("Enter Name Surname: ")
	name, _ := reader.ReadString('\n')

	c.Print("Enter E-mail Address: ")
	email, _ := reader.ReadString('\n')

	c.Print("Enter Master Password: ")
	password, _ := reader.ReadString('\n')

	name = clearInput(name)
	email = clearInput(email)
	password = clearInput(password)

	if name == "" || email == "" || password == "" {
		logger.Fatalf("All fields are required.")
	}

	// Create signup request
	req := &domain.SignUpRequest{
		Name:           name,
		Email:          email,
		MasterPassword: password,
	}

	// Create auth service for proper user creation with hashing and schema
	authConfig := &service.AuthConfig{
		JWTSecret:            cfg.Server.Secret,
		AccessTokenDuration:  cfg.Server.AccessTokenExpireDuration,
		RefreshTokenDuration: cfg.Server.RefreshTokenExpireDuration,
	}
	authService := service.NewAuthService(userRepo, tokenRepo, authConfig)

	user, err := authService.SignUp(context.Background(), req)
	if err != nil {
		logger.Fatalf("Failed to create user: %v", err)
	}

	color.Green("User created successfully!")
	fmt.Printf("User ID: %d\n", user.ID)
	fmt.Printf("Email: %s\n", user.Email)
	fmt.Printf("Schema: %s\n", user.Schema)
}

func clearInput(input string) string {
	return strings.TrimSpace(input)
}
