package main

import (
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/pass-wall/passwall-api/pkg/config"
	"github.com/pass-wall/passwall-api/pkg/database"
	"github.com/pass-wall/passwall-api/pkg/router"
	"github.com/spf13/viper"
)

func init() {
	config.Setup()
	database.Setup()
}

func main() {
	r := router.Setup()
	r.Run("0.0.0.0:" + viper.GetString("server.port"))
}
