package router

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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
			// it's not good idea to give error information to the requester.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		claims, _ := token.Claims.(jwt.MapClaims)
		uuid, _ := claims["uuid"].(string)

		//check from db
		tokenRow, tokenExist := s.Tokens().Any(uuid)

		if !tokenExist {
			userid, _ := strconv.Atoi(fmt.Sprintf("%.f", claims["user_id"]))
			s.Tokens().Delete(userid)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctxAuthorized := claims["authorized"].(bool)
		ctxUserID := claims["user_id"].(float64)
		ctxSchema := fmt.Sprintf("user%v", claims["user_id"])
		ctxTransmissionKey := tokenRow.TransmissionKey

		ctx := r.Context()
		ctxWithID := context.WithValue(ctx, "id", ctxUserID)
		ctxWithAuthorized := context.WithValue(ctxWithID, "authorized", ctxAuthorized)
		ctxWithSchema := context.WithValue(ctxWithAuthorized, "schema", ctxSchema)
		ctxWithTransmissionKey := context.WithValue(ctxWithSchema, "transmissionKey", ctxTransmissionKey)

		// These context variables can be accesable with
		// ctxAuthorized := r.Context().Value("authorized").(bool)
		// ctxID := r.Context().Value("id").(float64)

		next(w, r.WithContext(ctxWithTransmissionKey))
	})
}
