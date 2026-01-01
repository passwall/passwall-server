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

type noteRepository struct {
	db *gorm.DB
}

// NewNoteRepository creates a new note repository
func NewNoteRepository(db *gorm.DB) repository.NoteRepository {
	return &noteRepository{db: db}
}

func (r *noteRepository) GetByID(ctx context.Context, id uint) (*domain.Note, error) {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "notes")
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	
	var note domain.Note
	err = r.db.WithContext(ctx).Table(tableName).Where("id = ?", id).First(&note).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &note, nil
}

func (r *noteRepository) List(ctx context.Context) ([]*domain.Note, error) {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "notes")
	if err != nil {
		return nil, fmt.Errorf("invalid table name: %w", err)
	}
	
	var notes []*domain.Note
	err = r.db.WithContext(ctx).Table(tableName).Find(&notes).Error
	if err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *noteRepository) Create(ctx context.Context, note *domain.Note) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "notes")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Create(note).Error
}

func (r *noteRepository) Update(ctx context.Context, note *domain.Note) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "notes")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Save(note).Error
}

func (r *noteRepository) Delete(ctx context.Context, id uint) error {
	schema := database.GetSchema(ctx)
	
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "notes")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.WithContext(ctx).Table(tableName).Delete(&domain.Note{ID: id}).Error
}

func (r *noteRepository) Migrate(schema string) error {
	// Build safe qualified table name
	tableName, err := database.BuildQualifiedTableName(schema, "notes")
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	
	return r.db.Table(tableName).AutoMigrate(&domain.Note{})
}

