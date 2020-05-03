package app

import (
	"fmt"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

// StartCronJob ...
func StartCronJob(s storage.Store) {
	backupPeriod := viper.GetString("backup.period")
	cron := cron.New()
	cron.AddFunc(fmt.Sprintf("@every %s", backupPeriod), func() { BackupData(s) })
	cron.Start()
}
