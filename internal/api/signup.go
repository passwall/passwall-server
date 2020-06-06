package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
)

const (
	SignupSuccess = "User created successfully"
)

// Signup ...
func Signup(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userDTO := new(model.UserDTO)

		// 1. Decode request body to userDTO object
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

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: SignupSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
