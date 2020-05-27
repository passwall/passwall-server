package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/pass-wall/passwall-server/internal/auth"
	a "github.com/pass-wall/passwall-server/internal/auth"
	"github.com/pass-wall/passwall-server/internal/common"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

var validate *validator.Validate

var (
	InvalidUser = "Invalid user"
	InvalidJson = "Invalid json provided"

	ValidToken     = "Token is valid"
	InvalidToken   = "Token is expired or not valid!"
	TokenCreateErr = "Token could not be created"
)

// Signin ...
func Signin(w http.ResponseWriter, r *http.Request) {

	validate = validator.New()

	var loginDTO a.LoginDTO

	// get loginDTO
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&loginDTO); err != nil {
		common.RespondWithError(w, http.StatusUnprocessableEntity, InvalidJson)
		return
	}
	defer r.Body.Close()

	// validate struct
	validateError := validate.Struct(loginDTO)
	if validateError != nil {
		errs := common.GetErrors(validateError.(validator.ValidationErrors))
		common.RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
		return
	}

	// check user
	if viper.GetString("server.username") != loginDTO.Username ||
		viper.GetString("server.password") != loginDTO.Password {
		common.RespondWithError(w, http.StatusUnauthorized, InvalidUser)
		return
	}

	//create token
	token, err := a.CreateToken()
	if err != nil {
		common.RespondWithError(w, http.StatusInternalServerError, TokenCreateErr)
		return
	}

	tokens := map[string]string{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
	}

	common.RespondWithJSON(w, 200, tokens)
}

// RefreshToken ...
func RefreshToken(w http.ResponseWriter, r *http.Request) {

	// Get token from authorization header
	mapToken := map[string]string{}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&mapToken); err != nil {
		errs := []string{"REFRESH_TOKEN_ERROR"}
		common.RespondWithErrors(w, http.StatusUnprocessableEntity, InvalidJson, errs)
		return
	}
	defer r.Body.Close()

	token, err := a.RefreshToken(mapToken["refresh_token"])
	if err != nil {
		errs := []string{"REFRESH_TOKEN_ERROR"}
		common.RespondWithErrors(w, http.StatusUnauthorized, err.Error(), errs)
		return
	}

	tokens := map[string]string{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
	}

	common.RespondWithJSON(w, 200, tokens)

}

// CheckToken ...
func CheckToken(w http.ResponseWriter, r *http.Request) {

	err := auth.TokenValid(r)
	if err != nil {
		common.RespondWithError(w, http.StatusUnauthorized, InvalidToken)
		return
	}

	response := model.Response{Code: http.StatusOK, Status: Success, Message: ValidToken}
	common.RespondWithJSON(w, http.StatusOK, response)
}
