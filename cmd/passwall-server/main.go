package main

import (
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/pass-wall/passwall-server/internal/api"
	"github.com/pass-wall/passwall-server/internal/config"
	"github.com/pass-wall/passwall-server/internal/cron"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/spf13/viper"
)

func main() {
	config.Setup()

	port := ":" + viper.GetString("server.port")
	l := log.New(os.Stdout, "[passwall-server] ", 0)
	l.Printf("listening on %s", port)

	cfg := &storage.Configuration{
		Driver:   viper.GetString("database.driver"),
		DBName:   viper.GetString("database.dbname"),
		Username: viper.GetString("database.username"),
		Password: viper.GetString("database.password"),
		Host:     viper.GetString("database.host"),
		Port:     viper.GetString("database.port"),
		DBPath:   viper.GetString("database.path"),
	}

	s, err := storage.New(cfg)
	if err != nil {
		l.Fatalf("failed to open storage: %s\n", err)
	}

	cron.Setup(s)

	srv := &http.Server{
		Addr:         port,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      api.New(s),
	}

	if err := srv.ListenAndServe(); err != nil {
		l.Fatal(err)
	}
}
