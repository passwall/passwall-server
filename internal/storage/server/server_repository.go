package server

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
func (p *Repository) All(schema string) ([]model.Server, error) {
	servers := []model.Server{}
	err := p.db.Table(schema + ".servers").Find(&servers).Error
	if err != nil {
		logger.Errorf("Error getting all servers error %v", err)
		return nil, err
	}
	return servers, err
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.Server, error) {
	server := new(model.Server)
	err := p.db.Table(schema+".servers").Where(`id = ?`, id).First(&server).Error
	if err != nil {
		logger.Errorf("Error getting server by id %v error %v", id, err)
		return nil, err
	}
	return server, err
}

// Update ...
func (p *Repository) Update(server *model.Server, schema string) (*model.Server, error) {
	err := p.db.Table(schema + ".servers").Save(&server).Error
	if err != nil {
		logger.Errorf("Error updating server %v error %v", server, err)
		return nil, err
	}

	return server, nil
}

// Create ...
func (p *Repository) Create(server *model.Server, schema string) (*model.Server, error) {
	err := p.db.Table(schema + ".servers").Create(&server).Error
	if err != nil {
		logger.Errorf("Error creating server %v error %v", server, err)
		return nil, err
	}

	return server, nil
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	err := p.db.Table(schema + ".servers").Delete(&model.Server{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".servers").AutoMigrate(&model.Server{})
}
