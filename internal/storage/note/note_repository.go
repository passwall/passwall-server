package note

import (
	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/model"
)

// Repository ...
type Repository struct {
	db *gorm.DB
}

// NewRepository ...
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// All ...
func (p *Repository) All() ([]model.Note, error) {
	notes := []model.Note{}
	err := p.db.Find(&notes).Error
	return notes, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Note, error) {
	notes := []model.Note{}

	query := p.db
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	// TODO: This is not working because notes are encrypted
	if argsStr["search"] != "" {
		query = query.Where("note LIKE ?", "%"+argsStr["search"]+"%")
	}

	err := query.Find(&notes).Error
	return notes, err
}

// FindByID ...
func (p *Repository) FindByID(id uint) (model.Note, error) {
	note := model.Note{}
	err := p.db.Where(`id = ?`, id).First(&note).Error
	return note, err
}

// Save ...
func (p *Repository) Save(note model.Note) (model.Note, error) {
	err := p.db.Save(&note).Error
	return note, err
}

// Delete ...
func (p *Repository) Delete(id uint) error {
	err := p.db.Delete(&model.Note{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.Note{}).Error
}
