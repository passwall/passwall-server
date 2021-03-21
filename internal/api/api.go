package api

import (
	"time"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/patrickmn/go-cache"
)

var c *cache.Cache

func init() {
	c = app.CreateCache(time.Minute*5, time.Minute*10)
}
