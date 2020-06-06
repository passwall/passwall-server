package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/gorilla/mux"
)

// FindAllUsers ...
func FindAllUsers(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		users := []model.User{}

		fields := []string{"id", "created_at", "updated_at", "url", "username"}
		argsStr, argsInt := SetArgs(r, fields)

		users, err = s.Users().FindAll(argsStr, argsInt)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		usersDTOs := model.ToUserDTOs(users)

		// users = app.DecryptUserPasswords(users)
		RespondWithJSON(w, http.StatusOK, usersDTOs)
	}
}

// FindUserByID ...
func FindUserByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		user, err := s.Users().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToUserDTOTable(*user))
	}
}

// CreateUser ...
func CreateUser(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userDTO := new(model.UserDTO)

		// TODO: There are 6 action here. These should be moved to service layer
		// user's service layer functions located in /app/user.go file is

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
		// Hasing the master password with bcrypt
		pwdHash, err := bcrypt.GenerateFromPassword([]byte(userDTO.MasterPassword), bcrypt.MinCost)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		userDTO.MasterPassword = string(pwdHash)

		// All new users are free member and Member (not Admin)
		userDTO.Plan = "Free"
		userDTO.Role = "Member"

		// Generate new UUID for user
		userDTO.UUID = uuid.NewV4()

		createdUser, err := app.CreateUser(s, userDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 5. Generate Schema and update user Schema field
		// TODO: I am not sure if we need this schema field
		isAuthorized := r.Context().Value("authorized").(bool)
		userDTO.Schema = fmt.Sprintf("user%d", createdUser.ID)
		updatedUser, err := app.UpdateUser(s, createdUser, userDTO, isAuthorized)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 6. Migrate user specific tables in user's schema
		err = s.Users().Migrate(updatedUser.Schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToUserDTO(createdUser))
	}
}

// TODO:
// UpdateUser ...
func UpdateUser(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Get id and check if it is an integer
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Decode request body to userDTO object
		var userDTO model.UserDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&userDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Check if user exist in database
		user, err := s.Users().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// claims := token.Claims.(jwt.MapClaims)
		// fmt.Printf("Token for user %v expires %v", claims["user"], claims["exp"])

		// atClaims["authorized"] = true

		// Check if user exist in database with new email address
		if userDTO.Email != user.Email {
			_, err := s.Users().FindByEmail(userDTO.Email)
			if err == nil {
				errs := []string{"This email is already used!"}
				message := "User email address couldn't updated!"
				RespondWithErrors(w, http.StatusBadRequest, message, errs)
				return
			}
		}

		isAuthorized := r.Context().Value("authorized").(bool)

		// Update user
		userDTO.UUID = uuid.NewV4()
		userDTO.Plan = "Free"
		updatedUser, err := app.UpdateUser(s, user, &userDTO, isAuthorized)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToUserDTO(updatedUser))
	}
}

// DeleteUser ...
func DeleteUser(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		user, err := s.Users().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Users().Delete(user.ID, user.Schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  "Success",
			Message: "User deleted successfully!",
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
