package api

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/patrickmn/go-cache"
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
	verifySuccess  = "Email verified successfully"
	codeSuccess    = "Code created successfully"
)

// Create email verification code
func CreateCode(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Decode json to email
		var signup model.AuthEmail
		if err := json.NewDecoder(r.Body).Decode(&signup); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}

		// 2. Check if user exist in database
		_, err := s.Users().FindByEmail(signup.Email)
		if err == nil {
			log.Printf("email %s already exist in database\n", signup.Email)
			RespondWithError(w, http.StatusBadRequest, "User couldn't created!")
			return
		}

		// 2. Generate a random code
		rand.Seed(time.Now().Unix())
		min := 100000
		max := 999999
		code := strconv.Itoa(rand.Intn(max-min+1) + min)

		log.Printf("verification code %s generated for email %s\n", code, signup.Email)

		// 3. Save code in cache
		c.Set(signup.Email, code, cache.DefaultExpiration)

		// 4. Send verification email to user
		subject := "Passwall Email Verification"
		body := "Passwall verification code: " + code
		if err = app.SendMail("Passwall Verification Code", signup.Email, subject, body); err != nil {
			log.Printf("can't send email to %s error: %v\n", signup.Email, err)
			RespondWithError(w, http.StatusBadRequest, "Couldn't send email")
			return
		}

		// Return success message
		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: codeSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// Verify Email
func VerifyCode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userCode := mux.Vars(r)["code"]
		email := r.FormValue("email")

		code, ok := c.Get(email)
		if !ok {
			RespondWithError(w, http.StatusBadRequest, "Code couldn't found!")
			return
		}

		confirmationCode, ok := code.(string)
		if !ok {
			RespondWithError(w, http.StatusInternalServerError, "Server error!")
			return
		}

		if userCode != confirmationCode {
			RespondWithError(w, http.StatusBadRequest, "Code doesn't match!")
			return
		}

		c.Set(email, "verified", cache.DefaultExpiration)

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: verifySuccess,
		}

		RespondWithJSON(w, http.StatusOK, response)
	}
}

// Signup ...
func Signup(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Decode request body to userDTO object
		userSignup := new(model.UserSignup)
		decoderr := json.NewDecoder(r.Body)
		if err := decoderr.Decode(&userSignup); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		defer r.Body.Close()

		// 2. Check if email is verified
		if err := isMailVerified(userSignup.Email); err != nil {
			log.Println(err)
			RespondWithError(w, http.StatusUnauthorized, "Email is not verified")
			return
		}

		// 2. Run validator according to model.UserDTO validator tags
		err := app.PayloadValidator(userSignup)
		if err != nil {
			errs := GetErrors(err.(validator.ValidationErrors))
			RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
			return
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
		notifyAdminEmail(createdUser)

		// Return success message
		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: signupSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// Signin ...
func Signin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var loginDTO model.AuthLoginDTO
		subscriptionType := "pro"

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

		// Check if user has an active subscription
		subscription, err := s.Subscriptions().FindByEmail(user.Email)
		if err != nil {
			subscriptionType = "free"
		}

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
			Type:                subscriptionType,
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

func notifyAdminEmail(user *model.User) {
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

func isMailVerified(email string) error {
	cachedEmail, found := c.Get(email)
	if !found {
		err := fmt.Errorf("can't find email %q in cache", email)
		return err
	}

	verified, ok := cachedEmail.(string)
	if !ok {
		err := fmt.Errorf("can't convert cached email data %v to string", verified)
		return err
	}

	if verified != "verified" {
		err := fmt.Errorf("cached email value %s doesn't match for email %s", verified, email)
		return err
	}

	return nil
}
