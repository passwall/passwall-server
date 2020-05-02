package storage

import (
	"log"

	"github.com/jinzhu/gorm"
	"github.com/pass-wall/passwall-server/model"
)

// CreditCardRepository ...
type CreditCardRepository struct {
	DB *gorm.DB
}

// NewCreditCardRepository ...
func NewCreditCardRepository(db *gorm.DB) CreditCardRepository {
	return CreditCardRepository{DB: db}
}

// All ...
func (p *CreditCardRepository) All() ([]model.CreditCard, error) {
	creditCards := []model.CreditCard{}
	err := p.DB.Find(&creditCards).Error
	return creditCards, err
}

// FindAll ...
func (p *CreditCardRepository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.CreditCard, error) {
	creditCards := []model.CreditCard{}

	query := p.DB
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("card_name LIKE ?", "%"+argsStr["search"]+"%")

		fields := []string{"cardholder_name", "type", "number", "verification_number", "expiry_date"}
		for i := range fields {
			query = query.Or(fields[i]+" LIKE ?", "%"+argsStr["search"]+"%")
		}
	}

	err := query.Find(&creditCards).Error
	return creditCards, err
}

// FindByID ...
func (p *CreditCardRepository) FindByID(id uint) (model.CreditCard, error) {
	creditCard := model.CreditCard{}
	err := p.DB.Where(`id = ?`, id).First(&creditCard).Error
	return creditCard, err
}

// Save ...
func (p *CreditCardRepository) Save(creditCard model.CreditCard) (model.CreditCard, error) {
	err := p.DB.Save(&creditCard).Error
	return creditCard, err
}

// Delete ...
func (p *CreditCardRepository) Delete(id uint) error {
	err := p.DB.Delete(&model.CreditCard{ID: id}).Error
	return err
}

// Migrate ...
func (p *CreditCardRepository) Migrate() {
	err := p.DB.AutoMigrate(&model.CreditCard{}).Error
	if err != nil {
		log.Println(err)
	}
}
