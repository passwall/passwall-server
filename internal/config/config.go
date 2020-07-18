package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

var (
	configuration  *Configuration
	configFileName = "config"
	configFileExt  = ".yml"
	configType     = "yaml"
	appName        = "passwall-server"

	configurationDirectory = filepath.Join(osConfigDirectory(runtime.GOOS), appName)
	configFileAbsPath      = filepath.Join(configurationDirectory, configFileName)
)

// Configuration ...
type Configuration struct {
	Server   ServerConfiguration
	Database DatabaseConfiguration
	Email    EmailConfiguration
}

// ServerConfiguration is the required paramters to set up a server
type ServerConfiguration struct {
	Port       string `default:"3625"`
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

// EmailConfiguration is the required paramters to send emails
type EmailConfiguration struct {
	Host     string `default:"smtp.passwall.io"`
	Port     string `default:"25"`
	Username string `default:"hello@passwall.io"`
	Password string `default:"password"`
	From     string `default:"hello@passwall.io"`
	Admin    string `default:"hello@passwall.io"`
}

// SetupConfigDefaults ...
func SetupConfigDefaults() (*Configuration, error) {

	// initialize viper configuration
	initializeConfig()

	// Bind environment variables
	bindEnvs()

	// Set default values
	setDefaults()

	// Read or create configuration file
	if err := readConfiguration(); err != nil {
		return nil, err
	}

	// Auto read env variables
	viper.AutomaticEnv()

	// Unmarshal config file to struct
	if err := viper.Unmarshal(&configuration); err != nil {
		return nil, err
	}

	return configuration, nil
}

// read configuration from file
func readConfiguration() error {
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		// if file does not exist, simply create one
		if _, err := os.Stat(configFileAbsPath + configFileExt); os.IsNotExist(err) {
			os.MkdirAll(configurationDirectory, 0755)
			os.Create(configFileAbsPath + configFileExt)
		} else {
			return err
		}
		// let's write defaults
		if err := viper.WriteConfig(); err != nil {
			return err
		}
	}
	return nil
}

// initialize the configuration manager
func initializeConfig() {
	viper.AddConfigPath(configurationDirectory)
	viper.SetConfigName(configFileName)
	viper.SetConfigType(configType)
}

func bindEnvs() {
	viper.BindEnv("server.port", "PORT")
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

	viper.BindEnv("email.host", "PW_EMAIL_HOST")
	viper.BindEnv("email.port", "PW_EMAIL_PORT")
	viper.BindEnv("email.username", "PW_EMAIL_USERNAME")
	viper.BindEnv("email.password", "PW_EMAIL_PASSWORD")
	viper.BindEnv("email.from", "PW_EMAIL_FROM")
	viper.BindEnv("email.admin", "PW_EMAIL_ADMIN")

	viper.BindEnv("backup.folder", "PW_BACKUP_FOLDER")
	viper.BindEnv("backup.rotation", "PW_BACKUP_ROTATION")
	viper.BindEnv("backup.period", "PW_BACKUP_PERIOD")
}

func setDefaults() {

	// Server defaults
	viper.SetDefault("server.port", "3625")
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

	// Email defaults
	viper.SetDefault("email.host", "smtp.passwall.io")
	viper.SetDefault("email.port", "25")
	viper.SetDefault("email.username", "hello@passwall.io")
	viper.SetDefault("email.password", "password")
	viper.SetDefault("email.from", "hello@passwall.io")
	viper.SetDefault("email.admin", "hello@passwall.io")

	// Backup defaults
	viper.SetDefault("backup.folder", "./store/")
	viper.SetDefault("backup.rotation", 7)
	viper.SetDefault("backup.period", "24h")
}

// returns OS dependent config directory
func osConfigDirectory(osName string) (osConfigDirectory string) {
	switch osName {
	case "windows":
		osConfigDirectory = os.Getenv("APPDATA")
	case "darwin":
		osConfigDirectory = os.Getenv("HOME") + "/Library/Application Support"
	case "linux":
		osConfigDirectory = os.Getenv("HOME") + "/.config"
	}
	return osConfigDirectory
}
