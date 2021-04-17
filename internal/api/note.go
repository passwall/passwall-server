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
	noteDeleteSuccess = "Note deleted successfully!"
)

// FindAllNotes finds all notes
func FindAllNotes(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		argsStr, argsInt := SetArgs(r, []string{"id", "created_at", "updated_at", "note"})

		// Get all notes from db
		noteList, err := s.Notes().FindAll(argsStr, argsInt, r.Context().Value("schema").(string))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		for i := range noteList {
			uNote, err := app.DecryptModel(&noteList[i])
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
			noteList[i] = *uNote.(*model.Note)
		}

		RespondWithEncJSON(w, http.StatusOK, r.Context().Value("transmissionKey").(string), noteList)
	}
}

// FindNoteByID finds a note by id
func FindNoteByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if id is integer
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Find note by id from db
		note, err := s.Notes().FindByID(uint(id), r.Context().Value("schema").(string))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		uNote, err := app.DecryptModel(note)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithEncJSON(w, http.StatusOK, r.Context().Value("transmissionKey").(string), model.ToNoteDTO(uNote.(*model.Note)))
	}
}

// CreateNote creates a note
func CreateNote(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Setup variables
		transmissionKey := r.Context().Value("transmissionKey").(string)

		// Update request body according to env.
		// If env is dev, then do nothing
		// If env is prod, then decrypt payload with transmission key
		if err := ToBody(r, viper.GetString("server.env"), transmissionKey); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Unmarshal request body to noteDTO
		var noteDTO model.NoteDTO
		if err := json.NewDecoder(r.Body).Decode(&noteDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Add new note to db
		createdNote, err := app.CreateNote(s, &noteDTO, r.Context().Value("schema").(string))
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		decNote, err := app.DecryptModel(createdNote)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, model.ToNoteDTO(decNote.(*model.Note)))
	}
}

// UpdateNote updates a note
func UpdateNote(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Setup variables
		transmissionKey := r.Context().Value("transmissionKey").(string)

		if err := ToBody(r, viper.GetString("server.env"), transmissionKey); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		// Unmarshal request body to noteDTO
		var noteDTO model.NoteDTO
		if err := json.NewDecoder(r.Body).Decode(&noteDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Find note defined by id
		schema := r.Context().Value("schema").(string)
		note, err := s.Notes().FindByID(uint(id), schema)
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		// Update note
		updatedNote, err := app.UpdateNote(s, note, &noteDTO, schema)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Decrypt server side encrypted fields
		decNote, err := app.DecryptModel(updatedNote)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithEncJSON(w, http.StatusOK, transmissionKey, model.ToNoteDTO(decNote.(*model.Note)))
	}
}

// DeleteNote deletes a note
func DeleteNote(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(mux.Vars(r)["id"])
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

		RespondWithJSON(w, http.StatusOK,
			model.Response{
				Code:    http.StatusOK,
				Status:  Success,
				Message: noteDeleteSuccess,
			})
	}
}
