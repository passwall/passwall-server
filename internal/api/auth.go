package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"
)

var (
	InvalidUser    = "Invalid user"
	ValidToken     = "Token is valid"
	InvalidToken   = "Token is expired or not valid!"
	NoToken        = "Token could not found! "
	TokenCreateErr = "Token could not be created"
	SignupSuccess  = "User created successfully"
	VerifySuccess  = "Email verified succesfully"
)

// Signup ...
func Signup(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// 0. API Key Check
		keys, ok := r.URL.Query()["api_key"]

		if !ok || len(keys[0]) < 1 {
			RespondWithError(w, http.StatusBadRequest, "API Key is missing")
			return
		}

		if keys[0] != viper.GetString("server.apiKey") {
			RespondWithError(w, http.StatusUnauthorized, "API Key is wrong")
			return
		}

		// 1. Decode request body to userDTO object
		userDTO := new(model.UserDTO)
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&userDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// 2. Run validator according to model.UserDTO validator tags
		validate := validator.New()
		validateError := validate.Struct(userDTO)
		if validateError != nil {
			errs := GetErrors(validateError.(validator.ValidationErrors))
			RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
			return
		}

		// 3. Check if user exist in database
		_, err := s.Users().FindByEmail(userDTO.Email)
		if err == nil {
			errs := []string{"This email is already used!"}
			message := "User couldn't created!"
			RespondWithErrors(w, http.StatusBadRequest, message, errs)
			return
		}

		// 4. Create new user
		createdUser, err := app.CreateUser(s, userDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		confirmationCode := app.RandomMD5Hash()
		createdUser.ConfirmationCode = confirmationCode

		// 5. Update user once to generate schema
		updatedUser, err := app.GenerateSchema(s, createdUser)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 6. Create user schema and tables
		err = s.Users().CreateSchema(updatedUser.Schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 7. Create user tables in user schema
		app.MigrateUserTables(s, updatedUser.Schema)

		// 8. Send email to admin adbout new user subscription
		subject := "PassWall New User Subscription"

		body := "PassWall has new a user. User details:\n\n"
		body += "Name: " + userDTO.Name + "\n"
		body += "Email: " + userDTO.Email + "\n"

		go app.SendMail([]string{viper.GetString("email.admin")}, subject, body)

		// 9. Send confirmation email to new user
		confirmationBody := "Last step for use Passwall\n\n"
		confirmationBody += "Confirmation link: " + viper.GetString("server.domain")
		confirmationBody += "/auth/confirm/" + userDTO.Email + "/" + confirmationCode

		go app.SendMail([]string{userDTO.Email}, "Passwall Email Confirmation", confirmationBody)

		// Return success message
		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: SignupSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// Confirm ...
func Confirm(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := mux.Vars(r)["email"]
		code := mux.Vars(r)["code"]
		usr, err := s.Users().FindByEmail(email)
		if err != nil {
			errs := []string{"Email not found!", "Raw error: " + err.Error()}
			message := "Email couldn't confirm!"
			RespondWithErrors(w, http.StatusBadRequest, message, errs)
			return
		} else if !usr.EmailVerifiedAt.IsZero() {
			errs := []string{"Email is already verified!"}
			message := "Email couldn't confirm!"
			RespondWithErrors(w, http.StatusBadRequest, message, errs)
			return
		} else if code != usr.ConfirmationCode {
			errs := []string{"Confirmation code is wrong!"}
			message := "Email couldn't confirm!"
			RespondWithErrors(w, http.StatusBadRequest, message, errs)
			return
		}

		userDTO := model.ToUserDTO(usr)
		userDTO.MasterPassword = "" // Fix for not to update password
		userDTO.EmailVerifiedAt = time.Now()

		_, err = app.UpdateUser(s, usr, userDTO, false)
		if err != nil {
			errs := []string{"User can't updated!", "Raw error: " + err.Error()}
			message := "Email couldn't confirm!"
			RespondWithErrors(w, http.StatusBadRequest, message, errs)
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  VerifySuccess,
			Message: SignupSuccess,
		}

		RespondWithJSON(w, http.StatusOK, response)
		// RespondWithHTML(w, http.StatusOK, response)
	}
}

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
			RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Check if users email is verified
		if user.EmailVerifiedAt.IsZero() {
			RespondWithError(w, http.StatusUnauthorized, "Email is not verified!")
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
		s.Tokens().Save(int(user.ID), token.AtUUID, token.AccessToken, token.AtExpiresTime, token.TransmissionKey)
		s.Tokens().Save(int(user.ID), token.RtUUID, token.RefreshToken, token.RtExpiresTime, "")

		authLoginResponse := model.AuthLoginResponse{
			AccessToken:     token.AccessToken,
			RefreshToken:    token.RefreshToken,
			TransmissionKey: token.TransmissionKey,
			UserDTO:         model.ToUserDTO(user),
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
		_, tokenExist := s.Tokens().Any(uuid)
		if !tokenExist {
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
		s.Tokens().Save(int(userid), newtoken.AtUUID, newtoken.AccessToken, newtoken.AtExpiresTime, newtoken.TransmissionKey)
		s.Tokens().Save(int(userid), newtoken.RtUUID, newtoken.RefreshToken, newtoken.RtExpiresTime, "")

		authLoginResponse := model.AuthLoginResponse{
			AccessToken:     newtoken.AccessToken,
			RefreshToken:    newtoken.RefreshToken,
			TransmissionKey: newtoken.TransmissionKey,
			UserDTO:         model.ToUserDTO(user),
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
