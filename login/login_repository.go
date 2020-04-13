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
func (p *LoginRepository) FindAll() []Login {
	logins := []Login{}
	err := p.DB.Find(&logins).Error
	CheckErr(err)
	return logins
}

// FindByID ...
func (p *LoginRepository) FindByID(id uint) Login {
	login := Login{}
	err := p.DB.First(&login, id).Error
	CheckErr(err)
	return login
}

// Save ...
func (p *LoginRepository) Save(login Login) Login {
	err := p.DB.Save(&login).Error
	CheckErr(err)
	return login
}

// Delete ...
func (p *LoginRepository) Delete(id uint) {
	err := p.DB.Delete(&Login{ID: id}).Error
	CheckErr(err)
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
