package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/pass-wall/passwall-server/internal/auth"
	a "github.com/pass-wall/passwall-server/internal/auth"
	"github.com/pass-wall/passwall-server/internal/common"
	"github.com/spf13/viper"
)

var validate *validator.Validate

// Signin ...
func Signin(w http.ResponseWriter, r *http.Request) {

	validate = validator.New()

	var loginDTO a.LoginDTO

	// get loginDTO
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&loginDTO); err != nil {
		common.RespondWithError(w, http.StatusUnprocessableEntity, "Invalid json provided")
		return
	}
	defer r.Body.Close()

	// validate struct
	validateError := validate.Struct(loginDTO)
	if validateError != nil {
		errs := common.GetErrors(validateError.(validator.ValidationErrors))
		common.RespondWithErrors(w, http.StatusBadRequest, "Invalid resquest payload", errs)
		return
	}

	// check user
	if viper.GetString("server.username") != loginDTO.Username ||
		viper.GetString("server.password") != loginDTO.Password {
		common.RespondWithError(w, http.StatusUnauthorized, "Invalid User")
		return
	}

	//create token
	token, err := a.CreateToken()
	if err != nil {
		common.RespondWithError(w, http.StatusInternalServerError, "Token could not be created")
		return
	}

	tokens := map[string]string{
		"AccessToken":  token.AccessToken,
		"RefreshToken": token.RefreshToken,
	}

	common.RespondWithJSON(w, 200, tokens)
}

// RefreshToken ...
func RefreshToken(w http.ResponseWriter, r *http.Request) {

	// Get token from authorization header
	extractedToken := auth.ExtractToken(r)

	token, err := a.RefreshToken(extractedToken)
	if err != nil {
		common.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	tokens := map[string]string{
		"AccessToken":  token.AccessToken,
		"RefreshToken": token.RefreshToken,
	}

	common.RespondWithJSON(w, 200, tokens)

}

// Logout ...
func Logout(w http.ResponseWriter, r *http.Request) {
	//TODO
}
