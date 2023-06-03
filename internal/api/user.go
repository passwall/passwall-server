package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/go-playground/validator/v10"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
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

		RespondWithJSON(w, http.StatusOK, model.ToUserDTO(createdUser))
	}
}

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

		updatedUser, err := app.UpdateUser(s, user, &userDTO, isAuthorized)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToUserDTO(updatedUser))
	}
}

// UpdateUser ...
func Migrate(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Get id and check if it is an integer
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Check if user exist in database
		user, err := s.Users().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		userDTO := model.ToUserDTO(user)
		userDTO.IsMigrated = true

		isAuthorized := r.Context().Value("authorized").(bool)

		// Update user

		updatedUser, err := app.UpdateUser(s, user, userDTO, isAuthorized)
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

// UpdateUser ...
func ChangeMasterPassword(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenUserUUID := r.Context().Value("uuid").(string)

		var changeMasterPasswordDTO model.ChangeMasterPasswordDTO
		if err := json.NewDecoder(r.Body).Decode(&changeMasterPasswordDTO); err != nil {
			RespondWithError(w, http.StatusUnprocessableEntity, InvalidJSON)
			return
		}
		defer r.Body.Close()

		if err := app.PayloadValidator(changeMasterPasswordDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		email := changeMasterPasswordDTO.Email
		oldPass := changeMasterPasswordDTO.OldMasterPassword
		newPass := changeMasterPasswordDTO.NewMasterPassword

		if oldPass == newPass {
			RespondWithError(w, http.StatusBadRequest, "Passwords shouldn't be same")
			return
		}

		user, err := s.Users().FindByCredentials(email, oldPass)
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, userLoginErr)
			return
		}

		if tokenUserUUID != user.UUID.String() {
			RespondWithError(w, http.StatusUnauthorized, userLoginErr)
			return
		}

		_, err = app.ChangeMasterPassword(s, user, newPass)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  "Success",
			Message: "Master password changed successfully",
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// CheckCredentials ...
func CheckCredentials(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenUserUUID := r.Context().Value("uuid").(string)

		var loginDTO model.AuthLoginDTO
		if err := json.NewDecoder(r.Body).Decode(&loginDTO); err != nil {
			RespondWithError(w, http.StatusUnprocessableEntity, InvalidJSON)
			return
		}
		defer r.Body.Close()

		if err := app.PayloadValidator(loginDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		user, err := s.Users().FindByCredentials(loginDTO.Email, loginDTO.MasterPassword)
		if err != nil {
			RespondWithError(w, http.StatusUnauthorized, userLoginErr)
			return
		}

		if tokenUserUUID != user.UUID.String() {
			RespondWithError(w, http.StatusUnauthorized, userLoginErr)
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  "Success",
			Message: user.Secret,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
