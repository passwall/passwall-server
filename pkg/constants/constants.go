package constants

import "os"

const (
	ConfigPath = "./config"
	ConfigName = "config"
	CookieName = "passwall_token"
)

const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

func IsDev() bool {
	return os.Getenv("APP_ENV") == "dev"
}
