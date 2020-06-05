package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-playground/validator/v10"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
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

		// Check if user exist in database and credentials are true
		user, err := s.Users().FindByCredentials(loginDTO.Email, loginDTO.MasterPassword)
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, InvalidUser)
			return
		}

		//create token
		token, err := app.CreateToken(user)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, TokenCreateErr)
			return
		}

		//delete tokens from db
		s.Tokens().Delete(int(user.ID))

		//create tokens on db
		s.Tokens().Save(int(user.ID), token.AtUUID, token.AccessToken, token.AtExpiresTime)
		s.Tokens().Save(int(user.ID), token.RtUUID, token.RefreshToken, token.RtExpiresTime)

		authLoginResponse := model.AuthLoginResponse{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			UserDTOTable: model.ToUserDTOTable(*user),
		}

		RespondWithJSON(w, 200, authLoginResponse)
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
				claims := token.Claims.(jwt.MapClaims)
				userid := claims["user_id"].(float64)
				s.Tokens().Delete(int(userid))
			}
			RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		uuid := claims["uuid"].(string)

		//Check from tokens db table
		if !s.Tokens().Any(uuid) {
			userid := claims["user_id"].(float64)
			s.Tokens().Delete(int(userid))
			RespondWithError(w, http.StatusUnauthorized, InvalidToken)
			return
		}

		// Get user info
		userid := claims["user_id"].(float64)
		user, err := s.Users().FindByID(uint(userid))
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, InvalidUser)
			return
		}

		//create token
		newtoken, err := app.CreateToken(user)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, TokenCreateErr)
			return
		}

		//delete tokens from db
		s.Tokens().Delete(int(userid))

		//create tokens on db
		s.Tokens().Save(int(userid), newtoken.AtUUID, newtoken.AccessToken, newtoken.AtExpiresTime)
		s.Tokens().Save(int(userid), newtoken.RtUUID, newtoken.RefreshToken, newtoken.RtExpiresTime)

		authLoginResponse := model.AuthLoginResponse{
			AccessToken:  newtoken.AccessToken,
			RefreshToken: newtoken.RefreshToken,
			UserDTOTable: model.ToUserDTOTable(*user),
		}

		RespondWithJSON(w, 200, authLoginResponse)
	}
}

// CheckToken ...
func CheckToken(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string
		bearerToken := r.Header.Get("Authorization")
		strArr := strings.Split(bearerToken, " ")
		if len(strArr) == 2 {
			tokenStr = strArr[1]
		}

		if tokenStr == "" {
			RespondWithError(w, http.StatusUnauthorized, NoToken)
			return
		}

		token, err := app.TokenValid(tokenStr)
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, InvalidToken)
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := claims["user_id"].(float64)

		// Check if user exist in database and credentials are true
		user, err := s.Users().FindByID(uint(userID))
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, InvalidUser)
			return
		}

		response := model.ToUserDTOTable(*user)

		RespondWithJSON(w, http.StatusOK, response)
	}
}
