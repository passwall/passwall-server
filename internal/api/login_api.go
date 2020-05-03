package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/pass-wall/passwall-server/internal/app"
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// FindAll ...
func FindAllLogins(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		logins := []model.Login{}

		fields := []string{"id", "created_at", "updated_at", "url", "username"}
		argsStr, argsInt := SetArgs(r, fields)

		logins, err = s.Logins().FindAll(argsStr, argsInt)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		logins = app.DecryptLoginPasswords(logins)
		RespondWithJSON(w, http.StatusOK, logins)
	}
}

// FindByID ...
func FindLoginsByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		login, err := s.Logins().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		passByte, _ := base64.StdEncoding.DecodeString(login.Password)
		login.Password = string(app.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

		RespondWithJSON(w, http.StatusOK, model.ToLoginDTO(login))
	}
}

// Create ...
func CreateLogin(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var loginDTO model.LoginDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&loginDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		if loginDTO.Password == "" {
			loginDTO.Password = app.Password()
		}

		rawPass := loginDTO.Password
		loginDTO.Password = base64.StdEncoding.EncodeToString(app.Encrypt(loginDTO.Password, viper.GetString("server.passphrase")))

		createdLogin, err := s.Logins().Save(model.ToLogin(loginDTO))
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		createdLogin.Password = rawPass

		RespondWithJSON(w, http.StatusOK, model.ToLoginDTO(createdLogin))
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

		var loginDTO model.LoginDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&loginDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		login, err := s.Logins().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		if loginDTO.Password == "" {
			loginDTO.Password = app.Password()
		}
		rawPass := loginDTO.Password
		loginDTO.Password = base64.StdEncoding.EncodeToString(app.Encrypt(loginDTO.Password, viper.GetString("server.passphrase")))

		login.URL = loginDTO.URL
		login.Username = loginDTO.Username
		login.Password = loginDTO.Password
		updatedLogin, err := s.Logins().Save(login)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}
		updatedLogin.Password = rawPass
		RespondWithJSON(w, http.StatusOK, model.ToLoginDTO(updatedLogin))
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

		login, err := s.Logins().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Logins().Delete(login.ID)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{http.StatusOK, "Success", "Login deleted successfully!"}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
