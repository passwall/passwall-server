package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type teamUserRepository struct {
	db *gorm.DB
}

// NewTeamUserRepository creates a new team user repository
func NewTeamUserRepository(db *gorm.DB) repository.TeamUserRepository {
	return &teamUserRepository{db: db}
}

func (r *teamUserRepository) Create(ctx context.Context, teamUser *domain.TeamUser) error {
	return r.db.WithContext(ctx).Create(teamUser).Error
}

func (r *teamUserRepository) GetByID(ctx context.Context, id uint) (*domain.TeamUser, error) {
	var teamUser domain.TeamUser
	err := r.db.WithContext(ctx).
		Preload("Team").
		Preload("OrganizationUser").
		Preload("OrganizationUser.User").
		Where("id = ?", id).
		First(&teamUser).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &teamUser, nil
}

func (r *teamUserRepository) GetByTeamAndOrgUser(ctx context.Context, teamID, orgUserID uint) (*domain.TeamUser, error) {
	var teamUser domain.TeamUser
	err := r.db.WithContext(ctx).
		Preload("Team").
		Preload("OrganizationUser").
		Preload("OrganizationUser.User").
		Where("team_id = ? AND organization_user_id = ?", teamID, orgUserID).
		First(&teamUser).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &teamUser, nil
}

func (r *teamUserRepository) ListByTeam(ctx context.Context, teamID uint) ([]*domain.TeamUser, error) {
	var teamUsers []*domain.TeamUser
	err := r.db.WithContext(ctx).
		Preload("OrganizationUser").
		Preload("OrganizationUser.User").
		Where("team_id = ?", teamID).
		Order("is_manager DESC, created_at ASC").
		Find(&teamUsers).Error

	if err != nil {
		return nil, err
	}
	return teamUsers, nil
}

func (r *teamUserRepository) ListByOrgUser(ctx context.Context, orgUserID uint) ([]*domain.TeamUser, error) {
	var teamUsers []*domain.TeamUser
	err := r.db.WithContext(ctx).
		Preload("Team").
		Where("organization_user_id = ?", orgUserID).
		Order("created_at ASC").
		Find(&teamUsers).Error

	if err != nil {
		return nil, err
	}
	return teamUsers, nil
}

func (r *teamUserRepository) Update(ctx context.Context, teamUser *domain.TeamUser) error {
	// Clear associations
	teamUser.Team = nil
	teamUser.OrganizationUser = nil

	return r.db.WithContext(ctx).Save(teamUser).Error
}

func (r *teamUserRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.TeamUser{}, id).Error
}

func (r *teamUserRepository) DeleteByTeamAndOrgUser(ctx context.Context, teamID, orgUserID uint) error {
	return r.db.WithContext(ctx).
		Where("team_id = ? AND organization_user_id = ?", teamID, orgUserID).
		Delete(&domain.TeamUser{}).Error
}

