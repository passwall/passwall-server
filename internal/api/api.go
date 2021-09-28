package api

import (
	"time"

	"github.com/patrickmn/go-cache"
)

var c = cache.New(time.Minute*5, time.Minute*10)
