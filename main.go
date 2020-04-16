package main

import (
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/pass-wall/passwall-api/pkg/config"
	"github.com/pass-wall/passwall-api/pkg/database"
	"github.com/pass-wall/passwall-api/pkg/router"
)

func init() {
	config.Setup()
	database.Setup()
}

func main() {
	config := config.GetConfig()

	r := router.Setup()
	r.Run("0.0.0.0:" + config.Server.Port)
}
