package middleware

import (
	"fmt"
	"net/http"
)

// CORS ...
func CORS(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	fmt.Print("cors")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, HEAD")
	next(w, r)
}
