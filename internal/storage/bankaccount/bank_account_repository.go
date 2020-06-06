package bankaccount

import (
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
func (p *Repository) All() ([]model.BankAccount, error) {
	bankAccounts := []model.BankAccount{}
	err := p.db.Find(&bankAccounts).Error
	return bankAccounts, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.BankAccount, error) {
	bankAccounts := []model.BankAccount{}

	query := p.db
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
func (p *Repository) FindByID(id uint) (model.BankAccount, error) {
	bankAccount := model.BankAccount{}
	err := p.db.Where(`id = ?`, id).First(&bankAccount).Error
	return bankAccount, err
}

// Save ...
func (p *Repository) Save(bankAccount model.BankAccount) (model.BankAccount, error) {
	err := p.db.Save(&bankAccount).Error
	return bankAccount, err
}

// Delete ...
func (p *Repository) Delete(id uint) error {
	err := p.db.Delete(&model.BankAccount{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".bank_accounts").AutoMigrate(&model.BankAccount{}).Error
}
