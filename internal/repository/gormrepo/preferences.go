package gormrepo

import (
	"context"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type preferencesRepository struct {
	db *gorm.DB
}

func NewPreferencesRepository(db *gorm.DB) repository.PreferencesRepository {
	return &preferencesRepository{db: db}
}

func (r *preferencesRepository) ListByOwner(ctx context.Context, ownerType string, ownerID uint, section string) ([]*domain.Preference, error) {
	var prefs []*domain.Preference

	q := r.db.WithContext(ctx).Where("owner_type = ? AND owner_id = ?", strings.ToLower(ownerType), ownerID)
	if section != "" {
		q = q.Where("section = ?", strings.ToLower(section))
	}

	if err := q.Order("section ASC, key ASC").Find(&prefs).Error; err != nil {
		return nil, err
	}

	return prefs, nil
}

func (r *preferencesRepository) UpsertMany(ctx context.Context, prefs []*domain.Preference) error {
	if len(prefs) == 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, p := range prefs {
		if p == nil {
			return repository.ErrInvalidInput
		}
		p.UpdatedAt = now
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "owner_type"},
			{Name: "owner_id"},
			{Name: "section"},
			{Name: "key"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"value",
			"type",
			"updated_at",
		}),
	}).Create(&prefs).Error
}
