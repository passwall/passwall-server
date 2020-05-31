package api

import (
	"encoding/json"
	"net/http"

	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
)

// FindSamePassword ...
func FindSamePassword(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var password model.Password

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&password); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		urls, err := app.FindSamePassword(s, password)

		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, urls)
	}
}

// Password generates new password
func GeneratePassword(w http.ResponseWriter, r *http.Request) {
	generatedPass, err := app.Password()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, err.Error())
	}
	password := generatedPass
	response := model.Response{
		Code:    http.StatusOK,
		Status:  Success,
		Message: password,
	}
	RespondWithJSON(w, http.StatusOK, response)
}
