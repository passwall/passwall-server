package logger

import (
	"fmt"
	"strings"
)

// Adapter wraps the logger to implement service.Logger interface with structured logging
type Adapter struct{}

// NewAdapter creates a new logger adapter
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Debug(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) > 0 {
		formatted := formatKeyValues(keysAndValues...)
		Debugf("%s", msg+" "+formatted)
	} else {
		Debugf("%s", msg)
	}
}

func (a *Adapter) Info(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) > 0 {
		formatted := formatKeyValues(keysAndValues...)
		Infof("%s", msg+" "+formatted)
	} else {
		Infof("%s", msg)
	}
}

func (a *Adapter) Warn(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) > 0 {
		formatted := formatKeyValues(keysAndValues...)
		Warnf("%s", msg+" "+formatted)
	} else {
		Warnf("%s", msg)
	}
}

func (a *Adapter) Error(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) > 0 {
		formatted := formatKeyValues(keysAndValues...)
		Errorf("%s", msg+" "+formatted)
	} else {
		Errorf("%s", msg)
	}
}

// formatKeyValues formats key-value pairs for structured logging
// Example: formatKeyValues("key1", "value1", "key2", 123) -> "key1=value1 key2=123"
func formatKeyValues(keysAndValues ...interface{}) string {
	if len(keysAndValues) == 0 {
		return ""
	}

	var pairs []string
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key := fmt.Sprintf("%v", keysAndValues[i])
			value := fmt.Sprintf("%v", keysAndValues[i+1])

			// Add quotes if value contains spaces
			if strings.Contains(value, " ") {
				pairs = append(pairs, fmt.Sprintf("%s=\"%s\"", key, value))
			} else {
				pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
			}
		} else {
			// Odd number of arguments, append the last key without value
			pairs = append(pairs, fmt.Sprintf("%v=<missing>", keysAndValues[i]))
		}
	}

	return strings.Join(pairs, " ")
}
