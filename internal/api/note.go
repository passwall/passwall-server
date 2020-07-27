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

const (
	NoteDeleteSuccess = "Note deleted successfully!"
)

// FindAll ...
func FindAllNotes(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		noteList := []model.Note{}

		fields := []string{"id", "created_at", "updated_at", "note"}
		argsStr, argsInt := SetArgs(r, fields)

		schema := r.Context().Value("schema").(string)
		noteList, err = s.Notes().FindAll(argsStr, argsInt, schema)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		noteList = app.DecryptNotes(noteList)

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, noteList)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// FindByID ...
func FindNoteByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		note, err := s.Notes().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		uNote, err := app.DecryptNote(s, note)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		noteDTO := model.ToNoteDTO(uNote)

		// Encrypt payload
		var payload model.Payload
		key := r.Context().Value("transmissionKey").(string)
		encrypted, err := app.EncryptJSON(key, noteDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// Create ...
func CreateNote(s storage.Store) http.HandlerFunc {
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
		var noteDTO model.NoteDTO
		key := r.Context().Value("transmissionKey").(string)
		err := app.DecryptJSON(key, []byte(payload.Data), &noteDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		createdNote, err := app.CreateNote(s, &noteDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		createdNoteDTO := model.ToNoteDTO(createdNote)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, createdNoteDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// Update ...
func UpdateNote(s storage.Store) http.HandlerFunc {
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
		var noteDTO model.NoteDTO
		key := r.Context().Value("transmissionKey").(string)
		err = app.DecryptJSON(key, []byte(payload.Data), &noteDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		note, err := s.Notes().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedNote, err := app.UpdateNote(s, note, &noteDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		updatedNoteDTO := model.ToNoteDTO(updatedNote)

		// Encrypt payload
		encrypted, err := app.EncryptJSON(key, updatedNoteDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		payload.Data = string(encrypted)

		RespondWithJSON(w, http.StatusOK, payload)
	}
}

// DeleteNote ...
func DeleteNote(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		schema := r.Context().Value("schema").(string)
		note, err := s.Notes().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Notes().Delete(note.ID, schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		response := model.Response{
			Code:    http.StatusOK,
			Status:  Success,
			Message: NoteDeleteSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
