package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"
)

const (
	loginDeleteSuccess = "Login deleted successfully!"
)

// FindAllLogins finds all logins
func FindAllLogins(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var loginList []model.Login

		fields := []string{"id", "created_at", "updated_at", "title"}
		argsStr, argsInt := SetArgs(r, fields)

		schema := r.Context().Value("schema").(string)
		loginList, err = s.Logins().FindAll(argsStr, argsInt, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		for i := range loginList {
			uLogin, err := app.DecryptModel(&loginList[i])
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
			loginList[i] = *uLogin.(*model.Login)
		}

		transmissionKey := r.Context().Value("transmissionKey").(string)
		RespondWithEncJSON(w, http.StatusOK, transmissionKey, loginList)
	}
}

// FindLoginsByID finds a login by id
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

		transmissionKey := r.Context().Value("transmissionKey").(string)
		RespondWithEncJSON(w, http.StatusOK, transmissionKey, loginDTO)
	}
}

// CreateLogin creates a login
func CreateLogin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var loginDTO model.LoginDTO
		env := viper.GetString("server.env")
		transmissionKey := r.Context().Value("transmissionKey").(string)

		if env == "prod" {
			var payload model.Payload
			decoder := json.NewDecoder(r.Body)
			if err := decoder.Decode(&payload); err != nil {
				RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
				return
			}
			defer r.Body.Close()

			// Decrypt payload
			dec, err := app.DecryptPayload(transmissionKey, []byte(payload.Data))
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
			}

			r.Body = ioutil.NopCloser(strings.NewReader(string(dec)))
		}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&loginDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Add new login to db
		schema := r.Context().Value("schema").(string)
		createdLogin, err := app.CreateLogin(s, &loginDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		createdLoginDTO := model.ToLoginDTO(createdLogin)

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, createdLoginDTO)
	}
}

// UpdateLogin updates a login
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
		transmissionKey := r.Context().Value("transmissionKey").(string)
		err = app.DecryptJSON(transmissionKey, []byte(payload.Data), &loginDTO)
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

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, updatedLoginDTO)
	}
}

// DeleteLogin deletes a login
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
			Message: loginDeleteSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}

// TestLogin login endpoint for test purposes
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
