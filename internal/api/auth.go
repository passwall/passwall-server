package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-playground/validator/v10"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

var (
	InvalidUser    = "Invalid user"
	ValidToken     = "Token is valid"
	InvalidToken   = "Token is expired or not valid!"
	NoToken        = "Token could not found! "
	TokenCreateErr = "Token could not be created"
)

// Signin ...
func Signin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		validate := validator.New()

		var loginDTO model.AuthLoginDTO

		// get loginDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&loginDTO); err != nil {
			RespondWithError(w, http.StatusUnprocessableEntity, InvalidJSON)
			return
		}
		defer r.Body.Close()

		// validate struct
		validateError := validate.Struct(loginDTO)
		if validateError != nil {
			errs := GetErrors(validateError.(validator.ValidationErrors))
			RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
			return
		}

		// check user
		if viper.GetString("server.username") != loginDTO.Username ||
			viper.GetString("server.password") != loginDTO.Password {
			RespondWithError(w, http.StatusUnauthorized, InvalidUser)
			return
		}

		//create token
		token, err := app.CreateToken()
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, TokenCreateErr)
			return
		}

		//delete tokens from db
		s.Tokens().Delete(1)

		//create tokens on db
		s.Tokens().Save(1, token.AtUUID, token.AccessToken, token.AtExpiresTime)
		s.Tokens().Save(1, token.RtUUID, token.RefreshToken, token.RtExpiresTime)

		tokens := map[string]string{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
		}

		RespondWithJSON(w, 200, tokens)
	}
}

// RefreshToken ...
func RefreshToken(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get token from authorization header
		mapToken := map[string]string{}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&mapToken); err != nil {
			errs := []string{"REFRESH_TOKEN_ERROR"}
			RespondWithErrors(w, http.StatusUnprocessableEntity, InvalidJSON, errs)
			return
		}
		defer r.Body.Close()

		token, err := app.TokenValid(mapToken["refresh_token"])

		if err != nil {
			if token != nil {
				claims, _ := token.Claims.(jwt.MapClaims)
				userid, _ := strconv.Atoi(fmt.Sprintf("%.f", claims["user_id"]))
				s.Tokens().Delete(userid)
			}
			RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		claims, _ := token.Claims.(jwt.MapClaims)
		uuid, _ := claims["uuid"].(string)

		//check from db
		if !s.Tokens().Any(uuid) {
			userid, _ := strconv.Atoi(fmt.Sprintf("%.f", claims["user_id"]))
			s.Tokens().Delete(userid)
			RespondWithError(w, http.StatusUnauthorized, InvalidToken)
			return
		}

		//create token
		newtoken, err := app.CreateToken()
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, TokenCreateErr)
			return
		}

		//delete tokens from db
		s.Tokens().Delete(1)

		//create tokens on db
		s.Tokens().Save(1, newtoken.AtUUID, newtoken.AccessToken, newtoken.AtExpiresTime)
		s.Tokens().Save(1, newtoken.RtUUID, newtoken.RefreshToken, newtoken.RtExpiresTime)

		tokens := map[string]string{
			"access_token":  newtoken.AccessToken,
			"refresh_token": newtoken.RefreshToken,
		}

		RespondWithJSON(w, 200, tokens)
	}
}

// CheckToken ...
func CheckToken(w http.ResponseWriter, r *http.Request) {
	var token string
	bearerToken := r.Header.Get("Authorization")
	strArr := strings.Split(bearerToken, " ")
	if len(strArr) == 2 {
		token = strArr[1]
	}

	if token != "" {
		RespondWithError(w, http.StatusUnauthorized, NoToken)
		return
	}


	_, err := app.TokenValid(token)
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, InvalidToken)
		return
	}

	response := model.Response{
		Code:    http.StatusOK,
		Status:  Success,
		Message: ValidToken,
	}

	RespondWithJSON(w, http.StatusOK, response)
}
