package user

import (
	"log"

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
func (p *Repository) All() ([]model.User, error) {
	users := []model.User{}
	err := p.db.Find(&users).Error
	return users, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.User, error) {
	users := []model.User{}

	query := p.db
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("name LIKE ? OR email LIKE ? OR plan LIKE ? OR role LIKE ?",
			"%"+argsStr["search"]+"%",
			"%"+argsStr["search"]+"%",
			"%"+argsStr["search"]+"%",
			"%"+argsStr["search"]+"%")
	}

	err := query.Find(&users).Error
	return users, err
}

// FindByID ...
func (p *Repository) FindByID(id uint) (*model.User, error) {
	user := new(model.User)
	err := p.db.Where(`id = ?`, id).First(&user).Error
	return user, err
}

// FindByEmail ...
func (p *Repository) FindByEmail(email string) (*model.User, error) {
	user := new(model.User)
	err := p.db.Where(`email = ?`, email).First(&user).Error
	return user, err
}

// FindByCredentials ...
func (p *Repository) FindByCredentials(email, masterPassword string) (*model.User, error) {
	user := new(model.User)
	err := p.db.Where(`email = ? AND master_password = ?`, email, masterPassword).First(&user).Error
	return user, err
}

// Save ...
func (p *Repository) Save(user *model.User) (*model.User, error) {
	err := p.db.Save(&user).Error
	return user, err
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {

	err := p.db.Exec("DROP SCHEMA " + schema + " CASCADE").Error
	if err != nil {
		log.Println(err)
	}

	err = p.db.Delete(&model.User{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {

	if schema != "public" {
		err := p.db.Exec("CREATE SCHEMA IF NOT EXISTS " + schema).Error
		if err != nil {
			log.Println(err)
		}

		err = p.db.Table(schema + ".logins").AutoMigrate(&model.Login{}).Error
		if err != nil {
			log.Println(err)
		}
	}

	return p.db.AutoMigrate(&model.User{}).Error
}
