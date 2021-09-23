package main

import (
	"net/http"
	"time"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/router"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/logger"
)

func main() {
	cfg, err := config.Init(constants.ConfigPath, constants.ConfigName)
	if err != nil {
		logger.Fatalf("config.Init: %s", err)
	}

	db, err := storage.DBConn(&cfg.Database)
	if err != nil {
		logger.Fatalf("storage.DBConn: %s", err)
	}

	s := storage.New(db)

	app.MigrateSystemTables(s)

	srv := &http.Server{
		MaxHeaderBytes: 10, // 10 MB
		Addr:           ":" + cfg.Server.Port,
		WriteTimeout:   time.Second * time.Duration(cfg.Server.Timeout),
		ReadTimeout:    time.Second * time.Duration(cfg.Server.Timeout),
		IdleTimeout:    time.Second * 60,
		Handler:        router.New(s),
	}

	logger.Infof("listening on %s", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}
