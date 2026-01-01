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

type serverRepository struct {
	db *gorm.DB
}

// NewServerRepository creates a new server repository
func NewServerRepository(db *gorm.DB) repository.ServerRepository {
	return &serverRepository{db: db}
}

func (r *serverRepository) GetByID(ctx context.Context, id uint) (*domain.Server, error) {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "servers")
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	
	var server domain.Server
	err = r.db.WithContext(ctx).Table(tableName).Where("id = ?", id).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &server, nil
}

func (r *serverRepository) List(ctx context.Context) ([]*domain.Server, error) {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "servers")
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	
	var servers []*domain.Server
	err = r.db.WithContext(ctx).Table(tableName).Find(&servers).Error
	if err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *serverRepository) Create(ctx context.Context, server *domain.Server) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "servers")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Create(server).Error
}

func (r *serverRepository) Update(ctx context.Context, server *domain.Server) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "servers")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Save(server).Error
}

func (r *serverRepository) Delete(ctx context.Context, id uint) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "servers")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Delete(&domain.Server{ID: id}).Error
}

func (r *serverRepository) Migrate(schema string) error {
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "servers")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.Table(tableName).AutoMigrate(&domain.Server{})
}

