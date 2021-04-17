package note

import (
	"github.com/jinzhu/gorm"
	"github.com/passwall/passwall-server/model"
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
func (p *Repository) All(schema string) ([]model.Note, error) {
	notes := []model.Note{}
	return notes, p.db.Table(schema + ".notes").Find(&notes).Error
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Note, error) {
	query := p.db.Table(schema + ".notes").Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	// TODO: This is not working because notes are encrypted
	if argsStr["search"] != "" {
		query = query.Where("note LIKE ?", "%"+argsStr["search"]+"%")
	}

	notes := []model.Note{}
	return notes, query.Find(&notes).Error
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.Note, error) {
	note := new(model.Note)
	return note, p.db.Table(schema+".notes").Where(`id = ?`, id).First(&note).Error
}

// Save ...
func (p *Repository) Save(note *model.Note, schema string) (*model.Note, error) {
	return note, p.db.Table(schema + ".notes").Save(&note).Error
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	return p.db.Table(schema + ".notes").Delete(&model.Note{ID: id}).Error
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".notes").AutoMigrate(&model.Note{}).Error
}
