package main

import (
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/pass-wall/passwall-server/internal/api"
	"github.com/pass-wall/passwall-server/internal/cron"
)

func init() {
	cron.Setup()
}

func main() {

	api.Router()

}
