package app

import (
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/logger"
)

// FindAllNotes finds all logins
func FindAllNotes(s storage.Store, schema string) ([]model.Note, error) {
	list, err := s.Notes().All(schema)
	if err != nil {
		return nil, err
	}

	// Decrypt server side encrypted fields
	for i := range list {
		m, err := DecryptModel(&list[i])
		if err != nil {
			logger.Errorf("Error while decrypting credit card: %v", err)
			continue
		}
		list[i] = *m.(*model.Note)
	}

	return list, nil
}

// CreateNote creates a new note and saves it to the store
func CreateNote(s storage.Store, dto *model.NoteDTO, schema string) (*model.Note, error) {
	rawModel := model.ToNote(dto)
	encModel := EncryptModel(rawModel)

	createdNote, err := s.Notes().Create(encModel.(*model.Note), schema)
	if err != nil {
		return nil, err
	}

	return createdNote, nil
}

// UpdateNote updates the note with the dto and applies the changes in the store
func UpdateNote(s storage.Store, note *model.Note, dto *model.NoteDTO, schema string) (*model.Note, error) {
	rawModel := model.ToNote(dto)
	encModel := EncryptModel(rawModel).(*model.Note)

	note.Title = encModel.Title
	note.Note = encModel.Note

	updatedNote, err := s.Notes().Update(note, schema)
	if err != nil {
		return nil, err
	}

	return updatedNote, nil
}
