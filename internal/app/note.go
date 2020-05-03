package app

import (
	"encoding/base64"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// CreateNote creates a new note and saves it to the store
func CreateNote(s storage.Store, dto *model.NoteDTO) (*model.Note, error) {

	rawPass := dto.Note
	dto.Note = base64.StdEncoding.EncodeToString(Encrypt(dto.Note, viper.GetString("server.passphrase")))

	createdNote, err := s.Notes().Save(*model.ToNote(dto))
	if err != nil {
		return nil, err
	}

	createdNote.Note = rawPass
	return &createdNote, nil
}

// UpdateNote updates the note with the dto and applies the changes in the store
func UpdateNote(s storage.Store, note *model.Note, dto *model.NoteDTO) (*model.Note, error) {
	rawPass := dto.Note
	dto.Note = base64.StdEncoding.EncodeToString(Encrypt(dto.Note, viper.GetString("server.passphrase")))

	dto.ID = uint(note.ID)
	note = model.ToNote(dto)
	note.ID = uint(note.ID)

	updatedNote, err := s.Notes().Save(*note)
	if err != nil {

		return nil, err
	}
	updatedNote.Note = rawPass
	return &updatedNote, nil
}

// DecryptNote decrypts note
func DecryptNote(s storage.Store, note *model.Note) (*model.Note, error) {
	passByte, _ := base64.StdEncoding.DecodeString(note.Note)
	note.Note = string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

	return note, nil
}

// DecryptNotes ...
// TODO: convert to pointers
func DecryptNotes(notes []model.Note) []model.Note {
	for i := range notes {
		if notes[i].Note == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(notes[i].Note)
		passB64 := string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		notes[i].Note = passB64
	}
	return notes
}
