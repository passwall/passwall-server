package storage

import (
	"log"

	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/model"
)

// NoteRepository ...
type NoteRepository struct {
	DB *gorm.DB
}

// NewNoteRepository ...
func NewNoteRepository(db *gorm.DB) NoteRepository {
	return NoteRepository{DB: db}
}

// All ...
func (p *NoteRepository) All() ([]model.Note, error) {
	notes := []model.Note{}
	err := p.DB.Find(&notes).Error
	return notes, err
}

// FindAll ...
func (p *NoteRepository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Note, error) {
	notes := []model.Note{}

	query := p.DB
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("note LIKE ?", "%"+argsStr["search"]+"%")
	}

	err := query.Find(&notes).Error
	return notes, err
}

// FindByID ...
func (p *NoteRepository) FindByID(id uint) (model.Note, error) {
	note := model.Note{}
	err := p.DB.Where(`id = ?`, id).First(&note).Error
	return note, err
}

// Save ...
func (p *NoteRepository) Save(note model.Note) (model.Note, error) {
	err := p.DB.Save(&note).Error
	return note, err
}

// Delete ...
func (p *NoteRepository) Delete(id uint) error {
	err := p.DB.Delete(&model.Note{ID: id}).Error
	return err
}

// Migrate ...
func (p *NoteRepository) Migrate() {
	err := p.DB.AutoMigrate(&model.Note{}).Error
	if err != nil {
		log.Println(err)
	}
}
