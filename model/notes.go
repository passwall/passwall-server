package model

import (
	"time"
)

// Note ...
type Note struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
	Title     string     `json:"title"`
	Note      string     `json:"note" encrypt:"true"`
}

// NoteDTO ...
type NoteDTO struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
	Note  string `json:"note"`
}

// ToNote ...
func ToNote(noteDTO *NoteDTO) *Note {
	return &Note{
		Title: noteDTO.Title,
		Note:  noteDTO.Note,
	}
}

// ToNoteDTO ...
func ToNoteDTO(note *Note) *NoteDTO {
	return &NoteDTO{
		ID:    note.ID,
		Title: note.Title,
		Note:  note.Note,
	}
}

// ToNoteDTOs ...
func ToNoteDTOs(notes []*Note) []*NoteDTO {
	noteDTOs := make([]*NoteDTO, len(notes))

	for i, itm := range notes {
		noteDTOs[i] = ToNoteDTO(itm)
	}

	return noteDTOs
}

/* EXAMPLE JSON OBJECT
{
	"title":"Lorem ipsum",
	"note":"Lorem ipsum",
}
*/
