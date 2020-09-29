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
func (p *Repository) All(schema string) ([]model.Subscription, error) {
	subscriptions := []model.Subscription{}
	err := p.db.Table(schema + ".subscriptions").Find(&subscriptions).Error
	return subscriptions, err
}

// FindAll ...
func (p *Repository) FindAll(argsStr map[string]string, argsInt map[string]int, schema string) ([]model.Subscription, error) {
	subscriptions := []model.Subscription{}

	query := p.db
	query = query.Table(schema + ".subscriptions")
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
func (p *Repository) FindByID(id uint, schema string) (*model.Subscription, error) {
	subscription := new(model.Subscription)
	err := p.db.Table(schema+".subscriptions").Where(`id = ?`, id).First(&subscription).Error
	return subscription, err
}

// Save ...
func (p *Repository) Save(subscription *model.Subscription, schema string) (*model.Subscription, error) {
	err := p.db.Table(schema + ".subscriptions").Save(&subscription).Error
	return subscription, err
}

// Delete ...
func (p *Repository) Delete(id uint, schema string) error {
	err := p.db.Table(schema + ".subscriptions").Delete(&model.Subscription{ID: id}).Error
	return err
}

// Migrate ...
func (p *Repository) Migrate() error {
	return p.db.AutoMigrate(&model.Subscription{}).Error
}
