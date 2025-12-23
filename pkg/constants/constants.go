package constants

import "os"

const (
	ConfigFilePath = "./config.yml"
	CookieName     = "passwall_token"
)

const (
	EnvPrefix = "PW"
	EnvDev    = "dev"
	EnvProd   = "prod"
)

func IsDev() bool {
	return os.Getenv("APP_ENV") == "dev"
}
