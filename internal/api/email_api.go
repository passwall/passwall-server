package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"

	"github.com/gorilla/mux"
)

// FindAllEmails ...
func FindAllEmails(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		emails := []model.Email{}

		fields := []string{"id", "created_at", "updated_at", "email"}
		argsStr, argsInt := SetArgs(r, fields)

		emails, err = s.Emails().FindAll(argsStr, argsInt)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		emails = app.DecryptEmailPasswords(emails)
		RespondWithJSON(w, http.StatusOK, emails)
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

		account, err := s.Emails().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		email, err := app.DecryptEmailPassword(s, &account)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToEmailDTO(email))
	}
}

// CreateEmail ...
func CreateEmail(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var emailDTO model.EmailDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&emailDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		createdEmail, err := app.CreateEmail(s, &emailDTO)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToEmailDTO(createdEmail))
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

		var emailDTO model.EmailDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&emailDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		email, err := s.Emails().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedEmail, err := app.UpdateEmail(s, &email, &emailDTO)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToEmailDTO(updatedEmail))
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

		email, err := s.Emails().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Emails().Delete(email.ID)
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
