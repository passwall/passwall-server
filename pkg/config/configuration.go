package config

import (
	"os"
	"strconv"

	"log"

	"github.com/spf13/viper"
)

// Config ...
var Config *Configuration

// Configuration ...
type Configuration struct {
	Server   ServerConfiguration
	Database DatabaseConfiguration
}

func isConfigExist(path string) bool {
	_, err := os.Open("./store/config.yml")
	if err != nil {
		return false
	}
	return true
}

// Setup initialize configuration
func Setup() {
	if isConfigExist("./store/config.yml") {
		if err := readFromConfig(); err != nil {
			log.Fatalf("Error reading config file, %s", err)
		}
	} else {
		log.Println("Couldn't file ./store/config.yml. Started with defaul ENV values.")
		readFromEnv()
	}
}

// GetConfig helps you to get configuration data
func GetConfig() *Configuration {
	return Config
}

func readFromConfig() error {
	var configuration *Configuration

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./store")

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	err := viper.Unmarshal(&configuration)
	if err != nil {
		return err
	}

	Config = configuration

	return nil
}

func setEnv() {
	os.Setenv("PORT", "3625")
	os.Setenv("USERNAME", "passwall")
	os.Setenv("PASSWORD", "password")
	os.Setenv("PASSPHRASE", "-G84d}~Yr)H{c=Zx)>@VqM;d~o+$}x9y~X_Ma-otq|ifhP7]?s7OJBYXao,K]-+^")
	os.Setenv("SECRET", "JOa{+KBm5:hj]?k1 wsVJl?*HE(cEB<*WVXkL$qh}B2#Fry{C;j[k}-[|-9G:#b]")
	os.Setenv("TIMEOUT", "24")
	os.Setenv("DB_DRIVER", "sqlite")
	os.Setenv("DB_NAME", "passwall")
	os.Setenv("DB_USERNAME", "user")
	os.Setenv("DB_PASSWORD", "password")
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "5432")

}

func readFromEnv() {

	// This is for checking env variables
	port := os.Getenv("PORT")
	if port == "" {
		log.Println("Environment variables didn't set. Setting it manually.")
		setEnv()
	}

	timeout, _ := strconv.Atoi(os.Getenv("TIMEOUT"))
	configuration := &Configuration{
		Server: ServerConfiguration{
			Port:       os.Getenv("PORT"),
			Username:   os.Getenv("USERNAME"),
			Password:   os.Getenv("PASSWORD"),
			Passphrase: os.Getenv("PASSPHRASE"),
			Secret:     os.Getenv("SECRET"),
			Timeout:    timeout,
		},
		Database: DatabaseConfiguration{
			Driver:   os.Getenv("DB_DRIVER"),
			Dbname:   os.Getenv("DB_NAME"),
			Username: os.Getenv("DB_USERNAME"),
			Password: os.Getenv("DB_PASSWORD"),
			Host:     os.Getenv("DB_HOST"),
			Port:     os.Getenv("DB_PORT"),
		},
	}

	Config = configuration
}
