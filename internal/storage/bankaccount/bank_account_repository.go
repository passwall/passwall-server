package bankaccount

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
func (p *Repository) All(schema string) ([]model.BankAccount, error) {
	bankAccounts := []model.BankAccount{}
	err := p.db.Table(schema + ".bank_accounts").Find(&bankAccounts).Error
	if err != nil {
		logger.Errorf("Error finding all bank accounts error %v", err)
		return nil, err
	}
	return bankAccounts, err
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.BankAccount, error) {
	bankAccount := new(model.BankAccount)
	err := p.db.Table(schema+".bank_accounts").Where(`id = ?`, id).First(&bankAccount).Error
	if err != nil {
		logger.Errorf("Error finding bank account %v error %v", bankAccount, err)
		return nil, err
	}
	return bankAccount, err
}

// Update ...
func (p *Repository) Update(bankAccount *model.BankAccount, schema string) (*model.BankAccount, error) {
	err := p.db.Table(schema + ".bank_accounts").Save(&bankAccount).Error
	if err != nil {
		logger.Errorf("Error updating bank account %v error %v", bankAccount, err)
		return nil, err
	}

	return bankAccount, nil
}

// Create ...
func (p *Repository) Create(bankAccount *model.BankAccount, schema string) (*model.BankAccount, error) {
	err := p.db.Table(schema + ".bank_accounts").Create(&bankAccount).Error
	if err != nil {
		logger.Errorf("Error creating bank account %v error %v", bankAccount, err)
		return nil, err
	}
	return bankAccount, nil
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	err := p.db.Table(schema + ".bank_accounts").Delete(&model.BankAccount{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".bank_accounts").AutoMigrate(&model.BankAccount{})
}
