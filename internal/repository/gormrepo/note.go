package gormrepo

import (
	"context"
	"errors"

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
	
	var note domain.Note
	err := r.db.WithContext(ctx).Table(schema+".notes").Where("id = ?", id).First(&note).Error
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
	
	var notes []*domain.Note
	err := r.db.WithContext(ctx).Table(schema + ".notes").Find(&notes).Error
	if err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *noteRepository) Create(ctx context.Context, note *domain.Note) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".notes").Create(note).Error
}

func (r *noteRepository) Update(ctx context.Context, note *domain.Note) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".notes").Save(note).Error
}

func (r *noteRepository) Delete(ctx context.Context, id uint) error {
	schema := database.GetSchema(ctx)
	return r.db.WithContext(ctx).Table(schema + ".notes").Delete(&domain.Note{ID: id}).Error
}

func (r *noteRepository) Migrate(schema string) error {
	return r.db.Table(schema + ".notes").AutoMigrate(&domain.Note{})
}

