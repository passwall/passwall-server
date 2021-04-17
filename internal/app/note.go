package app

import (
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
)

// CreateNote creates a new note and saves it to the store
func CreateNote(s storage.Store, dto *model.NoteDTO, schema string) (*model.Note, error) {
	createdNote, err := s.Notes().Save(EncryptModel(model.ToNote(dto)).(*model.Note), schema)
	if err != nil {
		return nil, err
	}

	return createdNote, nil
}

// UpdateNote updates the note with the dto and applies the changes in the store
func UpdateNote(s storage.Store, note *model.Note, dto *model.NoteDTO, schema string) (*model.Note, error) {
	encModel := EncryptModel(model.ToNote(dto)).(*model.Note)

	note.Title = encModel.Title
	note.Note = encModel.Note

	updatedNote, err := s.Notes().Save(note, schema)
	if err != nil {
		return nil, err
	}

	return updatedNote, nil
}
