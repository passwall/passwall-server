package repository

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// InvitationRepository defines the interface for invitation data access
type InvitationRepository interface {
	Create(ctx context.Context, invitation *domain.Invitation) error
	GetByEmail(ctx context.Context, email string) (*domain.Invitation, error)
	GetByCode(ctx context.Context, code string) (*domain.Invitation, error)
	Delete(ctx context.Context, id uint) error
	DeleteExpired(ctx context.Context) error
}
