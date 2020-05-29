package email

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
func (p *Repository) All() ([]model.Email, error) {
	emails := []model.Email{}
	err := p.db.Find(&emails).Error
	return emails, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Email, error) {
	emails := []model.Email{}

	query := p.db
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("email LIKE ?", "%"+argsStr["search"]+"%")
	}

	err := query.Find(&emails).Error
	return emails, err
}

// FindByID ...
func (p *Repository) FindByID(id uint) (model.Email, error) {
	email := model.Email{}
	err := p.db.Where(`id = ?`, id).First(&email).Error
	return email, err
}

// Save ...
func (p *Repository) Save(email model.Email) (model.Email, error) {
	err := p.db.Save(&email).Error
	return email, err
}

// Delete ...
func (p *Repository) Delete(id uint) error {
	err := p.db.Delete(&model.Email{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.Email{}).Error
}
