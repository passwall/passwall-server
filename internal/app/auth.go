package app

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/logger"

	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
)

var (
	//ErrExpiredToken represents message for expired token
	ErrExpiredToken = errors.New("token expired or invalid")
	//ErrUnauthorized represents message for unauthorized
	ErrUnauthorized = errors.New("unauthorized")
)

// Create the JWT key used to create the signature
var jwtKey = []byte(viper.GetString("server.secret"))

// Create a struct that will be encoded to a JWT.
// We add jwt.StandardClaims as an embedded type, to provide fields like expiry time
type Claims struct {
	UUID            string `json:"uuid,omitempty"`
	UserUUID        string `json:"user_uuid,omitempty"`
	Authorized      bool   `json:"authorized,omitempty"`
	TransmissionKey string `json:"transmission_key,omitempty"`
	jwt.RegisteredClaims
}

// CreateToken generates new token with claims: user_uuid, exp, uuid, authorized
func CreateToken(user *model.User) (*http.Cookie, string, error) {

	transmissionKey, err := GenerateSecureKey(viper.GetInt("server.generatedPasswordLength"))
	if err != nil {
		logger.Errorf("Error while generating transmission key: %v\n", err)
		return nil, "", err
	}

	expirationTime := accessTokenExpTime()

	claims := &Claims{
		UUID:            uuid.NewV4().String(),
		UserUUID:        user.UUID.String(),
		Authorized:      isAuthorized(user.Role),
		TransmissionKey: transmissionKey,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	// Declare the token with the algorithm used for signing and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Create the JWT string
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		logger.Errorf("Error while signing the access token: %v", err)
		return nil, transmissionKey, err
	}

	// Finally, we set the client cookie for "token" as the JWT we just generated
	// we also set an expiry time which is the same as the token itself
	return &http.Cookie{
		Name:     constants.CookieName,
		Value:    tokenString,
		Expires:  expirationTime,
		HttpOnly: true,
		Path:     "/",
		// TODO : add secure flag
		// Secure:   true,
	}, transmissionKey, nil
}

func RefreshTokenWithClaims(user *model.User, claims *Claims) (*http.Cookie, error) {

	expirationTime := accessTokenExpTime()

	// Declare the token with the algorithm used for signing and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Create the JWT string
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		logger.Errorf("Error while signing the access token: %v", err)
		return nil, err
	}

	// Finally, we set the client cookie for "token" as the JWT we just generated
	// we also set an expiry time which is the same as the token itself
	return &http.Cookie{
		Name:     constants.CookieName,
		Value:    tokenString,
		Expires:  expirationTime,
		HttpOnly: true,
		Path:     "/",
		// TODO : add secure flag
		// Secure:   true,
	}, nil
}

// DeleteCookie sets defined cookie expire immediately
func DeleteCookie(cookieName string) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
	}
}

func accessTokenExpTime() time.Time {
	expirationDuration := resolveTokenExpireDuration(viper.GetString("server.accessTokenExpireDuration"))
	return time.Now().Add(expirationDuration)
}

func isAuthorized(role string) bool {
	return role == "Admin"
}

//TokenValid ...
func TokenValid(bearerToken string) (*jwt.Token, error) {
	token, err := verifyToken(bearerToken)
	if err != nil {
		if token != nil {
			return token, err
		}
		return nil, err
	}
	if _, ok := token.Claims.(jwt.Claims); !ok && !token.Valid {
		return nil, ErrUnauthorized
	}
	return token, nil
}

//verifyToken verify token
func verifyToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		//Make sure that the token method conform to "SigningMethodHMAC"
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(viper.GetString("server.secret")), nil
	})
	if err != nil {
		return token, ErrExpiredToken
	}
	return token, nil
}

func resolveTokenExpireDuration(config string) time.Duration {
	duration, _ := strconv.ParseInt(config[0:len(config)-1], 10, 64)
	timeFormat := config[len(config)-1:]

	switch timeFormat {
	case "s":
		return time.Duration(time.Second.Nanoseconds() * duration)
	case "m":
		return time.Duration(time.Minute.Nanoseconds() * duration)
	case "h":
		return time.Duration(time.Hour.Nanoseconds() * duration)
	case "d":
		return time.Duration(time.Hour.Nanoseconds() * 24 * duration)
	default:
		return time.Duration(time.Minute.Nanoseconds() * 30)
	}
}
