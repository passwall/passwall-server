package auth

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/spf13/viper"
)

//CreateToken ...
func CreateToken() (*TokenDetailsDTO, error) {

	var err error
	accessSecret := viper.GetString("server.secret")
	td := &TokenDetailsDTO{}

	accessTokenExpireDuration := resolveTokenExpireDuration(viper.GetString("server.accessTokenExpireDuration"))
	refreshTokenExpireDuration := resolveTokenExpireDuration(viper.GetString("server.refreshTokenExpireDuration"))

	td.AtExpires = time.Now().Add(accessTokenExpireDuration).Unix()
	td.RtExpires = time.Now().Add(refreshTokenExpireDuration).Unix()

	//create access token
	atClaims := jwt.MapClaims{}
	atClaims["authorized"] = true
	atClaims["user_id"] = 1
	atClaims["exp"] = td.AtExpires
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	td.AccessToken, err = at.SignedString([]byte(accessSecret))
	if err != nil {
		return nil, err
	}

	//create refresh token
	rtClaims := jwt.MapClaims{}
	rtClaims["user_id"] = 1
	rtClaims["exp"] = td.RtExpires
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	td.RefreshToken, err = rt.SignedString([]byte(accessSecret))
	if err != nil {
		return nil, err
	}
	return td, nil
}

//RefreshToken ...
func RefreshToken(refreshToken string) (*TokenDetailsDTO, error) {

	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		//Make sure that the token method conform to "SigningMethodHMAC"
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(viper.GetString("server.secret")), nil
	})

	//if there is an error, the token must have expired
	if err != nil {
		return nil, fmt.Errorf("Refresh token expired or invalid")
	}

	//is token valid?
	if _, ok := token.Claims.(jwt.Claims); !ok && !token.Valid {
		return nil, fmt.Errorf("Unauthorized")
	}

	//Since token is valid, get the user_id:
	_, ok := token.Claims.(jwt.MapClaims) //the token claims should conform to MapClaims, if you want to get claims _ to claims
	if ok && token.Valid {

		/* if we need to read claims values , we can use the code block
		userID, err := strconv.ParseUint(fmt.Sprintf("%.f", claims["user_id"]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Error occurred")
		}
		fmt.Println(userID)*/

		ts, createErr := CreateToken()
		if createErr != nil {
			return nil, createErr
		}
		return ts, nil
	}

	return nil, fmt.Errorf("Refresh token expired or invalid")

}

//TokenValid ...
func TokenValid(r *http.Request) error {

	token, err := verifyToken(r)
	if err != nil {
		return err
	}
	if _, ok := token.Claims.(jwt.Claims); !ok && !token.Valid {
		return err
	}
	return nil
}

//verifyToken verify token
func verifyToken(r *http.Request) (*jwt.Token, error) {

	tokenString := extractToken(r)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		//Make sure that the token method conform to "SigningMethodHMAC"
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(viper.GetString("server.secret")), nil
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}

// ExtractToken ...
func extractToken(r *http.Request) string {

	bearerToken := r.Header.Get("Authorization")
	strArr := strings.Split(bearerToken, " ")
	if len(strArr) == 2 {
		return strArr[1]
	}
	return ""
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
	UserId uint64
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
			UserId: userID,
		}, nil
	}
	return nil, err
}*/
