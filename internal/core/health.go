package core

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	readinessDatabaseTimeout = 2 * time.Second
	readinessCacheTTL        = time.Second
)

type databasePinger interface {
	Ping(context.Context) error
}

type readinessProbe struct {
	db        databasePinger
	mu        sync.Mutex
	checkedAt time.Time
	healthy   bool
}

func (p *readinessProbe) check(ctx context.Context) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.checkedAt.IsZero() && time.Since(p.checkedAt) < readinessCacheTTL {
		return p.healthy
	}

	pingCtx, cancel := context.WithTimeout(ctx, readinessDatabaseTimeout)
	defer cancel()
	p.healthy = p.db.Ping(pingCtx) == nil
	p.checkedAt = time.Now()
	return p.healthy
}

func registerHealthRoutes(router *gin.Engine, db databasePinger, isReady func() bool) {
	probe := &readinessProbe{db: db}

	router.GET("/health", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/ready", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if !isReady() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}

		if !probe.check(c.Request.Context()) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
