package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"
)

// FindAllEmails ...
func FindAllEmails(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		emailList := []model.Email{}

		// Setup variables
		transmissionKey := r.Context().Value("transmissionKey").(string)

		fields := []string{"id", "created_at", "updated_at", "email"}
		argsStr, argsInt := SetArgs(r, fields)

		schema := r.Context().Value("schema").(string)
		emailList, err = s.Emails().FindAll(argsStr, argsInt, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		for i := range emailList {
			decEmail, err := app.DecryptModel(&emailList[i])
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
			emailList[i] = *decEmail.(*model.Email)
		}

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, emailList)
	}
}

// FindEmailByID ...
func FindEmailByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Setup variables
		transmissionKey := r.Context().Value("transmissionKey").(string)

		// Check if id is integer
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

		// Decrypt server side encrypted fields
		decEmail, err := app.DecryptModel(email)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		emailDTO := model.ToEmailDTO(decEmail.(*model.Email))

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, emailDTO)
	}
}

// CreateEmail ...
func CreateEmail(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Setup variables
		env := viper.GetString("server.env")
		transmissionKey := r.Context().Value("transmissionKey").(string)

		// Update request body according to env.
		// If env is dev, then do nothing
		// If env is prod, then decrypt payload with transmission key
		if err := ToBody(r, env, transmissionKey); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Unmarshal request body to loginDTO
		var emailDTO model.EmailDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&emailDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Add new login to db
		schema := r.Context().Value("schema").(string)
		createdEmail, err := app.CreateEmail(s, &emailDTO, schema)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Decrypt server side encrypted fields
		decEmail, err := app.DecryptModel(createdEmail)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		createdEmailDTO := model.ToEmailDTO(decEmail.(*model.Email))

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, createdEmailDTO)
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

		// Setup variables
		env := viper.GetString("server.env")
		transmissionKey := r.Context().Value("transmissionKey").(string)

		if err := ToBody(r, env, transmissionKey); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Unmarshal request body to emailDTO
		var emailDTO model.EmailDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&emailDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Find login defined by id
		schema := r.Context().Value("schema").(string)
		email, err := s.Emails().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Update email
		updatedEmail, err := app.UpdateEmail(s, email, &emailDTO, schema)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Decrypt server side encrypted fields
		decEmail, err := app.DecryptModel(updatedEmail)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		updatedEmailDTO := model.ToEmailDTO(decEmail.(*model.Email))

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, updatedEmailDTO)

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
