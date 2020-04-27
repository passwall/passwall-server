package config

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"

	"github.com/spf13/viper"
)

// Config ...
var Config *Configuration

// Configuration ...
type Configuration struct {
	Server   ServerConfiguration
	Database DatabaseConfiguration
	PORT     string
}

// Setup initialize configuration
func Setup() {

	var configuration *Configuration

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./store")

	// Bind environment variables
	bindEnvs()

	// Set default values
	setDefaults()

	if !fileExists("./store/config.yml") {
		viper.Set("server.passphrase", generateSecureKey())
		viper.Set("server.secret", generateSecureKey())
		viper.WriteConfigAs("./store/config.yml")
	}

	// Enable VIPER to read Environment Variables
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Println(err)
	}

	err := viper.Unmarshal(&configuration)
	if err != nil {
		log.Println(err)
	}

	Config = configuration

}

func bindEnvs() {
	viper.BindEnv("server.port", "PORT")
	viper.BindEnv("server.username", "PW_SERVER_USERNAME")
	viper.BindEnv("server.password", "PW_SERVER_PASSWORD")
	viper.BindEnv("server.passphrase", "PW_SERVER_PASSPHRASE")
	viper.BindEnv("server.secret", "PW_SERVER_SECRET")
	viper.BindEnv("server.timeout", "PW_SERVER_TIMEOUT")
	viper.BindEnv("server.generatedPasswordLength", "PW_SERVER_GENERATED_PASSWORD_LENGTH")

	viper.BindEnv("database.driver", "PW_DB_DRIVER")
	viper.BindEnv("database.dbname", "PW_DB_DBNAME")
	viper.BindEnv("database.username", "PW_DB_USERNAME")
	viper.BindEnv("database.password", "PW_DB_PASSWORD")
	viper.BindEnv("database.host", "PW_DB_HOST")
	viper.BindEnv("database.port", "PW_DB_PORT")
	viper.BindEnv("database.path", "PW_DB_PATH")

	viper.BindEnv("backup.folder", "PW_BACKUP_FOLDER")
	viper.BindEnv("backup.rotation", "PW_BACKUP_ROTATION")
}

func setDefaults() {
	viper.SetDefault("server.port", "3625")
	viper.SetDefault("server.username", "passwall")
	viper.SetDefault("server.password", "password")
	viper.SetDefault("server.passphrase", "passphrase-for-encrypting-passwords-do-not-forget")
	viper.SetDefault("server.secret", "secret-key-for-JWT-TOKEN")
	viper.SetDefault("server.timeout", 24)
	viper.SetDefault("server.generatedPasswordLength", 16)

	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.dbname", "passwall")
	viper.SetDefault("database.username", "user")
	viper.SetDefault("database.password", "password")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.path", "./store/passwall.db")

	viper.SetDefault("backup.folder", "./store/")
	viper.SetDefault("backup.rotation", "7")
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
