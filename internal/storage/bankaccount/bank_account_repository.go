package bankaccount

import (
	"github.com/jinzhu/gorm"
	"github.com/passwall/passwall-server/model"
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
	return bankAccounts, p.db.Table(schema + ".bank_accounts").Find(&bankAccounts).Error
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.BankAccount, error) {
	query := p.db.Table(schema + ".bank_accounts").Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("bank_name LIKE ?", "%"+argsStr["search"]+"%")

		for _, field := range []string{"bank_code", "account_name", "account_number", "iban", "currency"} {
			query = query.Or(field+" LIKE ?", "%"+argsStr["search"]+"%")
		}
	}

	bankAccounts := []model.BankAccount{}
	return bankAccounts, query.Find(&bankAccounts).Error
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.BankAccount, error) {
	bankAccount := new(model.BankAccount)
	return bankAccount, p.db.Table(schema+".bank_accounts").Where(`id = ?`, id).First(&bankAccount).Error
}

// Save ...
func (p *Repository) Save(bankAccount *model.BankAccount, schema string) (*model.BankAccount, error) {
	return bankAccount, p.db.Table(schema + ".bank_accounts").Save(&bankAccount).Error
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	return p.db.Table(schema + ".bank_accounts").Delete(&model.BankAccount{ID: id}).Error
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".bank_accounts").AutoMigrate(&model.BankAccount{}).Error
}
