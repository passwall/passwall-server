package main

import (
	"log"
	"net/http"
	"os"

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
	addr := ":" + viper.GetString("server.port")
	l := log.New(os.Stdout, "[passwall-server] ", 0)
	l.Printf("listening on %s", addr)
	l.Fatal(http.ListenAndServe(addr, api.Router()))
}