package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/database"
	"gorm.io/gorm"
)

type emailRepository struct {
	db *gorm.DB
}

// NewEmailRepository creates a new email repository
func NewEmailRepository(db *gorm.DB) repository.EmailRepository {
	return &emailRepository{db: db}
}

func (r *emailRepository) GetByID(ctx context.Context, id uint) (*domain.Email, error) {
	schema := database.GetSchema(ctx)
	
	var email domain.Email
	err := r.db.WithContext(ctx).Table(schema+".emails").Where("id = ?", id).First(&email).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &email, nil
}

func (r *emailRepository) List(ctx context.Context) ([]*domain.Email, error) {
	schema := database.GetSchema(ctx)
	
	var emails []*domain.Email
	err := r.db.WithContext(ctx).Table(schema + ".emails").Find(&emails).Error
	if err != nil {
		return nil, err
	}
	return emails, nil
}

func (r *emailRepository) Create(ctx context.Context, email *domain.Email) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".emails").Create(email).Error
}

func (r *emailRepository) Update(ctx context.Context, email *domain.Email) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".emails").Save(email).Error
}

func (r *emailRepository) Delete(ctx context.Context, id uint) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".emails").Delete(&domain.Email{ID: id}).Error
}

func (r *emailRepository) Migrate(schema string) error {
	return r.db.Table(schema + ".emails").AutoMigrate(&domain.Email{})
}

