package router

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v4"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/pkg/logger"
	"github.com/spf13/viper"
	"github.com/urfave/negroni"
)

// Create the JWT key used to create the signature
var jwtKey = []byte(viper.GetString("server.secret"))

// Auth is a middleware that checks for a valid JWT token
func Auth(s storage.Store) negroni.HandlerFunc {

	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		// We can obtain the session token from the requests cookies, which come with every request
		c, err := r.Cookie("passwall_token")
		if err != nil {
			logger.Errorf("Error getting cookie: %v", err)
			if err == http.ErrNoCookie {
				// If the cookie is not set, return an unauthorized status
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// For any other type of error, return a bad request status
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Get the JWT string from the cookie
		tknStr := c.Value

		// Initialize a new instance of `Claims`
		claims := &app.Claims{}

		// Parse the JWT string and store the result in `claims`.
		tkn, err := jwt.ParseWithClaims(
			tknStr,
			claims,
			func(token *jwt.Token) (interface{}, error) {
				return jwtKey, nil
			},
		)
		if err != nil {
			logger.Errorf("Error parsing JWT: %v", err)

			if err == jwt.ErrSignatureInvalid {
				logger.Errorf("Error invalid token signature: %v", err)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !tkn.Valid {
			logger.Errorf("Token is invalid: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Get user details from db by User UUID
		user, err := s.Users().FindByUUID(claims.UserUUID)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctxWithUUID := context.WithValue(ctx, "uuid", claims.UserUUID)
		ctxWithAuthorized := context.WithValue(ctxWithUUID, "authorized", claims.Authorized)
		ctxWithSchema := context.WithValue(ctxWithAuthorized, "schema", user.Schema)
		ctxWithTransmissionKey := context.WithValue(ctxWithSchema, "transmissionKey", claims.TransmissionKey)

		// These context variables can be accesable with
		// ctxAuthorized := r.Context().Value("authorized").(bool)
		// ctxID := r.Context().Value("id").(float64)

		next(w, r.WithContext(ctxWithTransmissionKey))
	})
}
