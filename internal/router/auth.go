package router

import (
	"context"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/urfave/negroni"
)

//Auth verify authentication
func Auth(s storage.Store) negroni.HandlerFunc {

	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

		var tokenstr string
		bearerToken := r.Header.Get("Authorization")
		strArr := strings.Split(bearerToken, " ")
		if len(strArr) == 2 {
			tokenstr = strArr[1]
		}

		token, err := app.TokenValid(tokenstr)
		if err != nil {
			if token != nil {
				claims, _ := token.Claims.(jwt.MapClaims)
				uuid, _ := claims["uuid"].(string)
				s.Tokens().DeleteByUUID(uuid)
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		claims, _ := token.Claims.(jwt.MapClaims)
		uuid, _ := claims["uuid"].(string)

		// Check token from tokens db table
		tokenRow, tokenExist := s.Tokens().Any(uuid)

		// Get User UUID from claims
		ctxUserUUID, ok := claims["user_uuid"].(string)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Get user details from db by User UUID
		user, err := s.Users().FindByUUID(ctxUserUUID)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Token invalidation for old token usage
		if !tokenExist {
			s.Tokens().Delete(int(user.ID))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Admin or Member
		ctxAuthorized, ok := claims["authorized"].(bool)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctxSchema := user.Schema
		ctxTransmissionKey := tokenRow.TransmissionKey

		ctx := r.Context()
		ctxWithUUID := context.WithValue(ctx, "uuid", ctxUserUUID)
		ctxWithAuthorized := context.WithValue(ctxWithUUID, "authorized", ctxAuthorized)
		ctxWithSchema := context.WithValue(ctxWithAuthorized, "schema", ctxSchema)
		ctxWithTransmissionKey := context.WithValue(ctxWithSchema, "transmissionKey", ctxTransmissionKey)

		// These context variables can be accesable with
		// ctxAuthorized := r.Context().Value("authorized").(bool)
		// ctxID := r.Context().Value("id").(float64)

		next(w, r.WithContext(ctxWithTransmissionKey))
	})
}
