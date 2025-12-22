package app

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
)

var (
	mockName     = "passwall-patron"
	mockPassword = "123456789123456789"
	mockSecret   = "supersecret"
	now          = time.Now()
)

// generateTestEmail generates a unique email for each test
func generateTestEmail() string {
	return fmt.Sprintf("test-%s@passwall.io", uuid.NewV4().String())
}

func TestGenerateSchema(t *testing.T) {
	setupTestConfig()
	store, err := initDB()
	if err != nil {
		t.Errorf("Error in database initialization ! err:  %v", err)
	}
	defer cleanupTestDB(store, t)

	testEmail := generateTestEmail()

	type args struct {
		s    storage.Store
		user *model.User
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "No error ", args: args{
			s: store,
			user: &model.User{
				ID:               0,
				UUID:             uuid.NewV4(),
				Name:             mockName,
				Email:            testEmail,
				MasterPassword:   mockPassword,
				Secret:           mockSecret,
				Schema:           "users",
				Role:             "Member",
				ConfirmationCode: "12345678",
				EmailVerifiedAt:  time.Time{},
			},
		},
			wantErr: false,
		},
		// todo: populate different cases in test table
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSchema(tt.args.s, tt.args.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			want, err := store.Users().FindByEmail(testEmail)
			if err != nil {
				t.Errorf("FindByEmail() error %v", err)
			}
			got.CreatedAt = got.CreatedAt.UTC()
			got.UpdatedAt = got.UpdatedAt.UTC()
			got.EmailVerifiedAt = got.EmailVerifiedAt.UTC()
			if !reflect.DeepEqual(got, want) {
				t.Errorf("GenerateSchema() got = %v, want %v", got, want)
			}
		})
	}
	// todo: teardown resources
}

func TestCreateUser(t *testing.T) {
	setupTestConfig()
	store, err := initDB()
	if err != nil {
		t.Skipf("Skipping test - database not available: %v", err)
	}
	defer cleanupTestDB(store, t)

	testEmail := generateTestEmail()

	type args struct {
		s       storage.Store
		userDTO *model.UserDTO
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"No Error", args{
			s: store,
			userDTO: &model.UserDTO{
				ID:              0,
				UUID:            uuid.NewV4(),
				Name:            mockName,
				Email:           testEmail,
				MasterPassword:  mockPassword,
				Secret:          mockSecret,
				Schema:          "users",
				Role:            "Member",
				EmailVerifiedAt: now,
			},
		},
			false,
		},
		{"Expected Error: No name, email", args{
			s: store,
			userDTO: &model.UserDTO{
				ID:              1,
				UUID:            uuid.NewV4(),
				Name:            "",
				Email:           "",
				MasterPassword:  "",
				Secret:          "",
				Schema:          "users",
				Role:            "Member",
				EmailVerifiedAt: now,
			},
		},
			true, // Validation should fail for empty name, email, password
		},
		// todo: populate test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateUser(tt.args.s, tt.args.userDTO)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Skip validation if error is expected
			if tt.wantErr {
				return
			}

			user, err := store.Users().FindByEmail(testEmail)
			if err != nil {
				t.Errorf("FindByEmail() error = %v", err)
				return
			}
			// due to different timezone settings received data should be transformed to UTC timezone
			got.UpdatedAt = got.UpdatedAt.UTC()
			got.CreatedAt = got.CreatedAt.UTC()
			got.EmailVerifiedAt = got.EmailVerifiedAt.UTC()
			if !reflect.DeepEqual(got, user) {
				t.Errorf("CreateUser() got = %v, want %v", got, user)
			}

		})
	}
	// todo: teardown resources

}

// todo : complete TestUpdateUser, once the issues fixed above
func TestUpdateUser(t *testing.T) {
	t.Skip("TestUpdateUser is not yet implemented - empty test case causes nil pointer panic. TODO: add proper test scenarios")
}

// setupTestConfig initializes viper config for tests
func setupTestConfig() {
	// Set default values for testing
	viper.SetDefault("server.generatedPasswordLength", "16")
	viper.SetDefault("server.passphrase", "test-passphrase")
	viper.SetDefault("server.timeout", 30)
}

func initDB() (*storage.Database, error) {
	// Read from environment variables (for CI/CD) or use defaults (for local dev)
	dbHost := getEnv("PW_DB_HOST", "localhost")
	dbPort := getEnv("PW_DB_PORT", "5432")
	dbName := getEnv("PW_DB_NAME", "passwall")
	dbUsername := getEnv("PW_DB_USERNAME", "postgres")
	dbPassword := getEnv("PW_DB_PASSWORD", "postgres")
	dbSSLMode := getEnv("PW_DB_SSL_MODE", "disable")

	mockDBConfig := &config.DatabaseConfiguration{
		Name:     dbName,
		Username: dbUsername,
		Password: dbPassword,
		Host:     dbHost,
		Port:     dbPort,
		LogMode:  false,
		SSLMode:  dbSSLMode,
	}

	mockDB, err := storage.DBConn(mockDBConfig)
	if err != nil {
		return nil, err
	}

	db := storage.New(mockDB)

	// Run migrations to create necessary tables
	MigrateSystemTables(db)

	return db, nil
}

// getEnv reads an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// cleanupTestDB cleans up test data from database
func cleanupTestDB(store *storage.Database, t *testing.T) {
	// For now, we skip cleanup as tests use unique data
	// and the database is fresh for each CI run.
	// In the future, consider adding a DeleteAll method to repositories
	// or using database transactions with rollback for tests.
	t.Log("Test cleanup: skipped (database fresh for each CI run)")
}
