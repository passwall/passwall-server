package router

import (
	"net/http"

	"github.com/didip/tollbooth"
	"github.com/urfave/negroni"
)

// LimitHandler ...
func LimitHandler() negroni.HandlerFunc {
	lmt := tollbooth.NewLimiter(5, nil)

	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		httpError := tollbooth.LimitByRequest(lmt, w, r)
		if httpError != nil {
			w.Header().Add("Content-Type", lmt.GetMessageContentType())
			w.WriteHeader(httpError.StatusCode)
			w.Write([]byte(httpError.Message))
			return
		}
		next(w, r)
	})
}
