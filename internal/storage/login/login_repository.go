package login

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
func (p *Repository) All(schema string) ([]model.Login, error) {
	logins := []model.Login{}
	err := p.db.Table(schema + ".logins").Find(&logins).Error
	if err != nil {
		logger.Errorf("Error getting all logins error %v", err)
		return nil, err
	}

	return logins, err
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.Login, error) {
	login := new(model.Login)
	err := p.db.Table(schema+".logins").Where(`id = ?`, id).First(&login).Error
	if err != nil {
		logger.Errorf("Error finding login %v error %v", id, err)
		return nil, err
	}
	return login, err
}

// Update ...
func (p *Repository) Update(login *model.Login, schema string) (*model.Login, error) {
	err := p.db.Table(schema + ".logins").Save(&login).Error
	if err != nil {
		logger.Errorf("Error updating login %v error %v", login, err)
		return nil, err
	}

	return login, nil
}

// Create ...
func (p *Repository) Create(login *model.Login, schema string) (*model.Login, error) {
	err := p.db.Table(schema + ".logins").Create(&login).Error
	if err != nil {
		logger.Errorf("Error creating login %v error %v", login, err)
		return nil, err
	}

	return login, nil
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	err := p.db.Table(schema + ".logins").Delete(&model.Login{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".logins").AutoMigrate(&model.Login{})
}
