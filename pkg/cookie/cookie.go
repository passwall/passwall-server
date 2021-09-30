package cookie

import (
	"net/http"
	"time"
)

// Create creates a cookie with the given name, token and expiration time.
func Create(name, token string, expire time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    token,
		Expires:  expire,
		HttpOnly: true,
		Path:     "/",
		// TODO : add secure flag
		// Secure:   true,
	}
}

// Delete deletes the cookie with the given name.
func Delete(cookieName string) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
	}
}
