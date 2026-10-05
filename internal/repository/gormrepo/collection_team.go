package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type collectionTeamRepository struct {
	db *gorm.DB
}

// NewCollectionTeamRepository creates a new collection team repository
func NewCollectionTeamRepository(db *gorm.DB) repository.CollectionTeamRepository {
	return &collectionTeamRepository{db: db}
}

func (r *collectionTeamRepository) Create(ctx context.Context, ct *domain.CollectionTeam) error {
	return dbFromContext(ctx, r.db).Create(ct).Error
}

func (r *collectionTeamRepository) GetByID(ctx context.Context, id uint) (*domain.CollectionTeam, error) {
	var ct domain.CollectionTeam
	err := dbFromContext(ctx, r.db).
		Preload("Collection").
		Preload("Team").
		Where("id = ?", id).
		First(&ct).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &ct, nil
}

func (r *collectionTeamRepository) GetByCollectionAndTeam(ctx context.Context, collectionID, teamID uint) (*domain.CollectionTeam, error) {
	var ct domain.CollectionTeam
	err := dbFromContext(ctx, r.db).
		Preload("Collection").
		Preload("Team").
		Where("collection_id = ? AND team_id = ?", collectionID, teamID).
		First(&ct).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &ct, nil
}

func (r *collectionTeamRepository) ListByCollection(ctx context.Context, collectionID uint) ([]*domain.CollectionTeam, error) {
	var cts []*domain.CollectionTeam
	err := dbFromContext(ctx, r.db).
		Preload("Team").
		Where("collection_id = ?", collectionID).
		Order("created_at ASC").
		Find(&cts).Error

	if err != nil {
		return nil, err
	}
	return cts, nil
}

func (r *collectionTeamRepository) ListByTeam(ctx context.Context, teamID uint) ([]*domain.CollectionTeam, error) {
	var cts []*domain.CollectionTeam
	err := dbFromContext(ctx, r.db).
		Preload("Collection").
		Where("team_id = ?", teamID).
		Order("created_at ASC").
		Find(&cts).Error

	if err != nil {
		return nil, err
	}
	return cts, nil
}

func (r *collectionTeamRepository) Update(ctx context.Context, ct *domain.CollectionTeam) error {
	// Clear associations
	ct.Collection = nil
	ct.Team = nil

	return dbFromContext(ctx, r.db).Save(ct).Error
}

func (r *collectionTeamRepository) Delete(ctx context.Context, id uint) error {
	return dbFromContext(ctx, r.db).Delete(&domain.CollectionTeam{}, id).Error
}

func (r *collectionTeamRepository) DeleteByCollectionAndTeam(ctx context.Context, collectionID, teamID uint) error {
	return dbFromContext(ctx, r.db).
		Where("collection_id = ? AND team_id = ?", collectionID, teamID).
		Delete(&domain.CollectionTeam{}).Error
}
