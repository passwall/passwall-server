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
	PORT     string
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

	_ = readFromConfig()
	// if isConfigExist("./store/config.yml") {
	// 	if err := readFromConfig(); err != nil {
	// 		log.Fatalf("Error reading config file, %s", err)
	// 	}
	// } else {
	// 	log.Println("Couldn't file ./store/config.yml. Started with defaul ENV values.")
	// 	readFromEnv()
	// }
}

func readFromConfig() error {
	var configuration *Configuration

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./store")

	// Bind environment variables
	bindEnvs()

	// Set default values
	setDefaults()

	// Enable VIPER to read Environment Variables
	viper.AutomaticEnv()

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

func bindEnvs() {
	viper.BindEnv("server.port", "PORT")
	viper.BindEnv("server.username", "PW_SERVER_USERNAME")
	viper.BindEnv("server.password", "PW_SERVER_PASSWORD")
	viper.BindEnv("server.passphrase", "PW_SERVER_PASSPHRASE")
	viper.BindEnv("server.secret", "PW_SERVER_SECRET")
	viper.BindEnv("server.timeout", "PW_SERVER_TIMEOUT")

	viper.BindEnv("database.driver", "PW_DB_DRIVER")
	viper.BindEnv("database.dbname", "PW_DB_DBNAME")
	viper.BindEnv("database.username", "PW_DB_USERNAME")
	viper.BindEnv("database.password", "PW_DB_PASSWORD")
	viper.BindEnv("database.host", "PW_DB_HOST")
	viper.BindEnv("database.port", "PW_DB_PORT")
}

func setDefaults() {
	viper.SetDefault("server.port", "3625")
	viper.SetDefault("server.username", "passwall")
	viper.SetDefault("server.password", "password")
	viper.SetDefault("server.passphrase", "-G84d}~Yr)H{c=Zx)>@VqM;d~o+$}x9y~X_Ma-otq|ifhP7]?s7OJBYXao,K]-+^")
	viper.SetDefault("server.secret", "JOa{+KBm5:hj]?k1 wsVJl?*HE(cEB<*WVXkL$qh}B2#Fry{C;j[k}-[|-9G:#b]")
	viper.SetDefault("server.timeout", 24)

	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.dbname", "passwall")
	viper.SetDefault("database.username", "user")
	viper.SetDefault("database.password", "password")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5432")
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
		// setEnv()
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
