package main

import (
	"net/http"
	"time"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/router"
	"github.com/passwall/passwall-server/internal/storage"

	log "github.com/sirupsen/logrus"
)

func main() {
	cfg, err := config.SetupConfigDefaults()
	if err != nil {
		log.Fatal(err)
	}

	logFile, err := config.SetupLogger(cfg)
	if err != nil {
		log.Fatalf("Log folder %s doesn't exist", cfg.Server.LogPath)
	}
	defer logFile.Close()

	db, err := storage.DBConn(&cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	s := storage.New(db)

	srv := &http.Server{
		MaxHeaderBytes: 10, // 10 MB
		Addr:           ":" + cfg.Server.Port,
		WriteTimeout:   time.Second * time.Duration(cfg.Server.Timeout),
		ReadTimeout:    time.Second * time.Duration(cfg.Server.Timeout),
		IdleTimeout:    time.Second * 60,
		Handler:        router.New(s),
	}

	log.Infof("listening on %s", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
