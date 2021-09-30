package user

import (
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/logger"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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

// FindByUUID ...
func (p *Repository) FindByUUID(uuid string) (*model.User, error) {
	user := new(model.User)
	err := p.db.Where(`uuid = ?`, uuid).First(&user).Error
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
	err := p.db.Where(`email = ?`, email).First(&user).Error
	if err != nil {
		return user, err
	}

	// Comparing the password with the bcrypt hash
	err = bcrypt.CompareHashAndPassword([]byte(user.MasterPassword), []byte(masterPassword))
	if err != nil {
		return user, err
	}

	return user, nil
}

// Save ...
func (p *Repository) Save(user *model.User) (*model.User, error) {
	err := p.db.Save(&user).Error
	if err != nil {
		logger.Errorf("Error saving user %v error %v", user, err)
		return nil, err
	}

	return user, nil
}

// Update ...
func (p *Repository) Update(user *model.User) (*model.User, error) {
	err := p.db.Save(&user).Error
	if err != nil {
		logger.Errorf("Error saving user %v error %v", user, err)
		return nil, err
	}

	return user, nil
}

// Create ...
func (p *Repository) Create(user *model.User) (*model.User, error) {
	err := p.db.Create(&user).Error
	if err != nil {
		logger.Errorf("Error saving user %v error %v", user, err)
		return nil, err
	}

	return user, nil
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {

	err := p.db.Exec("DROP SCHEMA " + schema + " CASCADE").Error
	if err != nil {
		logger.Errorf("Error deleting schema %s error %v", schema, err)
	}

	err = p.db.Delete(&model.User{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.User{})
}

// CreateSchema ...
func (p *Repository) CreateSchema(schema string) error {
	var err error
	if schema != "" && schema != "public" {
		err := p.db.Exec("CREATE SCHEMA IF NOT EXISTS " + schema).Error
		if err != nil {
			logger.Errorf("Error creating schema %s error %v", schema, err)
		}
	}
	return err
}
