package login

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
func (p *Repository) All(schema string) ([]model.Login, error) {
	logins := []model.Login{}
	return logins, p.db.Table(schema + ".logins").Find(&logins).Error
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Login, error) {
	query := p.db.Table(schema + ".logins").Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("url LIKE ? OR username LIKE ?", "%"+argsStr["search"]+"%", "%"+argsStr["search"]+"%")
	}

	logins := []model.Login{}
	return logins, query.Find(&logins).Error
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.Login, error) {
	login := new(model.Login)
	return login, p.db.Table(schema+".logins").Where(`id = ?`, id).First(&login).Error
}

// Save ...
func (p *Repository) Save(login *model.Login, schema string) (*model.Login, error) {
	return login, p.db.Table(schema + ".logins").Save(&login).Error
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	return p.db.Table(schema + ".logins").Delete(&model.Login{ID: id}).Error
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".logins").AutoMigrate(&model.Login{}).Error
}
