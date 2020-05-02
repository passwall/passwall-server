package storage

import (
	"log"

	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/model"
)

// BankAccountRepository ...
type BankAccountRepository struct {
	DB *gorm.DB
}

// NewBankAccountRepository ...
func NewBankAccountRepository(db *gorm.DB) BankAccountRepository {
	return BankAccountRepository{DB: db}
}

// All ...
func (p *BankAccountRepository) All() ([]model.BankAccount, error) {
	bankAccounts := []model.BankAccount{}
	err := p.DB.Find(&bankAccounts).Error
	return bankAccounts, err
}

// FindAll ...
func (p *BankAccountRepository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.BankAccount, error) {
	bankAccounts := []model.BankAccount{}

	query := p.DB
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
func (p *BankAccountRepository) FindByID(id uint) (model.BankAccount, error) {
	bankAccount := model.BankAccount{}
	err := p.DB.Where(`id = ?`, id).First(&bankAccount).Error
	return bankAccount, err
}

// Save ...
func (p *BankAccountRepository) Save(bankAccount model.BankAccount) (model.BankAccount, error) {
	err := p.DB.Save(&bankAccount).Error
	return bankAccount, err
}

// Delete ...
func (p *BankAccountRepository) Delete(id uint) error {
	err := p.DB.Delete(&model.BankAccount{ID: id}).Error
	return err
}

// Migrate ...
func (p *BankAccountRepository) Migrate() {
	err := p.DB.AutoMigrate(&model.BankAccount{}).Error
	if err != nil {
		log.Println(err)
	}
}
