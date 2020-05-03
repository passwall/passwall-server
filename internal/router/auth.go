package router

import (
	"encoding/json"
	"net/http"

	"github.com/pass-wall/passwall-server/internal/app"
)

//Auth verify authentication
func Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	err := app.TokenValid(r)
	if err != nil {
		response, _ := json.Marshal("TOKEN_ERROR")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(response)
		return
	}

	next(w, r)
}
