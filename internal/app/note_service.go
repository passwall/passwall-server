package app

import (
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
)

// NoteService ...
type NoteService struct {
	NoteRepository storage.NoteRepository
}

// NewNoteService ...
func NewNoteService(p storage.NoteRepository) NoteService {
	return NoteService{NoteRepository: p}
}

// All ...
func (p *NoteService) All() ([]model.Note, error) {
	return p.NoteRepository.All()
}

// FindAll ...
func (p *NoteService) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Note, error) {
	return p.NoteRepository.FindAll(argsStr, argsInt)
}

// FindByID ...
func (p *NoteService) FindByID(id uint) (model.Note, error) {
	return p.NoteRepository.FindByID(id)
}

// Save ...
func (p *NoteService) Save(note model.Note) (model.Note, error) {
	return p.NoteRepository.Save(note)
}

// Delete ...
func (p *NoteService) Delete(id uint) error {
	return p.NoteRepository.Delete(id)
}

// Migrate ...
func (p *NoteService) Migrate() {
	p.NoteRepository.Migrate()
}
