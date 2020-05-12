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

// Signin ...
func Signin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		validate := validator.New()

		var loginDTO model.AuthLoginDTO

		// get loginDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&loginDTO); err != nil {
			RespondWithError(w, http.StatusUnprocessableEntity, "Invalid json provided")
			return
		}
		defer r.Body.Close()

		// validate struct
		validateError := validate.Struct(loginDTO)
		if validateError != nil {
			errs := GetErrors(validateError.(validator.ValidationErrors))
			RespondWithErrors(w, http.StatusBadRequest, "Invalid resquest payload", errs)
			return
		}

		// check user
		if viper.GetString("server.username") != loginDTO.Username ||
			viper.GetString("server.password") != loginDTO.Password {
			RespondWithError(w, http.StatusUnauthorized, "Invalid User")
			return
		}

		//create token
		token, err := app.CreateToken()
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Token could not be created")
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
			RespondWithErrors(w, http.StatusUnprocessableEntity, "Invalid json provided", errs)
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
			RespondWithError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		//create token
		newtoken, err := app.CreateToken()
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Token could not be created")
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

	//TODO: maybe check if there is any token? If not, return early.

	_, err := app.TokenValid(token)
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Token is expired or not valid!")
		return
	}

	response := model.Response{
		Code:    http.StatusOK,
		Status:  "Success",
		Message: "Token is valid!",
	}

	RespondWithJSON(w, http.StatusOK, response)
}
