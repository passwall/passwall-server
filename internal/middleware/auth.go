package middleware

import (
	"net/http"

	"github.com/pass-wall/passwall-server/internal/auth"
	"github.com/pass-wall/passwall-server/internal/common"
)

var (
	TokenErr        = "TOKEN_ERROR"
	UnauthorizedErr = "Unauthorized Error"
)

//Auth verify authentication
func Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	err := auth.TokenValid(r)
	if err != nil {
		errs := []string{TokenErr}
		common.RespondWithErrors(w, http.StatusUnauthorized, UnauthorizedErr, errs)
		return
	}

	next(w, r)
}
