package email

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
func (p *Repository) All(schema string) ([]model.Email, error) {
	emails := []model.Email{}
	return emails, p.db.Table(schema + ".emails").Find(&emails).Error
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Email, error) {
	query := p.db.Table(schema + ".emails").Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("email LIKE ?", "%"+argsStr["search"]+"%")
	}

	emails := []model.Email{}
	return emails, query.Find(&emails).Error
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.Email, error) {
	email := new(model.Email)
	return email, p.db.Table(schema+".emails").Where(`id = ?`, id).First(&email).Error
}

// Save ...
func (p *Repository) Save(email *model.Email, schema string) (*model.Email, error) {
	return email, p.db.Table(schema + ".emails").Save(&email).Error
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	return p.db.Table(schema + ".emails").Delete(&model.Email{ID: id}).Error
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".emails").AutoMigrate(&model.Email{}).Error
}
