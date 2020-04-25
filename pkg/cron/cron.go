package cron

import (
	"github.com/pass-wall/passwall-server/util"
	"github.com/robfig/cron/v3"
)

func Setup() {
	cron := cron.New()
	cron.AddFunc("@every 24h", func() { util.BackupData() })
	cron.Start()
}
