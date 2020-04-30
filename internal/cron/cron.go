package cron

import (
	"fmt"

	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

// Setup ...
func Setup() {
	backupPeriod := viper.GetString("backup.period")
	cron := cron.New()
	// TODO: This 24h option should be on config file with hours format.
	cron.AddFunc(fmt.Sprintf("@every %s", backupPeriod), func() { app.BackupData() })
	cron.Start()
}
