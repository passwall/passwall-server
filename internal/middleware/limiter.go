package middleware

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	limitergin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// LimiterMW limits login attempts
func LimiterMW() gin.HandlerFunc {
	// You can also use the simplified format "<limit>-<period>"", with the given
	// periods:
	//
	// * "S": second
	// * "M": minute
	// * "H": hour
	// * "D": day
	//
	// Examples:
	//
	// * 5 reqs/second: "5-S"
	// * 10 reqs/minute: "10-M"
	// * 1000 reqs/hour: "1000-H"
	// * 2000 reqs/day: "2000-D"

	// TODO: This limit (3-M) can be defined in config file.
	// However to do this, a documentation is needed
	rate, err := limiter.NewRateFromFormatted("5-M")
	if err != nil {
		log.Fatal(err)
	}
	store := memory.NewStore()
	limiterMW := limitergin.NewMiddleware(limiter.New(store, rate))
	return limiterMW
}
