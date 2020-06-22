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

const (
	ServerDeleteSuccess = "Server deleted successfully!"
)

// FindAllServers ...
func FindAllServers(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var serverList []model.Server

		fields := []string{"id", "created_at", "updated_at", "title", "ip", "username", "url", "hosting_username", "admin_username"}
		argsStr, argsInt := SetArgs(r, fields)

		schema := r.Context().Value("schema").(string)
		serverList, err = s.Servers().FindAll(argsStr, argsInt, schema)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// serverList = app.DecryptServerPasswords(serverList)
		RespondWithJSON(w, http.StatusOK, serverList)
	}
}

// FindServerByID ...
func FindServerByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		server, err := s.Servers().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToServerDTO(server))
	}
}

// CreateServer ...
func CreateServer(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var serverDTO model.ServerDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&serverDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		schema := r.Context().Value("schema").(string)
		createdServer, err := app.CreateServer(s, &serverDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToServerDTO(createdServer))
	}
}

// UpdateServer ...
func UpdateServer(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		var serverDTO model.ServerDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&serverDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()
		schema := r.Context().Value("schema").(string)
		server, err := s.Servers().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedServer, err := app.UpdateServer(s, server, &serverDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToServerDTO(updatedServer))
	}
}

// DeleteServer ...
func DeleteServer(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		server, err := s.Servers().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Servers().Delete(server.ID, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: ServerDeleteSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
