package api

import (
	"time"

	"github.com/passwall/passwall-server/internal/app"
)

var c = app.CreateCache(time.Minute*5, time.Minute*10)
