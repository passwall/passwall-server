package main

import (
	"github.com/yakuter/gpass/pkg/config"
	"github.com/yakuter/gpass/pkg/database"
	"github.com/yakuter/gpass/pkg/router"
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
