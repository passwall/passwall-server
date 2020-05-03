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

	l := log.New(os.Stdout, "[passwall-server] ", 0)
	l.Printf("listening on %s", cfg.Server.Port)

	s, err := storage.New(&cfg.Database)
	if err != nil {
		l.Fatalf("failed to open storage: %s\n", err)
	}

	app.StartCronJob(s)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      router.New(s),
	}

	if err := srv.ListenAndServe(); err != nil {
		l.Fatal(err)
	}
}
