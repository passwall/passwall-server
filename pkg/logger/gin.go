package logger

import (
	"time"

	"github.com/gin-gonic/gin"
)

// GinLogger returns a Gin middleware that uses our logger format
// Format matches: INFO 2025-12-23T08:44:20Z 1.1.2 [message] file:... func:...
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get request details
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if raw != "" {
			path = path + "?" + raw
		}

		// Log in our standard format (single line)
		HTTPInfof("[GIN] %d | %13v | %15s | %-7s %s",
			statusCode,
			latency,
			clientIP,
			method,
			path,
		)

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				HTTPErrorf("[GIN] Error: %v", e.Err)
			}
		}
	}
}

// GinRecovery returns a Gin middleware that recovers from panics
func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				HTTPErrorf("[GIN] Panic recovered: %v", err)
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}

// GinWriter returns an io.Writer for Gin's startup messages
// This captures [GIN-debug] messages during route registration
type ginWriter struct{}

func (w *ginWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	// Remove trailing newline for consistent formatting
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	// Use Infof directly to get proper file/func info
	HTTPInfof("%s", msg)
	return len(p), nil
}

// GetWriter returns an io.Writer that writes to our logger
func GetWriter() *ginWriter {
	return &ginWriter{}
}
