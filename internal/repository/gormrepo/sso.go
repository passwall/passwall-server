package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type ssoConnectionRepository struct {
	db *gorm.DB
}

// NewSSOConnectionRepository creates a new SSO connection repository
func NewSSOConnectionRepository(db *gorm.DB) repository.SSOConnectionRepository {
	return &ssoConnectionRepository{db: db}
}

func (r *ssoConnectionRepository) Create(ctx context.Context, conn *domain.SSOConnection) error {
	return r.db.WithContext(ctx).Create(conn).Error
}

func (r *ssoConnectionRepository) GetByID(ctx context.Context, id uint) (*domain.SSOConnection, error) {
	var conn domain.SSOConnection
	if err := r.db.WithContext(ctx).First(&conn, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &conn, nil
}

func (r *ssoConnectionRepository) GetByUUID(ctx context.Context, uuid string) (*domain.SSOConnection, error) {
	var conn domain.SSOConnection
	if err := r.db.WithContext(ctx).Where("uuid = ?", uuid).First(&conn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &conn, nil
}

func (r *ssoConnectionRepository) GetAnyByDomain(ctx context.Context, domainName string) (*domain.SSOConnection, error) {
	var conn domain.SSOConnection
	if err := r.db.WithContext(ctx).
		Where("domain = ?", domainName).
		First(&conn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &conn, nil
}

func (r *ssoConnectionRepository) GetByDomain(ctx context.Context, domainName string) (*domain.SSOConnection, error) {
	var conn domain.SSOConnection
	if err := r.db.WithContext(ctx).
		Where("domain = ? AND status = ?", domainName, domain.SSOStatusActive).
		First(&conn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &conn, nil
}

func (r *ssoConnectionRepository) GetByOrganizationID(ctx context.Context, orgID uint) (*domain.SSOConnection, error) {
	var conn domain.SSOConnection
	if err := r.db.WithContext(ctx).
		Where("organization_id = ? AND status = ?", orgID, domain.SSOStatusActive).
		First(&conn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &conn, nil
}

func (r *ssoConnectionRepository) ListByOrganization(ctx context.Context, orgID uint) ([]*domain.SSOConnection, error) {
	var conns []*domain.SSOConnection
	if err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		Find(&conns).Error; err != nil {
		return nil, err
	}
	return conns, nil
}

func (r *ssoConnectionRepository) Update(ctx context.Context, conn *domain.SSOConnection) error {
	return r.db.WithContext(ctx).Save(conn).Error
}

func (r *ssoConnectionRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.SSOConnection{}, id).Error
}

// --- SSO State ---

type ssoStateRepository struct {
	db *gorm.DB
}

// NewSSOStateRepository creates a new SSO state repository
func NewSSOStateRepository(db *gorm.DB) repository.SSOStateRepository {
	return &ssoStateRepository{db: db}
}

func (r *ssoStateRepository) Create(ctx context.Context, state *domain.SSOState) error {
	return r.db.WithContext(ctx).Create(state).Error
}

func (r *ssoStateRepository) GetByState(ctx context.Context, stateVal string) (*domain.SSOState, error) {
	var state domain.SSOState
	if err := r.db.WithContext(ctx).Where("state = ?", stateVal).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &state, nil
}

func (r *ssoStateRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.SSOState{}, id).Error
}

func (r *ssoStateRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Where("expires_at < NOW()").Delete(&domain.SSOState{})
	return result.RowsAffected, result.Error
}
