package subscription

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
func (p *Repository) All() ([]model.Subscription, error) {
	subscriptions := []model.Subscription{}
	return subscriptions, p.db.Find(&subscriptions).Error
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Subscription, error) {
	query := p.db.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("title LIKE ? OR ip LIKE ?", "%"+argsStr["search"]+"%", "%"+argsStr["search"]+"%")
	}

	subscriptions := []model.Subscription{}
	return subscriptions, query.Find(&subscriptions).Error
}

// FindByID ...
func (p *Repository) FindByID(id uint) (*model.Subscription, error) {
	subscription := new(model.Subscription)
	return subscription, p.db.Where(`id = ?`, id).First(&subscription).Error
}

// FindByEmail ...
func (p *Repository) FindByEmail(email string) (*model.Subscription, error) {
	subscription := new(model.Subscription)
	return subscription, p.db.Where(`email = ?`, email).First(&subscription).Error
}

// FindBySubscriptionID ...
func (p *Repository) FindBySubscriptionID(id uint) (*model.Subscription, error) {
	subscription := new(model.Subscription)
	return subscription, p.db.Where(`subscription_id = ?`, id).First(&subscription).Error
}

// Save ...
func (p *Repository) Save(subscription *model.Subscription) (*model.Subscription, error) {
	return subscription, p.db.Save(&subscription).Error
}

// Delete ...
func (p *Repository) Delete(id uint) error {
	return p.db.Delete(&model.Subscription{ID: id}).Error
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.Subscription{}).Error
}
