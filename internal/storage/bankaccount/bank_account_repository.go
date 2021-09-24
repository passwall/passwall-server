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
	return bankAccounts, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.BankAccount, error) {
	bankAccounts := []model.BankAccount{}

	query := p.db
	query = query.Table(schema + ".bank_accounts")
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("bank_name LIKE ?", "%"+argsStr["search"]+"%")

		fields := []string{"bank_code", "account_name", "account_number", "iban", "currency"}
		for i := range fields {
			query = query.Or(fields[i]+" LIKE ?", "%"+argsStr["search"]+"%")
		}
	}

	err := query.Find(&bankAccounts).Error
	return bankAccounts, err
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.BankAccount, error) {
	bankAccount := new(model.BankAccount)
	err := p.db.Table(schema+".bank_accounts").Where(`id = ?`, id).First(&bankAccount).Error
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
