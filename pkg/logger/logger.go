package logger

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()
var httpLogger = logrus.New()

const (
	skipFrameCount    = 4
	splitAfterPkgName = "github.com/passwall/"
	logFileName       = "passwall-server.log"
	httpLogFileName   = "passwall-http.log"
)

func init() {
	logger.Out = getAppWriter()
	logger.Level = logrus.InfoLevel
	logger.Formatter = &formatter{}

	logger.SetReportCaller(true)

	httpLogger.Out = getHTTPWriter()
	httpLogger.Level = logrus.InfoLevel
	httpLogger.Formatter = &formatter{}
	httpLogger.SetReportCaller(true)
}

// SetLogLevel sets log level
func SetLogLevel(level logrus.Level) {
	logger.Level = level
}

// Fields sets fields on the logger.
type Fields logrus.Fields

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	if logger.Level >= logrus.DebugLevel {
		entry := newEntry()
		entry.Debugf(format, args...)
	}
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	if logger.Level >= logrus.InfoLevel {
		entry := newEntry()
		entry.Infof(format, args...)
	}
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	if logger.Level >= logrus.WarnLevel {
		entry := newEntry()
		entry.Warnf(format, args...)
	}
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	if logger.Level >= logrus.ErrorLevel {
		entry := newEntry()
		entry.Errorf(format, args...)
	}
}

// Fatalf logs a message at level Fatal on the standard logger.
func Fatalf(format string, args ...interface{}) {
	if logger.Level >= logrus.FatalLevel {
		entry := newEntry()
		entry.Fatalf(format, args...)
	}
}

func newEntry() *logrus.Entry {
	file, function, line := callerInfo(skipFrameCount, splitAfterPkgName)

	entry := logger.WithFields(logrus.Fields{})
	entry.Data["file"] = file
	entry.Data["line"] = line
	entry.Data["function"] = function
	return entry
}

func newHTTPEntry() *logrus.Entry {
	file, function, line := callerInfo(skipFrameCount, splitAfterPkgName)

	entry := httpLogger.WithFields(logrus.Fields{})
	entry.Data["file"] = file
	entry.Data["line"] = line
	entry.Data["function"] = function
	return entry
}

// HTTPDebugf logs a message to the HTTP log.
func HTTPDebugf(format string, args ...interface{}) {
	if httpLogger.Level >= logrus.DebugLevel {
		entry := newHTTPEntry()
		entry.Debugf(format, args...)
	}
}

// HTTPInfof logs a message to the HTTP log.
func HTTPInfof(format string, args ...interface{}) {
	if httpLogger.Level >= logrus.InfoLevel {
		entry := newHTTPEntry()
		entry.Infof(format, args...)
	}
}

// HTTPWarnf logs a message to the HTTP log.
func HTTPWarnf(format string, args ...interface{}) {
	if httpLogger.Level >= logrus.WarnLevel {
		entry := newHTTPEntry()
		entry.Warnf(format, args...)
	}
}

// HTTPErrorf logs a message to the HTTP log.
func HTTPErrorf(format string, args ...interface{}) {
	if httpLogger.Level >= logrus.ErrorLevel {
		entry := newHTTPEntry()
		entry.Errorf(format, args...)
	}
}

// callerInfo grabs caller file, function and line number
func callerInfo(skip int, pkgName string) (file, function string, line int) {

	// Grab frame
	pc := make([]uintptr, 1)
	n := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()

	// Set file, function and line number
	file = trimPkgName(frame.File, pkgName)
	function = trimPkgName(frame.Function, pkgName)
	line = frame.Line

	return
}

// trimPkgName trims string after splitStr
func trimPkgName(frameStr, splitStr string) string {
	count := strings.LastIndex(frameStr, splitStr)
	if count > -1 {
		frameStr = frameStr[count+len(splitStr):]
	}

	return frameStr
}

// getAppWriter returns io.Writer
func getAppWriter() io.Writer {
	// 1) Explicit override
	logPath := strings.TrimSpace(os.Getenv(constants.LogPathEnv))
	if logPath != "" {
		if st, err := os.Stat(logPath); err == nil && st.IsDir() {
			logPath = filepath.Join(logPath, logFileName)
		}
	} else {
		// Default: next to the running executable (independent from cwd and PW_WORK_DIR).
		exePath, err := os.Executable()
		if err == nil && exePath != "" {
			exeDir := filepath.Dir(exePath)
			if exeDir != "" && exeDir != "." {
				logPath = filepath.Join(exeDir, logFileName)
			} else {
				logPath = logFileName
			}
		} else {
			logPath = logFileName
		}
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Errorf("Failed to open log file: %v", err)
		return os.Stdout
	} else {
		return file
	}
}

// getHTTPWriter returns io.Writer for HTTP logs
func getHTTPWriter() io.Writer {
	logPath := strings.TrimSpace(os.Getenv(constants.HTTPLogPathEnv))
	if logPath != "" {
		if st, err := os.Stat(logPath); err == nil && st.IsDir() {
			logPath = filepath.Join(logPath, httpLogFileName)
		}
	} else {
		exePath, err := os.Executable()
		if err == nil && exePath != "" {
			exeDir := filepath.Dir(exePath)
			if exeDir != "" && exeDir != "." {
				logPath = filepath.Join(exeDir, httpLogFileName)
			} else {
				logPath = httpLogFileName
			}
		} else {
			logPath = httpLogFileName
		}
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Errorf("Failed to open http log file: %v", err)
		return os.Stdout
	}
	return file
}

// Formatter implements logrus.Formatter interface.
type formatter struct {
	prefix string
}

// Format building log message.
func (f *formatter) Format(entry *logrus.Entry) ([]byte, error) {
	var sb bytes.Buffer
	sb.WriteString(strings.ToUpper(entry.Level.String()))
	sb.WriteString(" ")
	sb.WriteString(entry.Time.Format(time.RFC3339))
	sb.WriteString(" ")
	sb.WriteString("1.1.2")
	sb.WriteString(" ")
	sb.WriteString(f.prefix)
	sb.WriteString(entry.Message)
	sb.WriteString(" ")
	file, ok := entry.Data["file"].(string)
	if ok {
		sb.WriteString("file:")
		sb.WriteString(file)
	}
	line, ok := entry.Data["line"].(int)
	if ok {
		sb.WriteString(":")
		sb.WriteString(strconv.Itoa(line))
	}
	function, ok := entry.Data["function"].(string)
	if ok {
		sb.WriteString(" ")
		sb.WriteString("func:")
		sb.WriteString(function)
	}
	sb.WriteString("\n")

	return sb.Bytes(), nil
}
