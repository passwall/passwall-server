package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pass-wall/passwall-server/model"

	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
)

var (
	ExpiredToken = fmt.Errorf("Token expired or invalid")
	Unauthorized = fmt.Errorf("Unauthorized")
)

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
	atClaims["user_id"] = user.ID
	atClaims["exp"] = td.AtExpiresTime.Unix()
	atClaims["uuid"] = td.AtUUID.String()
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	td.AccessToken, err = at.SignedString([]byte(accessSecret))
	if err != nil {
		return nil, err
	}

	//create refresh token
	rtClaims := jwt.MapClaims{}
	rtClaims["user_id"] = user.ID
	rtClaims["exp"] = td.RtExpiresTime.Unix()
	rtClaims["uuid"] = td.RtUUID.String()

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	td.RefreshToken, err = rt.SignedString([]byte(accessSecret))
	if err != nil {
		return nil, err
	}

	td.SecureKey, err = Password()
	fmt.Println(err)

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
		return nil, Unauthorized
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
		return token, ExpiredToken
	}
	return token, nil
}

func resolveTokenExpireDuration(config string) time.Duration {
	duration, _ := strconv.ParseInt(config[0:len(config)-1], 10, 64)
	timeFormat := config[len(config)-1:]

	switch timeFormat {
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
