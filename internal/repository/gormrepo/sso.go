package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ssoConnectionRepository struct {
	db *gorm.DB
}

// NewSSOConnectionRepository creates a new SSO connection repository
func NewSSOConnectionRepository(db *gorm.DB) repository.SSOConnectionRepository {
	return &ssoConnectionRepository{db: db}
}

func (r *ssoConnectionRepository) Create(ctx context.Context, conn *domain.SSOConnection) error {
	return dbFromContext(ctx, r.db).Create(conn).Error
}

func (r *ssoConnectionRepository) GetByID(ctx context.Context, id uint) (*domain.SSOConnection, error) {
	var conn domain.SSOConnection
	if err := dbFromContext(ctx, r.db).First(&conn, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &conn, nil
}

func (r *ssoConnectionRepository) GetByUUID(ctx context.Context, uuid string) (*domain.SSOConnection, error) {
	var conn domain.SSOConnection
	if err := dbFromContext(ctx, r.db).Where("uuid = ?", uuid).First(&conn).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &conn, nil
}

func (r *ssoConnectionRepository) GetVerifiedByDomain(ctx context.Context, domainName string) (*domain.SSOConnection, error) {
	var conn domain.SSOConnection
	if err := dbFromContext(ctx, r.db).
		Where("domain = ? AND domain_verified_at IS NOT NULL", domainName).
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
	if err := dbFromContext(ctx, r.db).
		Where("domain = ? AND status = ? AND domain_verified_at IS NOT NULL", domainName, domain.SSOStatusActive).
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
	if err := dbFromContext(ctx, r.db).
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
	if err := dbFromContext(ctx, r.db).
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		Find(&conns).Error; err != nil {
		return nil, err
	}
	return conns, nil
}

func (r *ssoConnectionRepository) Update(ctx context.Context, conn *domain.SSOConnection) error {
	return dbFromContext(ctx, r.db).Save(conn).Error
}

func (r *ssoConnectionRepository) Delete(ctx context.Context, id uint) error {
	return dbFromContext(ctx, r.db).Delete(&domain.SSOConnection{}, id).Error
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
	return dbFromContext(ctx, r.db).Create(state).Error
}

func (r *ssoStateRepository) GetByState(ctx context.Context, stateVal string) (*domain.SSOState, error) {
	var state domain.SSOState
	if err := dbFromContext(ctx, r.db).Where("state = ?", stateVal).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &state, nil
}

// Consume deletes and returns an unexpired state in one statement, so a state
// can be used once even under concurrent callbacks.
func (r *ssoStateRepository) Consume(ctx context.Context, stateVal string) (*domain.SSOState, error) {
	var states []domain.SSOState
	if err := dbFromContext(ctx, r.db).
		Clauses(clause.Returning{}).
		Where("state = ? AND expires_at > NOW()", stateVal).
		Delete(&states).Error; err != nil {
		return nil, err
	}
	if len(states) == 0 {
		return nil, repository.ErrNotFound
	}
	return &states[0], nil
}

func (r *ssoStateRepository) Delete(ctx context.Context, id uint) error {
	return dbFromContext(ctx, r.db).Delete(&domain.SSOState{}, id).Error
}

func (r *ssoStateRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := dbFromContext(ctx, r.db).Where("expires_at < NOW()").Delete(&domain.SSOState{})
	return result.RowsAffected, result.Error
}

// --- SSO login codes ---

type ssoLoginCodeRepository struct {
	db *gorm.DB
}

// NewSSOLoginCodeRepository creates a new SSO login code repository
func NewSSOLoginCodeRepository(db *gorm.DB) repository.SSOLoginCodeRepository {
	return &ssoLoginCodeRepository{db: db}
}

func (r *ssoLoginCodeRepository) Create(ctx context.Context, code *domain.SSOLoginCode) error {
	return dbFromContext(ctx, r.db).Create(code).Error
}

// Consume deletes and returns an unexpired code in one statement.
func (r *ssoLoginCodeRepository) Consume(ctx context.Context, codeHash string) (*domain.SSOLoginCode, error) {
	var codes []domain.SSOLoginCode
	if err := dbFromContext(ctx, r.db).
		Clauses(clause.Returning{}).
		Where("code_hash = ? AND expires_at > NOW()", codeHash).
		Delete(&codes).Error; err != nil {
		return nil, err
	}
	if len(codes) == 0 {
		return nil, repository.ErrNotFound
	}
	return &codes[0], nil
}

func (r *ssoLoginCodeRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := dbFromContext(ctx, r.db).Where("expires_at < NOW()").Delete(&domain.SSOLoginCode{})
	return result.RowsAffected, result.Error
}
