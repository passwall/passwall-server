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
	NoteDeleteSuccess = "Note deleted successfully!"
)

// FindAll ...
func FindAllNotes(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		notes := []model.Note{}

		fields := []string{"id", "created_at", "updated_at", "note"}
		argsStr, argsInt := SetArgs(r, fields)

		notes, err = s.Notes().FindAll(argsStr, argsInt)

		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		notes = app.DecryptNotes(notes)
		RespondWithJSON(w, http.StatusOK, notes)
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

		note, err := s.Notes().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		uNote, err := app.DecryptNote(s, &note)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToNoteDTO(uNote))
	}
}

// Create ...
func CreateNote(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var noteDTO model.NoteDTO

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&noteDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		createdNote, err := app.CreateNote(s, &noteDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToNoteDTO(createdNote))
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

		var noteDTO model.NoteDTO
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&noteDTO); err != nil {
			RespondWithError(w, http.StatusBadRequest, InvalidRequestPayload)
			return
		}
		defer r.Body.Close()

		note, err := s.Notes().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		updatedNote, err := app.UpdateNote(s, &note, &noteDTO)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		RespondWithJSON(w, http.StatusOK, model.ToNoteDTO(updatedNote))
	}
}

// Delete ...
func DeleteNote(s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		note, err := s.Notes().FindByID(uint(id))
		if err != nil {
			RespondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		err = s.Notes().Delete(note.ID)
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
