package creditcard

import (
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/logger"
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
func (p *Repository) All(schema string) ([]model.CreditCard, error) {
	creditCards := []model.CreditCard{}
	err := p.db.Table(schema + ".credit_cards").Find(&creditCards).Error
	if err != nil {
		logger.Errorf("Error getting all credit cards error %v", err)
		return nil, err
	}
	return creditCards, err
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.CreditCard, error) {
	creditCard := new(model.CreditCard)
	err := p.db.Table(schema+".credit_cards").Where(`id = ?`, id).First(&creditCard).Error
	if err != nil {
		logger.Errorf("Error getting credit card by id %v error %v", id, err)
		return nil, err
	}
	return creditCard, err
}

// Update ...
func (p *Repository) Update(creditCard *model.CreditCard, schema string) (*model.CreditCard, error) {
	err := p.db.Table(schema + ".credit_cards").Save(&creditCard).Error
	if err != nil {
		logger.Errorf("Error updating credit card %v error %v", creditCard, err)
		return nil, err
	}

	return creditCard, nil
}

// Create ...
func (p *Repository) Create(creditCard *model.CreditCard, schema string) (*model.CreditCard, error) {
	err := p.db.Table(schema + ".credit_cards").Create(&creditCard).Error
	if err != nil {
		logger.Errorf("Error creating credit card %v error %v", creditCard, err)
		return nil, err
	}
	return creditCard, nil
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	err := p.db.Table(schema + ".credit_cards").Delete(&model.CreditCard{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".credit_cards").AutoMigrate(&model.CreditCard{})
}
