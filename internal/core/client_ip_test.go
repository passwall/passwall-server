package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func clientIPFor(t *testing.T, remoteAddr string, headers map[string]string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	ConfigureClientIP(router, []string{"127.0.0.0/8", "172.16.0.0/12"})
	var got string
	router.GET("/", func(c *gin.Context) { got = c.ClientIP() })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	router.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func TestClientIPIgnoresSpoofedForwardingHeaders(t *testing.T) {
	// Through the reverse proxy: X-Real-IP is what nginx saw; a client-supplied
	// X-Forwarded-For prefix must not win.
	assert.Equal(t, "203.0.113.9", clientIPFor(t, "172.19.0.1:5555", map[string]string{
		"X-Real-IP":       "203.0.113.9",
		"X-Forwarded-For": "1.2.3.4, 203.0.113.9",
	}))
	// Proxy without X-Real-IP: X-Forwarded-For is read from the right.
	assert.Equal(t, "203.0.113.9", clientIPFor(t, "172.19.0.1:5555", map[string]string{
		"X-Forwarded-For": "1.2.3.4, 203.0.113.9",
	}))
	// Direct request from an untrusted address: headers are ignored.
	assert.Equal(t, "198.51.100.7", clientIPFor(t, "198.51.100.7:4444", map[string]string{
		"X-Real-IP":       "1.2.3.4",
		"X-Forwarded-For": "1.2.3.4",
	}))
}
