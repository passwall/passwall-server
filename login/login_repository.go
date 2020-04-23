package login

import (
	"log"

	"github.com/jinzhu/gorm"
)

// LoginRepository ...
type LoginRepository struct {
	DB *gorm.DB
}

// NewLoginRepository ...
func NewLoginRepository(db *gorm.DB) LoginRepository {
	return LoginRepository{DB: db}
}

// FindAll ...
func (p *LoginRepository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]Login, error) {
	logins := []Login{}

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

// All ...
func (p *LoginRepository) All() ([]Login, error) {
	logins := []Login{}
	err := p.DB.Find(&logins).Error
	return logins, err
}

// FindByID ...
func (p *LoginRepository) FindByID(id uint) (Login, error) {
	login := Login{}
	err := p.DB.Where(`id = ?`, id).First(&login).Error
	return login, err
}

// Save ...
func (p *LoginRepository) Save(login Login) (Login, error) {
	err := p.DB.Save(&login).Error
	return login, err
}

// Delete ...
func (p *LoginRepository) Delete(id uint) error {
	err := p.DB.Delete(&Login{ID: id}).Error
	return err
}

// Migrate ...
func (p *LoginRepository) Migrate() {
	err := p.DB.AutoMigrate(&Login{}).Error
	if err != nil {
		log.Println(err)
	}
}
