package cron

import (
	"github.com/robfig/cron/v3"
	"github.com/pass-wall/passwall-server/internal/app"
)

// Setup ...
func Setup() {
	cron := cron.New()
	// TODO: This 24h option should be on config file with hours format.
	cron.AddFunc("@every 24h", func() { app.BackupData() })
	cron.Start()
}
