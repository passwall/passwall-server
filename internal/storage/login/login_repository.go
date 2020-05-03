package login

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
func (p *Repository) All() ([]model.Login, error) {
	logins := []model.Login{}
	err := p.db.Find(&logins).Error
	return logins, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Login, error) {
	logins := []model.Login{}

	query := p.db
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
func (p *Repository) FindByID(id uint) (model.Login, error) {
	login := model.Login{}
	err := p.db.Where(`id = ?`, id).First(&login).Error
	return login, err
}

// Save ...
func (p *Repository) Save(login model.Login) (model.Login, error) {
	err := p.db.Save(&login).Error
	return login, err
}

// Delete ...
func (p *Repository) Delete(id uint) error {
	err := p.db.Delete(&model.Login{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.Login{}).Error
}
