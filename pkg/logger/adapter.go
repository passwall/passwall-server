package logger

// Adapter wraps the logger to implement service.Logger interface
type Adapter struct{}

// NewAdapter creates a new logger adapter
func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Debug(msg string, keysAndValues ...interface{}) {
	Debugf(msg+" %v", keysAndValues...)
}

func (a *Adapter) Info(msg string, keysAndValues ...interface{}) {
	Infof(msg+" %v", keysAndValues...)
}

func (a *Adapter) Warn(msg string, keysAndValues ...interface{}) {
	Warnf(msg+" %v", keysAndValues...)
}

func (a *Adapter) Error(msg string, keysAndValues ...interface{}) {
	Errorf(msg+" %v", keysAndValues...)
}

