package middleware

import (
	"fmt"
	"net/http"
)

type login struct {
	Username string `form:"Username" json:"Username" binding:"required"`
	Password string `form:"Password" json:"Password" binding:"required"`
}

// User demo
type User struct {
	Username string
	Password string
}

var identityKey = "username"

//Auth ...
func Auth(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	//TODO verify token
	fmt.Print("auth")

	bearerToken := r.Header.Get("Authorization")
	fmt.Println(bearerToken)
	next(w, r)
}
