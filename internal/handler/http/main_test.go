package http

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMain sets Gin's process-wide mode once, before any test in this
// package runs. gin.SetMode writes globals that engine creation and
// request handling read, so calling it from parallel tests races.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
