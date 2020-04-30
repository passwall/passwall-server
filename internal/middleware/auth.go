package middleware

import (
	"net/http"

	"github.com/pass-wall/passwall-server/internal/auth"
	"github.com/pass-wall/passwall-server/internal/common"
)

//Auth verify authentication
func Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	err := auth.TokenValid(r)
	if err != nil {
		common.RespondWithError(w, http.StatusUnauthorized, "Unauthorized Error")
		return
	}

	next(w, r)
}
