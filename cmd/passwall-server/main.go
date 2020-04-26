package main

import (
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/pass-wall/passwall-server/internal/config"
	"github.com/pass-wall/passwall-server/internal/cron"
	"github.com/pass-wall/passwall-server/internal/store"
	"github.com/pass-wall/passwall-server/internal/router"
	"github.com/spf13/viper"
)

func init() {
	config.Setup()
	store.Setup()
	cron.Setup()
}

func main() {
	r := router.Setup()
	r.Run("0.0.0.0:" + viper.GetString("server.port"))
}
