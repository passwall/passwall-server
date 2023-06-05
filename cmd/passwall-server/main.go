package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/router"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/pkg/buildvars"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/logger"
)

func main() {
	// Set current working directory to make logger and config use the application dir
	if err := os.Chdir(filepath.Dir(appFilePath())); err != nil {
		logger.Fatalf("os.Chdir failed error: %v", err)
	}

	logStartupInfo()

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

	msg := fmt.Sprintf("Passwall Server is up and running on '%s' in '%s' mode", cfg.Server.Port, cfg.Server.Env)
	fmt.Println(msg)
	logger.Infof("Passwall Server is up and running on %s", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}

func logStartupInfo() {
	args := os.Args
	if args == nil {
		args = []string{}
	}

	logger.Infof("Version: %s Commit ID: %s Build Time: %s", buildvars.Version, buildvars.CommitID, buildvars.BuildTime)

	// Important: %q quotes the each slice item and provides better logging but it panicks! if slice is nil.
	logger.Infof("Application arguments: %q", args)
}

// appFilePath returns the file path of the executable that is currently running
func appFilePath() string {
	path, err := os.Executable()
	if err != nil {
		// Fallback to args array which may not always be the full path
		return os.Args[0]
	}
	return path
}
