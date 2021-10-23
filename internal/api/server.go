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

const (
	//ServerDeleteSuccess represents message when deleting server successfully
	ServerDeleteSuccess = "Server deleted successfully!"
)

// FindAllServers ...
func FindAllServers(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var serverList []model.Server

		// Setup variables
		transmissionKey := r.Context().Value("transmissionKey").(string)

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

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, serverList)
	}
}

// FindServerByID ...
func FindServerByID(s storage.Store) http.HandlerFunc {
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

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, serverDTO)
	}
}

// CreateServer ...
func CreateServer(s storage.Store) http.HandlerFunc {
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

		// Unmarshal request body to serverDTO
		var serverDTO model.ServerDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&serverDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
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

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, createdServerDTO)
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

		// Setup variables
		env := viper.GetString("server.env")
		transmissionKey := r.Context().Value("transmissionKey").(string)

		if err := ToBody(r, env, transmissionKey); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Unmarshal request body to serverDTO
		var serverDTO model.ServerDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&serverDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
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

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, updatedServerDTO)
	}
}

// BulkUpdateServers updates servers in payload
func BulkUpdateServers(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var serverList []model.ServerDTO

		// Setup variables
		env := viper.GetString("server.env")
		transmissionKey := r.Context().Value("transmissionKey").(string)
		if err := ToBody(r, env, transmissionKey); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

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
