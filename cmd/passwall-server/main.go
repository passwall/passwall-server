package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/passwall/passwall-server/internal/core"
	"github.com/passwall/passwall-server/pkg/buildvars"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/logger"
)

func main() {
	if showVersion() {
		return
	}
	applyWorkDir()
	logStartupInfo()

	// Create application context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Handle signals in a goroutine
	go func() {
		sig := <-sigChan
		logger.Infof("Received signal: %v", sig)
		fmt.Printf("\n⏳ Shutting down gracefully (signal: %v)...\n", sig)
		cancel() // Cancel the application context
	}()

	// Create and run application with context
	app, err := core.New(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	if err := app.Run(ctx); err != nil {
		log.Fatalf("Application error: %v", err)
	}

	logger.Infof("Application exited successfully")
	fmt.Println("✅ Server stopped")
}

func applyWorkDir() {
	workDir := strings.TrimSpace(os.Getenv(constants.WorkDirEnv))
	if workDir == "" {
		return
	}

	if err := os.MkdirAll(workDir, 0755); err != nil {
		log.Fatalf("Failed to create work dir %q: %v", workDir, err)
	}

	if err := os.Chdir(workDir); err != nil {
		log.Fatalf("Failed to change work dir to %q: %v", workDir, err)
	}
}

// showVersion prints version, commit id, build id and build time to stdout and returns true if -v/--version was passed.
func showVersion() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" {
			fmt.Printf("Version: %s\nCommit ID: %s\nBuild ID: %s\nBuild Time: %s\n",
				buildvars.Version, buildvars.CommitID, buildvars.BuildID, buildvars.BuildTime)
			return true
		}
	}
	return false
}

func logStartupInfo() {
	args := os.Args
	if args == nil {
		args = []string{}
	}

	logger.Infof("Passwall Server started")
	logger.Infof("Version: %s Commit ID: %s Build ID: %s Build Time: %s", buildvars.Version, buildvars.CommitID, buildvars.BuildID, buildvars.BuildTime)
	logger.Infof("Application arguments: %q", args)
}
