package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type teamRepository struct {
	db *gorm.DB
}

// NewTeamRepository creates a new team repository
func NewTeamRepository(db *gorm.DB) repository.TeamRepository {
	return &teamRepository{db: db}
}

func (r *teamRepository) Create(ctx context.Context, team *domain.Team) error {
	// Generate UUID if not set
	if team.UUID == uuid.Nil {
		team.UUID = uuid.NewV4()
	}

	return r.db.WithContext(ctx).Create(team).Error
}

func (r *teamRepository) GetByID(ctx context.Context, id uint) (*domain.Team, error) {
	var team domain.Team
	err := r.db.WithContext(ctx).
		Preload("Organization").
		Where("id = ?", id).
		First(&team).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &team, nil
}

func (r *teamRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.Team, error) {
	var team domain.Team
	err := r.db.WithContext(ctx).
		Preload("Organization").
		Where("uuid = ?", uuidStr).
		First(&team).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &team, nil
}

func (r *teamRepository) GetByName(ctx context.Context, orgID uint, name string) (*domain.Team, error) {
	var team domain.Team
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND name = ?", orgID, name).
		First(&team).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &team, nil
}

func (r *teamRepository) GetDefaultByOrganization(ctx context.Context, orgID uint) (*domain.Team, error) {
	var team domain.Team
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND is_default = true", orgID).
		First(&team).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &team, nil
}

func (r *teamRepository) ListByOrganization(ctx context.Context, orgID uint) ([]*domain.Team, error) {
	var teams []*domain.Team
	err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("name ASC").
		Find(&teams).Error

	if err != nil {
		return nil, err
	}
	return teams, nil
}

func (r *teamRepository) Update(ctx context.Context, team *domain.Team) error {
	// Clear associations
	team.Organization = nil
	team.Members = nil

	return r.db.WithContext(ctx).Save(team).Error
}

func (r *teamRepository) Delete(ctx context.Context, id uint) error {
	// Hard delete - will cascade delete team_users and collection_teams
	return r.db.WithContext(ctx).Unscoped().Delete(&domain.Team{}, id).Error
}

func (r *teamRepository) GetMemberCount(ctx context.Context, teamID uint) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.TeamUser{}).
		Where("team_id = ?", teamID).
		Count(&count).Error

	return int(count), err
}
