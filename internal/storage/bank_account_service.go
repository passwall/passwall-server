package storage

import "github.com/pass-wall/passwall-server/model"

// BankAccountService ...
type BankAccountService struct {
	BankAccountRepository BankAccountRepository
}

// NewBankAccountService ...
func NewBankAccountService(p BankAccountRepository) BankAccountService {
	return BankAccountService{BankAccountRepository: p}
}

// All ...
func (p *BankAccountService) All() ([]model.BankAccount, error) {
	return p.BankAccountRepository.All()
}

// FindAll ...
func (p *BankAccountService) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.BankAccount, error) {
	return p.BankAccountRepository.FindAll(argsStr, argsInt)
}

// FindByID ...
func (p *BankAccountService) FindByID(id uint) (model.BankAccount, error) {
	return p.BankAccountRepository.FindByID(id)
}

// Save ...
func (p *BankAccountService) Save(bankAccount model.BankAccount) (model.BankAccount, error) {
	return p.BankAccountRepository.Save(bankAccount)
}

// Delete ...
func (p *BankAccountService) Delete(id uint) error {
	return p.BankAccountRepository.Delete(id)
}

// Migrate ...
func (p *BankAccountService) Migrate() {
	p.BankAccountRepository.Migrate()
}
