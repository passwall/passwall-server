package repository

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// ExcludedDomainRepository defines the interface for excluded domain operations
type ExcludedDomainRepository interface {
	// Create adds a new excluded domain for a user
	Create(ctx context.Context, excludedDomain *domain.ExcludedDomain) error

	// GetByUserID returns all excluded domains for a user
	GetByUserID(ctx context.Context, userID uint) ([]*domain.ExcludedDomain, error)

	// GetByUserIDAndDomain checks if a domain is excluded for a user
	GetByUserIDAndDomain(ctx context.Context, userID uint, domain string) (*domain.ExcludedDomain, error)

	// Delete removes an excluded domain by ID (only if it belongs to the user)
	Delete(ctx context.Context, id uint, userID uint) error

	// DeleteByDomain removes an excluded domain by domain name (only if it belongs to the user)
	DeleteByDomain(ctx context.Context, userID uint, domain string) error
}
