package creditcard

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
func (p *Repository) All(schema string) ([]model.CreditCard, error) {
	creditCards := []model.CreditCard{}
	return creditCards, p.db.Table(schema + ".credit_cards").Find(&creditCards).Error
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.CreditCard, error) {
	query := p.db.Table(schema + ".credit_cards").Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("card_name LIKE ?", "%"+argsStr["search"]+"%")

		for _, field := range []string{"cardholder_name", "type", "number", "verification_number", "expiry_date"} {
			query = query.Or(field+" LIKE ?", "%"+argsStr["search"]+"%")
		}
	}

	creditCards := []model.CreditCard{}
	return creditCards, query.Find(&creditCards).Error
}

// FindByID ...
func (p *Repository) FindByID(id uint, schema string) (*model.CreditCard, error) {
	creditCard := new(model.CreditCard)
	return creditCard, p.db.Table(schema+".credit_cards").Where(`id = ?`, id).First(&creditCard).Error
}

// Save ...
func (p *Repository) Save(creditCard *model.CreditCard, schema string) (*model.CreditCard, error) {
	return creditCard, p.db.Table(schema + ".credit_cards").Save(&creditCard).Error
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	return p.db.Table(schema + ".credit_cards").Delete(&model.CreditCard{ID: id}).Error
}

// Migrate ...
func (p *Repository) Migrate(schema string) error {
	return p.db.Table(schema + ".credit_cards").AutoMigrate(&model.CreditCard{}).Error
}
