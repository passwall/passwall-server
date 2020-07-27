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

// FindAllEmails ...
func FindAllEmails(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		emails := []model.Email{}

		fields := []string{"id", "created_at", "updated_at", "email"}
		argsStr, argsInt := SetArgs(r, fields)

		schema := r.Context().Value("schema").(string)
		emails, err = s.Emails().FindAll(argsStr, argsInt, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// emails = app.DecryptEmailPasswords(emails)

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, emails)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// FindEmailByID ...
func FindEmailByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		account, err := s.Emails().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		email, err := app.DecryptEmailPassword(s, account)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		emailDTO := model.ToEmailDTO(email)

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, emailDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// CreateEmail ...
func CreateEmail(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// TODO BEGIN: This part should be in a helper function
		// Unmarshal request body to payload
		var payload model.Payload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()
		// TODO END:

		// Decrypt payload
		var emailDTO model.EmailDTO
		key := r.Context().Value("transmissionKey").(string)
		err := app.DecryptJSON(key, []byte(payload.Data), &emailDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		createdEmail, err := app.CreateEmail(s, &emailDTO, schema)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		createdEmailDTO := model.ToEmailDTO(createdEmail)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, createdEmailDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// UpdateEmail ...
func UpdateEmail(s storage.Store) http.HandlerFunc {
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
		var emailDTO model.EmailDTO
		key := r.Context().Value("transmissionKey").(string)
		err = app.DecryptJSON(key, []byte(payload.Data), &emailDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		email, err := s.Emails().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedEmail, err := app.UpdateEmail(s, email, &emailDTO, schema)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		updatedEmailDTO := model.ToEmailDTO(updatedEmail)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, updatedEmailDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// DeleteEmail ...
func DeleteEmail(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		email, err := s.Emails().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Emails().Delete(email.ID, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  "Success",
			Message: "Email deleted successfully!",
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
