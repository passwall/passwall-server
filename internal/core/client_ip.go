package core

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// ConfigureClientIP makes c.ClientIP() trust forwarding headers only from
// the given proxy networks. The reverse proxy sets X-Real-IP to the TCP peer
// it saw; X-Forwarded-For is the fallback and is walked from the right, so a
// client-supplied prefix is ignored.
func ConfigureClientIP(router *gin.Engine, trustedProxies []string) {
	router.ForwardedByClientIP = true
	router.RemoteIPHeaders = []string{"X-Real-IP", "X-Forwarded-For"}
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		slog.Error("invalid trusted proxies; forwarding headers are ignored", "error", err)
		_ = router.SetTrustedProxies(nil)
	}
}
