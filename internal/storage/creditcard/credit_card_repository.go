package creditcard

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
func (p *Repository) All() ([]model.CreditCard, error) {
	creditCards := []model.CreditCard{}
	err := p.db.Find(&creditCards).Error
	return creditCards, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.CreditCard, error) {
	creditCards := []model.CreditCard{}

	query := p.db
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
func (p *Repository) FindByID(id uint) (model.CreditCard, error) {
	creditCard := model.CreditCard{}
	err := p.db.Where(`id = ?`, id).First(&creditCard).Error
	return creditCard, err
}

// Save ...
func (p *Repository) Save(creditCard model.CreditCard) (model.CreditCard, error) {
	err := p.db.Save(&creditCard).Error
	return creditCard, err
}

// Delete ...
func (p *Repository) Delete(id uint) error {
	err := p.db.Delete(&model.CreditCard{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.CreditCard{}).Error
}
