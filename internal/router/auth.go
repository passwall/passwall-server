package router

import (
	"net/http"
	"strings"

	"github.com/pass-wall/passwall-server/internal/app"
)

//Auth verify authentication
func Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	var token string
	bearerToken := r.Header.Get("Authorization")
	strArr := strings.Split(bearerToken, " ")
	if len(strArr) == 2 {
		token = strArr[1]
	}

	err := app.TokenValid(token)
	if err != nil {
		// it's not good idea to give error information to the requester.
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	next(w, r)
}
