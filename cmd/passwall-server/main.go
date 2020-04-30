package main

import (
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/pass-wall/passwall-server/internal/api"
	"github.com/pass-wall/passwall-server/internal/config"
	"github.com/pass-wall/passwall-server/internal/cron"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/spf13/viper"
)

func init() {
	config.Setup()
	storage.Setup()
	cron.Setup()
}

func main() {

	n := api.Router()
	n.Run(":" + viper.GetString("server.port"))

}
