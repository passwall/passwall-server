package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"

	"github.com/gorilla/mux"
)

// FindAllUsers ...
func FindAllUsers(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		argsStr, argsInt := SetArgs(r, []string{"id", "created_at", "updated_at", "url", "username"})

		users, err := s.Users().FindAll(argsStr, argsInt)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// users = app.DecryptUserPasswords(users)
		RespondWithJSON(w, http.StatusOK, model.ToUserDTOs(users))
	}
}

// FindUserByID ...
func FindUserByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(mux.Vars(r)["id"])
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
		if err := json.NewDecoder(r.Body).Decode(&userDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// 2. Run validator according to model.UserDTO validator tags
		if validateError := validator.New().Struct(userDTO); validateError != nil {
			errs := GetErrors(validateError.(validator.ValidationErrors))
			RespondWithErrors(w, http.StatusBadRequest, InvalidRequestPayload, errs)
			return
		}

		// 3. Check if user exist in database
		if _, err := s.Users().FindByEmail(userDTO.Email); err == nil {
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
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Decode request body to userDTO object
		var userDTO model.UserDTO
		if err := json.NewDecoder(r.Body).Decode(&userDTO); err != nil {
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
			if _, err := s.Users().FindByEmail(userDTO.Email); err == nil {
				errs := []string{"This email is already used!"}
				message := "User email address couldn't updated!"
				RespondWithErrors(w, http.StatusBadRequest, message, errs)
				return
			}
		}

		// Update user
		updatedUser, err := app.UpdateUser(s, user, &userDTO, r.Context().Value("authorized").(bool))
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

		RespondWithJSON(w, http.StatusOK,
			model.Response{
				Code:    http.StatusOK,
				Status:  Success,
				Message: "User deleted successfully!",
			})
	}
}
