package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/router"
	"github.com/passwall/passwall-server/internal/storage"
)

func main() {
	logger := log.New(os.Stdout, "[passwall-server] ", 0)

	cfg, err := config.SetupConfigDefaults()
	if err != nil {
		log.Fatal(err)
	}

	db, err := storage.DBConn(&cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	s := storage.New(db)

	// Migrate database tables
	// TODO: Migrate should be in storege.New functions of categories
	app.MigrateSystemTables(s)

	// Start cron jobs like backup
	// app.StartCronJob(s)

	fmt.Println(cfg.Server.Dir)

	srv := &http.Server{
		MaxHeaderBytes: 10, // 10 MB
		Addr:           ":" + cfg.Server.Port,
		WriteTimeout:   time.Second * time.Duration(cfg.Server.Timeout),
		ReadTimeout:    time.Second * time.Duration(cfg.Server.Timeout),
		IdleTimeout:    time.Second * 60,
		Handler:        router.New(s),
	}

	logger.Printf("listening on %s", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal(err)
	}
}
