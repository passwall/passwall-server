package login

import (
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
func (p *LoginRepository) FindAll() ([]Login, error) {
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
	CheckErr(err)
}

// func (p *Repository) List(offset, limit int) ([]*Login, error) {
// 	var l []*Login
// 	err := p.DB.Offset(offset).Limit(limit).Find(&l).Error
// 	return l, err
// }
