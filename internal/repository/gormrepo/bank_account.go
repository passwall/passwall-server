package gormrepo

import (
	"context"
	"errors"

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
	
	var account domain.BankAccount
	err := r.db.WithContext(ctx).Table(schema+".bank_accounts").Where("id = ?", id).First(&account).Error
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
	
	var accounts []*domain.BankAccount
	err := r.db.WithContext(ctx).Table(schema + ".bank_accounts").Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *bankAccountRepository) Create(ctx context.Context, account *domain.BankAccount) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".bank_accounts").Create(account).Error
}

func (r *bankAccountRepository) Update(ctx context.Context, account *domain.BankAccount) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".bank_accounts").Save(account).Error
}

func (r *bankAccountRepository) Delete(ctx context.Context, id uint) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".bank_accounts").Delete(&domain.BankAccount{ID: id}).Error
}

func (r *bankAccountRepository) Migrate(schema string) error {
	return r.db.Table(schema + ".bank_accounts").AutoMigrate(&domain.BankAccount{})
}

