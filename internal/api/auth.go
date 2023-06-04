package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v4"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/cookie"
	"github.com/passwall/passwall-server/pkg/logger"
	"github.com/passwall/passwall-server/pkg/token"
)

var (
	userLoginErr         = "User email or master password is wrong."
	invalidUser          = "Invalid user"
	invalidToken         = "Token is expired or not valid!"
	noToken              = "Token could not found! "
	tokenCreateErr       = "Token could not be created"
	signupSuccess        = "User created successfully"
	signoutSuccess       = "User signed out successfully"
	codeSuccess          = "Code created successfully"
	subscriptionTypePro  = "pro"
	subscriptionTypeFree = "free"
)

// Signin ...
func Signin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var loginDTO model.AuthLoginDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&loginDTO); err != nil {
			RespondWithError(w, http.StatusUnprocessableEntity, InvalidJSON)
			return
		}
		defer func() {
			if err := r.Body.Close(); err != nil {
				logger.Errorf("Failed to close body error: %v", err)
			}
		}()

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

		sType := subscriptionTypeFree
		if isPro(user.UUID) {
			sType = subscriptionTypePro
		}

		// token is necessary for Passwall Extension
		token, err := app.CreateToken(user)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, tokenCreateErr)
			return
		}

		//delete tokens from db
		s.Tokens().DeleteByUUID(token.AtUUID.String())
		s.Tokens().DeleteByUUID(token.RtUUID.String())

		//create tokens on db
		s.Tokens().Create(int(user.ID), token.AtUUID, token.AccessToken, token.AtExpiresTime)
		s.Tokens().Create(int(user.ID), token.RtUUID, token.RefreshToken, token.RtExpiresTime)

		authLoginResponse := model.AuthLoginResponse{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			Type:         sType,
			UserDTO:      model.ToUserDTO(user),
		}

		// cookie is necessary for Passwall Desktop
		newCookie := cookie.Create(constants.CookieName, token.AccessToken, token.AtExpiresTime)

		RespondWithCookie(w, 200, newCookie, authLoginResponse)
	}
}

func Signout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deletedCookie := cookie.Delete(constants.CookieName)

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: signoutSuccess,
		}
		RespondWithCookie(w, http.StatusOK, deletedCookie, response)
	}
}

// RefreshToken ...
func RefreshToken(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		refreshToken := token.ExtractRefreshToken(r)

		token, err := app.TokenValid(refreshToken)
		if err != nil {
			if token != nil {
				claims := token.Claims.(jwt.MapClaims)
				userUUID := claims["user_uuid"].(string)
				s.Tokens().DeleteByUUID(userUUID)
			}
			RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		uuid := claims["uuid"].(string)

		// Get token details from db by User UUID
		_, err = s.Tokens().FindByUUID(uuid)
		if err != nil {
			userUUID := claims["user_uuid"].(string)
			s.Tokens().DeleteByUUID(userUUID)
			RespondWithError(w, http.StatusUnauthorized, invalidToken)
			return
		}

		// Get user info
		userUUID := claims["user_uuid"].(string)
		user, err := s.Users().FindByUUID(userUUID)
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
		s.Tokens().DeleteByUUID(userUUID)

		//create tokens on db
		s.Tokens().Create(int(user.ID), newtoken.AtUUID, newtoken.AccessToken, newtoken.AtExpiresTime)
		s.Tokens().Create(int(user.ID), newtoken.RtUUID, newtoken.RefreshToken, newtoken.RtExpiresTime)

		authLoginResponse := model.AuthLoginResponse{
			AccessToken:  newtoken.AccessToken,
			RefreshToken: newtoken.RefreshToken,
			UserDTO:      model.ToUserDTO(user),
		}

		// cookie is necessary for Passwall Desktop
		newCookie := cookie.Create(constants.CookieName, newtoken.AccessToken, newtoken.AtExpiresTime)

		RespondWithCookie(w, 200, newCookie, authLoginResponse)
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
		userUUID := claims["user_uuid"].(string)

		// Check if user exist in database and credentials are true
		user, err := s.Users().FindByUUID(userUUID)
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

func isPro(uuid uuid.UUID) bool {
	url := "https://api.revenuecat.com/v1/subscribers/" + uuid.String()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Errorf("Error creating request: %v", err)
		return false
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("X-Platform", "ios")
	req.Header.Add("authorization", "Bearer "+viper.GetString("server.apiKey"))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Errorf("Error sending request: %v", err)
		return false
	}

	defer func() {
		if res.Body.Close() != nil {
			logger.Errorf("Error closing response body: %v", err)
		}
	}()

	type Customer struct {
		Subscriber struct {
			Entitlements map[string]any `json:"entitlements"`
		} `json:"subscriber"`
	}
	var customer Customer
	if err = json.NewDecoder(res.Body).Decode(&customer); err != nil {
		logger.Errorf("Error decoding response body: %v", err)
		return false
	}

	_, ok := customer.Subscriber.Entitlements["Pro"]
	return ok
}
