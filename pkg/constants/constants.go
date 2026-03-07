package constants

import "os"

const (
	ConfigFilePath = "./config.yml"
	CookieName     = "passwall_token"
)

const (
	EnvPrefix      = "PW"
	EnvDev         = "dev"
	EnvProd        = "prod"
	WorkDirEnv     = "PW_WORK_DIR"
	LogPathEnv     = "PW_LOG_PATH"
	HTTPLogPathEnv = "PW_HTTP_LOG_PATH"
)

// Pagination constants
// These values are used across the application for consistent pagination behavior
const (
	// DefaultPageSize is the default number of items per page when not specified
	DefaultPageSize = 50

	// MaxPageSize is the maximum allowed number of items per page
	// Set to a high value to allow fetching all user items in one request
	// Since all items are encrypted and belong to the authenticated user,
	// this is safe from a security perspective
	MaxPageSize = 50000
)

// BcryptCost is the bcrypt hash cost used for all password hashing in the application.
// Cost 12 provides ~4× more brute-force resistance than the default (10).
// This applies to master password hashes and Secure Send passwords.
const BcryptCost = 12

func IsDev() bool {
	return os.Getenv("APP_ENV") == "dev"
}

// DefaultPersonalVaultFolders are created as organization-level folders
// when a Personal Vault organization is provisioned.
var DefaultPersonalVaultFolders = []string{"Work", "Personal", "Family", "Social"}
