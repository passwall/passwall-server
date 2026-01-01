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

type bankAccountRepository struct {
	db *gorm.DB
}

// NewBankAccountRepository creates a new bank account repository
func NewBankAccountRepository(db *gorm.DB) repository.BankAccountRepository {
	return &bankAccountRepository{db: db}
}

func (r *bankAccountRepository) GetByID(ctx context.Context, id uint) (*domain.BankAccount, error) {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "bank_accounts")
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	
	var account domain.BankAccount
	err = r.db.WithContext(ctx).Table(tableName).Where("id = ?", id).First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &account, nil
}

func (r *bankAccountRepository) List(ctx context.Context) ([]*domain.BankAccount, error) {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "bank_accounts")
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	
	var accounts []*domain.BankAccount
	err = r.db.WithContext(ctx).Table(tableName).Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *bankAccountRepository) Create(ctx context.Context, account *domain.BankAccount) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "bank_accounts")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Create(account).Error
}

func (r *bankAccountRepository) Update(ctx context.Context, account *domain.BankAccount) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "bank_accounts")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Save(account).Error
}

func (r *bankAccountRepository) Delete(ctx context.Context, id uint) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "bank_accounts")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Delete(&domain.BankAccount{ID: id}).Error
}

func (r *bankAccountRepository) Migrate(schema string) error {
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "bank_accounts")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.Table(tableName).AutoMigrate(&domain.BankAccount{})
}

