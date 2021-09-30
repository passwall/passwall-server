package email

import (
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/logger"
	"gorm.io/gorm"
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
	err := p.db.Table(schema + ".emails").Find(&emails).Error
	return emails, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Email, error) {
	emails := []model.Email{}

	query := p.db
	query = query.Table(schema + ".emails")
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
func (p *Repository) FindByID(id uint, schema string) (*model.Email, error) {
	email := new(model.Email)
	err := p.db.Table(schema+".emails").Where(`id = ?`, id).First(&email).Error
	return email, err
}

// Update ...
func (p *Repository) Update(email *model.Email, schema string) (*model.Email, error) {
	err := p.db.Table(schema + ".emails").Save(&email).Error
	if err != nil {
		logger.Errorf("Error updating email %v error %v", email, err)
		return nil, err
	}
	return email, nil
}

// Create ...
func (p *Repository) Create(email *model.Email, schema string) (*model.Email, error) {
	err := p.db.Table(schema + ".emails").Create(&email).Error
	if err != nil {
		logger.Errorf("Error creating email %v error %v", email, err)
		return nil, err
	}
	return email, nil
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	err := p.db.Table(schema + ".emails").Delete(&model.Email{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".emails").AutoMigrate(&model.Email{})
}
