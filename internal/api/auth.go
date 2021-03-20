package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/matcornic/hermes"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"
)

var (
	userLoginErr   = "User email or master password is wrong."
	userVerifyErr  = "Please verify your email first."
	invalidUser    = "Invalid user"
	invalidToken   = "Token is expired or not valid!"
	noToken        = "Token could not found! "
	tokenCreateErr = "Token could not be created"
	signupSuccess  = "User created successfully"
)

// Signup ...
func Signup(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// 1. Decode request body to userDTO object
		userSignup := new(model.UserSignup)
		decoderr := json.NewDecoder(r.Body)
		if err := decoderr.Decode(&userSignup); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// 2. Run validator according to model.UserDTO validator tags
		err := app.PayloadValidator(userSignup)
		if err != nil {
			errs := GetErrors(err.(validator.ValidationErrors))
			RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
			return
		}

		// 3. Check and verify the recaptcha response token only in production.
		if viper.GetString("server.env") == "prod" {
			if err := CheckRecaptcha(userSignup.Recaptcha); err != nil {
				RespondWithError(w, http.StatusUnauthorized, err.Error())
				return
			}
		}

		// 4. Check if user exist in database
		userDTO := model.ConvertUserDTO(userSignup)
		_, err = s.Users().FindByEmail(userDTO.Email)
		if err == nil {
			RespondWithError(w, http.StatusBadRequest, "User couldn't created!")
			return
		}

		// 5. Create new user
		createdUser, err := app.CreateUser(s, userDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 6. Send email to admin about new user subscription
		notifyAdminEmail(userDTO)

		// 7. Send confirmation email to new user
		sendConfirmationEmail(userDTO, createdUser.ConfirmationCode)

		// Return success message
		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: signupSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// CheckRecaptcha ...
func CheckRecaptcha(gCaptchaValue string) error {

	secret := viper.GetString("server.recaptcha")

	type SiteVerifyResponse struct {
		Success     bool      `json:"success"`
		Score       float64   `json:"score"`
		Action      string    `json:"action"`
		ChallengeTS time.Time `json:"challenge_ts"`
		Hostname    string    `json:"hostname"`
		ErrorCodes  []string  `json:"error-codes"`
	}

	const siteVerifyURL = "https://www.google.com/recaptcha/api/siteverify"

	// Create new request
	req, err := http.NewRequest(http.MethodPost, siteVerifyURL, nil)
	if err != nil {
		return err
	}

	// Add necessary request
	q := req.URL.Query()
	q.Add("secret", secret)
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

	// Check response score.
	if body.Score < 0.5 {
		return errors.New("Lower received score than expected")
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
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		http.Redirect(w, r, "https://signup.passwall.io/confirmed", http.StatusSeeOther)
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

		// Run validator according to model.AuthLoginDTO validator tags
		err := app.PayloadValidator(loginDTO)
		if err != nil {
			errs := GetErrors(err.(validator.ValidationErrors))
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
		if user.EmailVerifiedAt.IsZero() {
			RespondWithError(w, http.StatusForbidden, userVerifyErr)
			return
		}

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

func sendConfirmationEmail(user *model.UserDTO, confirmationCode string) {
	h := hermes.Hermes{
		Product: hermes.Product{
			Name:      "Passwall",
			Link:      "https://passwall.io",
			Logo:      "https://signup.passwall.io//images/passwall-logo.png",
			Copyright: "Copyright © 2021 Passwall. All rights reserved.",
		},
	}

	email := hermes.Email{
		Body: hermes.Body{
			Name: user.Name,
			Intros: []string{
				"Welcome to Passwall! We're very excited to have you on board.",
			},
			Actions: []hermes.Action{
				{
					Instructions: "To get started with Passwall, please click here:",
					Button: hermes.Button{
						Color: "#22BC66",
						Text:  "Confirm your account",
						Link:  viper.GetString("server.domain") + "/auth/confirm/" + user.Email + "/" + confirmationCode,
					},
				},
			},
			Outros: []string{
				"Need help, or have questions? Just reply to this email, we'd love to help.",
			},
		},
	}

	// Generate an HTML email with the provided contents (for modern clients)
	emailBody, err := h.GenerateHTML(email)
	if err != nil {
		log.Println(err)
	}

	app.SendMail(
		user.Name,
		user.Email,
		"Passwall Email Confirmation",
		emailBody)
}

func notifyAdminEmail(user *model.UserDTO) {
	subject := "PassWall New User Subscription"
	body := "PassWall has new a user. User details:\n\n"
	body += "Name: " + user.Name + "\n"
	body += "Email: " + user.Email + "\n"
	app.SendMail(
		viper.GetString("email.fromName"),
		viper.GetString("email.fromEmail"),
		subject,
		body)
}
