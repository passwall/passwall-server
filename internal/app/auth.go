package app

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/passwall/passwall-server/model"
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

// CreateCache
func CreateCache(defaultExpiration, cleanupInterval time.Duration) *cache.Cache {
	return cache.New(defaultExpiration, cleanupInterval)
}

//CreateToken ...
func CreateToken(user *model.User) (*model.TokenDetailsDTO, error) {

	var err error
	accessSecret := viper.GetString("server.secret")
	td := &model.TokenDetailsDTO{}

	accessTokenExpireDuration := resolveTokenExpireDuration(viper.GetString("server.accessTokenExpireDuration"))
	refreshTokenExpireDuration := resolveTokenExpireDuration(viper.GetString("server.refreshTokenExpireDuration"))

	td.AtExpiresTime = time.Now().Add(accessTokenExpireDuration)
	td.RtExpiresTime = time.Now().Add(refreshTokenExpireDuration)

	td.AtUUID = uuid.NewV4()
	td.RtUUID = uuid.NewV4()

	//create access token
	atClaims := jwt.MapClaims{}

	atClaims["authorized"] = false
	if user.Role == "Admin" {
		atClaims["authorized"] = true
	}

	atClaims["user_uuid"] = user.UUID.String()
	atClaims["exp"] = td.AtExpiresTime.Unix()
	atClaims["uuid"] = td.AtUUID.String()
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	td.AccessToken, err = at.SignedString([]byte(accessSecret))
	if err != nil {
		return nil, err
	}

	//create refresh token
	rtClaims := jwt.MapClaims{}
	rtClaims["user_uuid"] = user.UUID.String()
	rtClaims["exp"] = td.RtExpiresTime.Unix()
	rtClaims["uuid"] = td.RtUUID.String()

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	td.RefreshToken, err = rt.SignedString([]byte(accessSecret))
	if err != nil {
		return nil, err
	}

	generatedPass, err := GenerateSecureKey(viper.GetInt("server.generatedPasswordLength"))
	if err != nil {
		return nil, err
	}
	td.TransmissionKey = generatedPass

	return td, nil
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
