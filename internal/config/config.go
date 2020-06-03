package config

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"

	"github.com/spf13/viper"
)

var (
	configuration *Configuration
)

// Configuration ...
type Configuration struct {
	Server   ServerConfiguration
	Database DatabaseConfiguration
}

// ServerConfiguration is the required paramters to set up a server
type ServerConfiguration struct {
	Port       string `default:"3625"`
	Username   string `default:"passwall"`
	Password   string `default:"password"`
	Passphrase string `default:"passphrase-for-encrypting-passwords-do-not-forget"`
	Secret     string `default:"secret-key-for-JWT-TOKEN"`
	Timeout    int    `default:"24"`
}

// DatabaseConfiguration is the required paramters to set up a DB instance
type DatabaseConfiguration struct {
	Name     string `default:"passwall"`
	Username string `default:"user"`
	Password string `default:"password"`
	Host     string `default:"localhost"`
	Port     string `default:"5432"`
	LogMode  bool   `default:"false"`
}

// SetupConfigDefaults ...
func SetupConfigDefaults() *Configuration {

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./store")

	// Bind environment variables
	bindEnvs()

	// Set default values
	setDefaults()

	// Auto generate config.yml file if it doesn't exist
	if !fileExists("./store/config.yml") {
		viper.Set("server.passphrase", generateSecureKey())
		viper.Set("server.secret", generateSecureKey())
		viper.WriteConfigAs("./store/config.yml")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Println(err)
	}
	viper.AutomaticEnv()

	err := viper.Unmarshal(&configuration)
	if err != nil {
		log.Println(err)
	}

	return configuration
}

func bindEnvs() {
	viper.BindEnv("server.port", "PORT")
	viper.BindEnv("server.username", "PW_SERVER_USERNAME")
	viper.BindEnv("server.password", "PW_SERVER_PASSWORD")
	viper.BindEnv("server.passphrase", "PW_SERVER_PASSPHRASE")
	viper.BindEnv("server.secret", "PW_SERVER_SECRET")
	viper.BindEnv("server.timeout", "PW_SERVER_TIMEOUT")

	viper.BindEnv("server.generatedPasswordLength", "PW_SERVER_GENERATED_PASSWORD_LENGTH")
	viper.BindEnv("server.accessTokenExpireDuration", "PW_SERVER_ACCESS_TOKEN_EXPIRE_DURATION")
	viper.BindEnv("server.refreshTokenExpireDuration", "PW_SERVER_REFRESH_TOKEN_EXPIRE_DURATION")

	viper.BindEnv("database.name", "PW_DB_NAME")
	viper.BindEnv("database.username", "PW_DB_USERNAME")
	viper.BindEnv("database.password", "PW_DB_PASSWORD")
	viper.BindEnv("database.host", "PW_DB_HOST")
	viper.BindEnv("database.port", "PW_DB_PORT")
	viper.BindEnv("database.logmode", "PW_DB_LOG_MODE")

	viper.BindEnv("backup.folder", "PW_BACKUP_FOLDER")
	viper.BindEnv("backup.rotation", "PW_BACKUP_ROTATION")
	viper.BindEnv("backup.period", "PW_BACKUP_PERIOD")
}

func setDefaults() {

	// Server defaults
	viper.SetDefault("server.port", "3625")
	viper.SetDefault("server.username", "passwall")
	viper.SetDefault("server.password", "password")
	viper.SetDefault("server.passphrase", "passphrase-for-encrypting-passwords-do-not-forget")
	viper.SetDefault("server.secret", "secret-key-for-JWT-TOKEN")
	viper.SetDefault("server.timeout", 24)
	viper.SetDefault("server.generatedPasswordLength", 16)
	viper.SetDefault("server.accessTokenExpireDuration", "30m")
	viper.SetDefault("server.refreshTokenExpireDuration", "15d")

	// Database defaults
	viper.SetDefault("database.name", "passwall")
	viper.SetDefault("database.username", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.logmode", false)

	// Backup defaults
	viper.SetDefault("backup.folder", "./store/")
	viper.SetDefault("backup.rotation", 7)
	viper.SetDefault("backup.period", "24h")
}

func generateSecureKey() string {
	key := make([]byte, 64)
	_, err := rand.Read(key)
	if err != nil {
		// handle error here
	}
	keyEnc := base64.StdEncoding.EncodeToString(key)
	return keyEnc
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
