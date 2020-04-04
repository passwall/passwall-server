package main

import (
	"gpass/pkg/config"
	"gpass/pkg/database"
	"gpass/pkg/router"
)

func init() {
	config.Setup()
	database.Setup()
}

func main() {
	config := config.GetConfig()

	r := router.Setup()
	r.Run("127.0.0.1:" + config.Server.Port)
}
