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
	noteDeleteSuccess = "Note deleted successfully!"
)

// FindAllNotes finds all notes
func FindAllNotes(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var noteList []model.Note
		// Get all notes from db
		schema := r.Context().Value("schema").(string)
		noteList, err = s.Notes().All(schema)
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

		RespondWithJSON(w, http.StatusOK, noteList)
	}
}

// FindNoteByID finds a note by id
func FindNoteByID(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if id is integer
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Find note by id from db
		schema := r.Context().Value("schema").(string)
		note, err := s.Notes().FindByID(uint(id), schema)
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

		// Create DTO
		noteDTO := model.ToNoteDTO(uNote.(*model.Note))

		RespondWithJSON(w, http.StatusOK, noteDTO)
	}
}

// CreateNote creates a note
func CreateNote(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Unmarshal request body to noteDTO
		var noteDTO model.NoteDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&noteDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
			return
		}
		defer r.Body.Close()

		// Add new note to db
		schema := r.Context().Value("schema").(string)
		createdNote, err := app.CreateNote(s, &noteDTO, schema)
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

		// Create DTO
		createdNoteDTO := model.ToNoteDTO(decNote.(*model.Note))

		RespondWithJSON(w, http.StatusOK, createdNoteDTO)
	}
}

// UpdateNote updates a note
func UpdateNote(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Unmarshal request body to noteDTO
		var noteDTO model.NoteDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&noteDTO); err != nil {
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

		// Create DTO
		updatedNoteDTO := model.ToNoteDTO(decNote.(*model.Note))

		RespondWithJSON(w, http.StatusOK, updatedNoteDTO)
	}
}

// BulkUpdateNotes updates notes in payload
func BulkUpdateNotes(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var noteList []model.NoteDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&noteList); err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
		defer r.Body.Close()

		for _, noteDTO := range noteList {
			// Find note defined by id
			schema := r.Context().Value("schema").(string)
			note, err := s.Notes().FindByID(noteDTO.ID, schema)
			if err != nil {
				RespondWithError(w, http.StatusNotFound, err.Error())
				return
			}

			// Update note
			_, err = app.UpdateNote(s, note, &noteDTO, schema)
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

// DeleteNote deletes a note
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
			Message: noteDeleteSuccess,
		}
		RespondWithJSON(w, http.StatusOK, response)
	}
}
