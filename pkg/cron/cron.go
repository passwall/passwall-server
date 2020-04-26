package cron

import (
	"github.com/pass-wall/passwall-server/util"
	"github.com/robfig/cron/v3"
)

// Setup ...
func Setup() {
	cron := cron.New()
	// TODO: This 24h option should be on config file with hours format.
	cron.AddFunc("@every 24h", func() { util.BackupData() })
	cron.Start()
}
