package storage

import (
	"log"

	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/model"
)

// LoginRepository ...
type LoginRepository struct {
	DB *gorm.DB
}

// NewLoginRepository ...
func NewLoginRepository(db *gorm.DB) LoginRepository {
	return LoginRepository{DB: db}
}

// All ...
func (p *LoginRepository) All() ([]model.Login, error) {
	logins := []model.Login{}
	err := p.DB.Find(&logins).Error
	return logins, err
}

// FindAll ...
func (p *LoginRepository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Login, error) {
	logins := []model.Login{}

	query := p.DB
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
func (p *LoginRepository) FindByID(id uint) (model.Login, error) {
	login := model.Login{}
	err := p.DB.Where(`id = ?`, id).First(&login).Error
	return login, err
}

// Save ...
func (p *LoginRepository) Save(login model.Login) (model.Login, error) {
	err := p.DB.Save(&login).Error
	return login, err
}

// Delete ...
func (p *LoginRepository) Delete(id uint) error {
	err := p.DB.Delete(&model.Login{ID: id}).Error
	return err
}

// Migrate ...
func (p *LoginRepository) Migrate() {
	err := p.DB.AutoMigrate(&model.Login{}).Error
	if err != nil {
		log.Println(err)
	}
}
