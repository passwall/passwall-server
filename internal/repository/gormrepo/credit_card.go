package gormrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/database"
	"gorm.io/gorm"
)

type creditCardRepository struct {
	db *gorm.DB
}

// NewCreditCardRepository creates a new credit card repository
func NewCreditCardRepository(db *gorm.DB) repository.CreditCardRepository {
	return &creditCardRepository{db: db}
}

func (r *creditCardRepository) GetByID(ctx context.Context, id uint) (*domain.CreditCard, error) {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "credit_cards")
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	
	var card domain.CreditCard
	err = r.db.WithContext(ctx).Table(tableName).Where("id = ?", id).First(&card).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &card, nil
}

func (r *creditCardRepository) List(ctx context.Context) ([]*domain.CreditCard, error) {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "credit_cards")
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	
	var cards []*domain.CreditCard
	err = r.db.WithContext(ctx).Table(tableName).Find(&cards).Error
	if err != nil {
		return nil, err
	}
	return cards, nil
}

func (r *creditCardRepository) Create(ctx context.Context, card *domain.CreditCard) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "credit_cards")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Create(card).Error
}

func (r *creditCardRepository) Update(ctx context.Context, card *domain.CreditCard) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "credit_cards")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Save(card).Error
}

func (r *creditCardRepository) Delete(ctx context.Context, id uint) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "credit_cards")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Delete(&domain.CreditCard{ID: id}).Error
}

func (r *creditCardRepository) Migrate(schema string) error {
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "credit_cards")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.Table(tableName).AutoMigrate(&domain.CreditCard{})
}

