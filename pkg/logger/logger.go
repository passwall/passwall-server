package logger

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

const (
	skipFrameCount    = 4
	splitAfterPkgName = "github.com/passwall/"
	logFileName       = "passwall-server.log"
)

func init() {
	logger.Out = getWriter()
	logger.Level = logrus.InfoLevel
	logger.Formatter = &formatter{}

	logger.SetReportCaller(true)
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

// getWriter returns io.Writer
func getWriter() io.Writer {
	file, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Errorf("Failed to open log file: %v", err)
		return os.Stdout
	} else {
		return file
	}
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
