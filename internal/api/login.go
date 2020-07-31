package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"

	"github.com/gorilla/mux"
)

const (
	LoginDeleteSuccess = "Login deleted successfully!"
)

// FindAllLogins ...
func FindAllLogins(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var loginList []model.Login

		fields := []string{"id", "created_at", "updated_at", "url", "username"}
		argsStr, argsInt := SetArgs(r, fields)

		schema := r.Context().Value("schema").(string)
		loginList, err = s.Logins().FindAll(argsStr, argsInt, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, loginList)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// FindLoginsByID ...
func FindLoginsByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Check if id is integer
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Find login by id from db
		schema := r.Context().Value("schema").(string)
		login, err := s.Logins().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		uLogin, err := app.DecryptModel(login)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		loginDTO := model.ToLoginDTO(uLogin.(*model.Login))

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, loginDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// Create ...
func CreateLogin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		payload, err := ToPayload(r)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Decrypt payload
		var loginDTO model.LoginDTO
		key := r.Context().Value("transmissionKey").(string)
		err = app.DecryptJSON(key, []byte(payload.Data), &loginDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Add new login to db
		schema := r.Context().Value("schema").(string)
		createdLogin, err := app.CreateLogin(s, &loginDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		createdLoginDTO := model.ToLoginDTO(createdLogin)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, createdLoginDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// Update ...
func UpdateLogin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Unmarshal request body to payload
		var payload model.Payload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Decrypt payload
		var loginDTO model.LoginDTO
		key := r.Context().Value("transmissionKey").(string)
		err = app.DecryptJSON(key, []byte(payload.Data), &loginDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		login, err := s.Logins().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedLogin, err := app.UpdateLogin(s, login, &loginDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		updatedLoginDTO := model.ToLoginDTO(updatedLogin)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, updatedLoginDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// Delete ...
func DeleteLogin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		login, err := s.Logins().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Logins().Delete(login.ID, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: LoginDeleteSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// Test endpoint ...
func TestLogin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: "Test success!",
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
