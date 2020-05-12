package router

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
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
		if !s.Tokens().Any(uuid) {
			userid, _ := strconv.Atoi(fmt.Sprintf("%.f", claims["user_id"]))
			s.Tokens().Delete(userid)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next(w, r)
	})
}
