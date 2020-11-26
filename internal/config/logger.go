package config

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// SetupLogger checks log path and create log file.
// Also use logrus hooks to send logs 3rd party apps like Sentry.
func SetupLogger(cfg *Configuration) (*os.File, error) {
	var err error
	var logFile *os.File
	if cfg.Server.Environment == "production" {
		logPath := filepath.Join(cfg.Server.LogPath, "passwall.log")
		logFile, err = os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		log.SetOutput(logFile)
		log.SetReportCaller(true)

		/** Sentry Hokk **/
		// hook, err := logrus_sentry.NewSentryHook(detectDSN(service), []log.Level{
		// 	log.PanicLevel,
		// 	log.FatalLevel,
		// 	log.ErrorLevel,
		// })
		// hook.StacktraceConfiguration.Enable = true

		// if err == nil {
		// 	log.AddHook(hook)
		// }
	}

	return logFile, err
}
