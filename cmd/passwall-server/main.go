package main

import (
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/router"
	"github.com/pass-wall/passwall-server/internal/storage"
)

func main() {
	cfg := setupConfigDefaults()

	logger := log.New(os.Stdout, "[passwall-server] ", 0)
	logger.Printf("listening on %s", cfg.Server.Port)

	s, err := storage.New(&cfg.Database)
	if err != nil {
		logger.Fatalf("failed to open storage: %s\n", err)
	}

	app.StartCronJob(s)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		WriteTimeout: time.Second * time.Duration(cfg.Server.Timeout),
		ReadTimeout:  time.Second * time.Duration(cfg.Server.Timeout),
		IdleTimeout:  time.Second * 60,
		Handler:      router.New(s),
	}

	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal(err)
	}
}
