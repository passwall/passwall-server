package api

import (
	"encoding/json"
	"errors"
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
	userLoginErr   = "User email or master password is wrong."
	userVerifyErr  = "Please verify your email first."
	invalidUser    = "Invalid user"
	validToken     = "Token is valid"
	invalidToken   = "Token is expired or not valid!"
	noToken        = "Token could not found! "
	tokenCreateErr = "Token could not be created"
	signupSuccess  = "User created successfully"
	verifySuccess  = "Email verified successfully"
)

// Signup ...
func Signup(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 0. Decode request body to userDTO object
		userSignup := new(model.UserSignup)
		decoderr := json.NewDecoder(r.Body)
		if err := decoderr.Decode(&userSignup); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// 1. Run validator according to model.UserDTO validator tags
		validate := validator.New()
		validateError := validate.Struct(userSignup)
		if validateError != nil {
			errs := GetErrors(validateError.(validator.ValidationErrors))
			RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
			return
		}

		// 2. Check and verify the recaptcha response token.
		if err := CheckRecaptcha(userSignup.Recaptcha); err != nil {
			RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// 3. Check if user exist in database
		userDTO := model.ConvertUserDTO(userSignup)
		_, err := s.Users().FindByEmail(userDTO.Email)
		if err == nil {
			RespondWithError(w, http.StatusBadRequest, "User couldn't created!")
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
		app.SendMail(
			viper.GetString("email.fromName"),
			viper.GetString("email.fromEmail"),
			subject,
			body)

		// 9. Send confirmation email to new user
		confirmationSubject := "Passwall Email Confirmation"
		confirmationBody := "Last step for use Passwall\n\n"
		confirmationBody += "Confirmation link: " + viper.GetString("server.domain")
		confirmationBody += "/auth/confirm/" + userDTO.Email + "/" + confirmationCode
		app.SendMail(
			userDTO.Name,
			userDTO.Email,
			confirmationSubject,
			confirmationBody)

		// Return success message
		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: signupSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

func CheckRecaptcha(gCaptchaValue string) error {

	type SiteVerifyResponse struct {
		Success     bool     `json:"success"`
		ChallengeTS string   `json:"challenge_ts"`
		Hostname    string   `json:"hostname"`
		ErrorCodes  []string `json:"error-codes"`
	}

	const siteVerifyURL = "https://www.google.com/recaptcha/api/siteverify"

	// Create new request
	req, err := http.NewRequest(http.MethodPost, siteVerifyURL, nil)
	if err != nil {
		return err
	}

	// Add necessary request
	q := req.URL.Query()
	q.Add("secret", viper.GetString("server.recaptcha"))
	q.Add("response", gCaptchaValue)
	req.URL.RawQuery = q.Encode()

	// Make request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Decode response.
	var body SiteVerifyResponse
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}

	// Check recaptcha verification success.
	if !body.Success {
		return errors.New("Unsuccessful recaptcha verify request")
	}

	return nil
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
			Status:  verifySuccess,
			Message: signupSuccess,
		}

		RespondWithJSON(w, http.StatusOK, response)
		// RespondWithHTML(w, http.StatusOK, response)
	}
}

// Signin ...
func Signin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var loginDTO model.AuthLoginDTO

		// get loginDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&loginDTO); err != nil {
			RespondWithError(w, http.StatusUnprocessableEntity, InvalidJSON)
			return
		}
		defer r.Body.Close()

		// validate struct
		validate := validator.New()
		validateError := validate.Struct(loginDTO)
		if validateError != nil {
			errs := GetErrors(validateError.(validator.ValidationErrors))
			RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
			return
		}

		// Check if user exist in database and credentials are true
		user, err := s.Users().FindByCredentials(loginDTO.Email, loginDTO.MasterPassword)
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, userLoginErr)
			return
		}

		// Check if users email is verified
		// if user.EmailVerifiedAt.IsZero() {
		// 	RespondWithError(w, http.StatusForbidden, userVerifyErr)
		// 	return
		// }

		// Check if user has an active subscription
		subscription, _ := s.Subscriptions().FindByEmail(user.Email)

		//create token
		token, err := app.CreateToken(user)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, tokenCreateErr)
			return
		}

		//delete tokens from db
		s.Tokens().Delete(int(user.ID))

		//create tokens on db
		s.Tokens().Save(int(user.ID), token.AtUUID, token.AccessToken, token.AtExpiresTime, token.TransmissionKey)
		s.Tokens().Save(int(user.ID), token.RtUUID, token.RefreshToken, token.RtExpiresTime, "")

		authLoginResponse := model.AuthLoginResponse{
			AccessToken:         token.AccessToken,
			RefreshToken:        token.RefreshToken,
			TransmissionKey:     token.TransmissionKey,
			UserDTO:             model.ToUserDTO(user),
			SubscriptionAuthDTO: model.ToSubscriptionAuthDTO(subscription),
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
			RespondWithError(w, http.StatusUnauthorized, invalidToken)
			return
		}

		// Get user info
		userid := claims["user_id"].(float64)
		user, err := s.Users().FindByID(uint(userid))
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, invalidUser)
			return
		}

		//create token
		newtoken, err := app.CreateToken(user)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, tokenCreateErr)
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
			RespondWithError(w, http.StatusUnauthorized, noToken)
			return
		}

		token, err := app.TokenValid(tokenStr)
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, invalidToken)
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		userID := claims["user_id"].(float64)

		// Check if user exist in database and credentials are true
		user, err := s.Users().FindByID(uint(userID))
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, invalidUser)
			return
		}

		response := model.ToUserDTOTable(*user)

		RespondWithJSON(w, http.StatusOK, response)
	}
}
