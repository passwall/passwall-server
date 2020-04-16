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

func readFromEnv() {

	// This is for checking env variables
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("PORT must be set")
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
			Driver: os.Getenv("DRIVER"),
			Dbname: os.Getenv("DBNAME"),
		},
	}

	Config = configuration
}
