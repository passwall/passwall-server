package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
)

const (
	//ServerDeleteSuccess represents message when deleting server successfully
	ServerDeleteSuccess = "Server deleted successfully!"
)

// FindAllServers ...
func FindAllServers(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var serverList []model.Server

		// Get all servers from db
		schema := r.Context().Value("schema").(string)
		serverList, err = s.Servers().All(schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		for i := range serverList {
			decServer, err := app.DecryptModel(&serverList[i])
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
			serverList[i] = *decServer.(*model.Server)
		}

		RespondWithJSON(w, http.StatusOK, serverList)
	}
}

// FindServerByID ...
func FindServerByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if id is integer
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Find server by id from db
		schema := r.Context().Value("schema").(string)
		server, err := s.Servers().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		decServer, err := app.DecryptModel(server)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		serverDTO := model.ToServerDTO(decServer.(*model.Server))

		RespondWithJSON(w, http.StatusOK, serverDTO)
	}
}

// CreateServer ...
func CreateServer(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Unmarshal request body to serverDTO
		var serverDTO model.ServerDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&serverDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		defer r.Body.Close()

		// Add new server to db
		schema := r.Context().Value("schema").(string)
		createdServer, err := app.CreateServer(s, &serverDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Decrypt server side encrypted fields
		decServer, err := app.DecryptModel(createdServer)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		createdServerDTO := model.ToServerDTO(decServer.(*model.Server))

		RespondWithJSON(w, http.StatusOK, createdServerDTO)
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

		// Unmarshal request body to serverDTO
		var serverDTO model.ServerDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&serverDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		defer r.Body.Close()

		// Find server defined by id
		schema := r.Context().Value("schema").(string)
		server, err := s.Servers().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Update server
		updatedServer, err := app.UpdateServer(s, server, &serverDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		decServer, err := app.DecryptModel(updatedServer)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create DTO
		updatedServerDTO := model.ToServerDTO(decServer.(*model.Server))

		RespondWithJSON(w, http.StatusOK, updatedServerDTO)
	}
}

// BulkUpdateServers updates servers in payload
func BulkUpdateServers(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var serverList []model.ServerDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&serverList); err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		defer r.Body.Close()

		for _, serverDTO := range serverList {
			// Find server defined by id
			schema := r.Context().Value("schema").(string)
			server, err := s.Servers().FindByID(serverDTO.ID, schema)
			if err != nil {
				RespondWithError(w, http.StatusNotFound, err.Error())
				return
			}

			// Update server
			_, err = app.UpdateServer(s, server, &serverDTO, schema)
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  "Success",
			Message: "Bulk update completed successfully!",
		}
		RespondWithJSON(w, http.StatusOK, response)
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
