package model

import (
	"github.com/jinzhu/gorm"
)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

type Repository struct {
	DB *gorm.DB
}

func (p *Repository) Load(id uint) (*Login, error) {
	login := &Login{}
	err := p.DB.Unscoped().Where(`id = ?`, id).First(login).Error
	return login, err
}

func (p *Repository) ListAll() ([]Login, error) {
	var l []Login
	err := p.DB.Find(&l).Error
	return l, err
}

func (p *Repository) List(offset, limit int) ([]*Login, error) {
	var l []*Login
	err := p.DB.Offset(offset).Limit(limit).Find(&l).Error
	return l, err
}

func (p *Repository) Save(login *Login) error {
	return p.DB.Save(login).Error
}

// func (p *Repository) Delete(id uint) error {
// 	return p.db.Delete(&Login{gorm.Model: id}).Error
// }

func (p *Repository) Migrate() error {
	return p.DB.AutoMigrate(&Login{}).Error
}
