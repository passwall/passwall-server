package main

import (
	"log"
	"os"

	"github.com/passwall/passwall-server/internal/core"
	"github.com/passwall/passwall-server/pkg/buildvars"
	"github.com/passwall/passwall-server/pkg/logger"
)

func main() {
	logStartupInfo()

	// Create and run application
	app, err := core.New()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func logStartupInfo() {
	args := os.Args
	if args == nil {
		args = []string{}
	}

	logger.Infof("Passwall Server started")
	logger.Infof("Version: %s Commit ID: %s Build Time: %s", buildvars.Version, buildvars.CommitID, buildvars.BuildTime)
	logger.Infof("Application arguments: %q", args)
}
