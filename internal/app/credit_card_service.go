package app

import (
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
)

// CreditCardService ...
type CreditCardService struct {
	CreditCardRepository storage.CreditCardRepository
}

// NewCreditCardService ...
func NewCreditCardService(p storage.CreditCardRepository) CreditCardService {
	return CreditCardService{CreditCardRepository: p}
}

// All ...
func (p *CreditCardService) All() ([]model.CreditCard, error) {
	return p.CreditCardRepository.All()
}

// FindAll ...
func (p *CreditCardService) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.CreditCard, error) {
	return p.CreditCardRepository.FindAll(argsStr, argsInt)
}

// FindByID ...
func (p *CreditCardService) FindByID(id uint) (model.CreditCard, error) {
	return p.CreditCardRepository.FindByID(id)
}

// Save ...
func (p *CreditCardService) Save(creditCard model.CreditCard) (model.CreditCard, error) {
	return p.CreditCardRepository.Save(creditCard)
}

// Delete ...
func (p *CreditCardService) Delete(id uint) error {
	return p.CreditCardRepository.Delete(id)
}

// Migrate ...
func (p *CreditCardService) Migrate() {
	p.CreditCardRepository.Migrate()
}
