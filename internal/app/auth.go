package app

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/logger"
	"github.com/patrickmn/go-cache"

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

// CreateCache
func CreateCache(defaultExpiration, cleanupInterval time.Duration) *cache.Cache {
	return cache.New(defaultExpiration, cleanupInterval)
}

// CreateToken generates new token with claims: user_uuid, exp, uuid, authorized

func CreateToken(user *model.User) (*http.Cookie, error) {

	transmissionKey, err := GenerateSecureKey(viper.GetInt("server.generatedPasswordLength"))
	if err != nil {
		logger.Errorf("Error while generating transmission key: %v\n", err)
		return nil, err
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
		return nil, err
	}

	// Finally, we set the client cookie for "token" as the JWT we just generated
	// we also set an expiry time which is the same as the token itself
	return &http.Cookie{
		Name:     "passwall_token",
		Value:    tokenString,
		Expires:  expirationTime,
		HttpOnly: true,
		// TODO : add secure flag
		// Secure:   true,
	}, nil
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
		Name:     "passwall_token",
		Value:    tokenString,
		Expires:  expirationTime,
		HttpOnly: true,
		// TODO : add secure flag
		// Secure:   true,
	}, nil
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

/* it may need us later to read access detail

type AccessDetailsDTO struct {
	UserID uint64
}

func extractTokenMetadata(r *http.Request) (*AccessDetailsDTO, error) {
	token, err := verifyToken(r)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		userID, err := strconv.ParseUint(fmt.Sprintf("%.f", claims["user_id"]), 10, 64)
		if err != nil {
			return nil, err
		}
		return &AccessDetailsDTO{
			UserID: userID,
		}, nil
	}
	return nil, err
}*/
