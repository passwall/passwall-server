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
	err := p.db.Find(&subscriptions).Error
	return subscriptions, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int) ([]model.Subscription, error) {
	subscriptions := []model.Subscription{}

	query := p.db
	query = query.Limit(argsInt["limit"])
	if argsInt["limit"] > 0 {
		// offset can't be declared without a valid limit
		query = query.Offset(argsInt["offset"])
	}

	query = query.Order(argsStr["order"])

	if argsStr["search"] != "" {
		query = query.Where("title LIKE ? OR ip LIKE ?", "%"+argsStr["search"]+"%", "%"+argsStr["search"]+"%")
	}

	err := query.Find(&subscriptions).Error
	return subscriptions, err
}

// FindByID ...
func (p *Repository) FindByID(id uint) (*model.Subscription, error) {
	subscription := new(model.Subscription)
	err := p.db.Where(`id = ?`, id).First(&subscription).Error
	return subscription, err
}

// FindBySubscriptionID ...
func (p *Repository) FindBySubscriptionID(id uint) (*model.Subscription, error) {
	subscription := new(model.Subscription)
	err := p.db.Where(`subscription_id = ?`, id).First(&subscription).Error
	return subscription, err
}

// Save ...
func (p *Repository) Save(subscription *model.Subscription) (*model.Subscription, error) {
	err := p.db.Save(&subscription).Error
	return subscription, err
}

// Delete ...
func (p *Repository) Delete(id uint) error {
	err := p.db.Delete(&model.Subscription{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.Subscription{}).Error
}
