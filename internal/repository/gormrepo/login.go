package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/database"
	"gorm.io/gorm"
)

type loginRepository struct {
	db *gorm.DB
}

// NewLoginRepository creates a new login repository
func NewLoginRepository(db *gorm.DB) repository.LoginRepository {
	return &loginRepository{db: db}
}

func (r *loginRepository) GetByID(ctx context.Context, id uint) (*domain.Login, error) {
	schema := database.GetSchema(ctx)
	
	var login domain.Login
	err := r.db.WithContext(ctx).Table(schema+".logins").Where("id = ?", id).First(&login).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &login, nil
}

func (r *loginRepository) List(ctx context.Context) ([]*domain.Login, error) {
	schema := database.GetSchema(ctx)
	
	var logins []*domain.Login
	err := r.db.WithContext(ctx).Table(schema + ".logins").Find(&logins).Error
	if err != nil {
		return nil, err
	}
	return logins, nil
}

func (r *loginRepository) Create(ctx context.Context, login *domain.Login) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".logins").Create(login).Error
}

func (r *loginRepository) Update(ctx context.Context, login *domain.Login) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".logins").Save(login).Error
}

func (r *loginRepository) Delete(ctx context.Context, id uint) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".logins").Delete(&domain.Login{ID: id}).Error
}

func (r *loginRepository) Migrate(schema string) error {
	return r.db.Table(schema + ".logins").AutoMigrate(&domain.Login{})
}

