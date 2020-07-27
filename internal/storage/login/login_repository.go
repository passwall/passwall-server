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
	err := p.db.Table(schema + ".logins").Find(&logins).Error
	return logins, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Login, error) {
	logins := []model.Login{}

	query := p.db
	query = query.Table(schema + ".logins")
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("url LIKE ? OR username LIKE ?", "%"+argsStr["search"]+"%", "%"+argsStr["search"]+"%")
	}

	err := query.Find(&logins).Error
	return logins, err
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.Login, error) {
	login := new(model.Login)
	err := p.db.Table(schema+".logins").Where(`id = ?`, id).First(&login).Error
	return login, err
}

// Save ...
func (p *Repository) Save(login *model.Login, schema string) (*model.Login, error) {
	err := p.db.Table(schema + ".logins").Save(&login).Error
	return login, err
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	err := p.db.Table(schema + ".logins").Delete(&model.Login{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".logins").AutoMigrate(&model.Login{}).Error
}
