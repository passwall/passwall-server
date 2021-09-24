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
	return servers, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Server, error) {
	servers := []model.Server{}

	query := p.db
	query = query.Table(schema + ".servers")
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("title LIKE ? OR ip LIKE ?", "%"+argsStr["search"]+"%", "%"+argsStr["search"]+"%")
	}

	err := query.Find(&servers).Error
	return servers, err
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.Server, error) {
	server := new(model.Server)
	err := p.db.Table(schema+".servers").Where(`id = ?`, id).First(&server).Error
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
