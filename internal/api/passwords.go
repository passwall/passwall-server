package api

import (
	"encoding/json"
	"net/http"

	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/common"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
)

// FindSamePassword ...
func FindSamePassword(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var password model.Password

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&password); err != nil {
			common.RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		urls, err := app.FindSamePassword(s, password)

		if err != nil {
			common.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		common.RespondWithJSON(w, http.StatusOK, urls)
	}
}

// GeneratePassword generates new password
func GeneratePassword(w http.ResponseWriter, r *http.Request) {
	password := encryption.Password()
	response := model.Response{http.StatusOK, "Success", password}
	common.RespondWithJSON(w, http.StatusOK, response)
}
