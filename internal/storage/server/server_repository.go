package server

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
func (p *Repository) All(schema string) ([]model.Server, error) {
	servers := []model.Server{}
	return servers, p.db.Table(schema + ".servers").Find(&servers).Error
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Server, error) {
	query := p.db.Table(schema + ".servers").Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("title LIKE ? OR ip LIKE ?", "%"+argsStr["search"]+"%", "%"+argsStr["search"]+"%")
	}

	servers := []model.Server{}
	return servers, query.Find(&servers).Error
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.Server, error) {
	server := new(model.Server)
	return server, p.db.Table(schema+".servers").Where(`id = ?`, id).First(&server).Error
}

// Save ...
func (p *Repository) Save(server *model.Server, schema string) (*model.Server, error) {
	err := p.db.Table(schema + ".servers").Save(&server).Error
	return server, err
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	err := p.db.Table(schema + ".servers").Delete(&model.Server{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".servers").AutoMigrate(&model.Server{}).Error
}
