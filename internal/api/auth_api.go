package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// Signin ...
func Signin(w http.ResponseWriter, r *http.Request) {

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

	tokens := map[string]string{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
	}

	RespondWithJSON(w, 200, tokens)
}

// RefreshToken ...
func RefreshToken(w http.ResponseWriter, r *http.Request) {

	// Get token from authorization header
	mapToken := map[string]string{}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&mapToken); err != nil {
		errs := []string{"REFRESH_TOKEN_ERROR"}
		RespondWithErrors(w, http.StatusUnprocessableEntity, "Invalid json provided", errs)
		return
	}
	defer r.Body.Close()

	token, err := app.RefreshToken(mapToken["refresh_token"])
	if err != nil {
		errs := []string{"REFRESH_TOKEN_ERROR"}
		RespondWithErrors(w, http.StatusUnauthorized, err.Error(), errs)
		return
	}

	tokens := map[string]string{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
	}

	RespondWithJSON(w, 200, tokens)

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

	err := app.TokenValid(token)
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
