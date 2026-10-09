package repository

import (
	"context"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
)

// InvitationRepository stores referral ("invite a friend") invitations.
// Organization invitations live in OrganizationInvitationRepository.
type InvitationRepository interface {
	Create(ctx context.Context, invitation *domain.Invitation) error
	// GetActiveByEmail returns an unused, unexpired referral for email.
	GetActiveByEmail(ctx context.Context, email string) (*domain.Invitation, error)
	ListByCreator(ctx context.Context, createdBy uint) ([]*domain.Invitation, error)
	CountByCreatorSince(ctx context.Context, createdBy uint, since time.Time) (int, error)
	// MarkUsedByEmail records that the invited person signed up.
	MarkUsedByEmail(ctx context.Context, email string, usedAt time.Time) error
	DeleteByEmail(ctx context.Context, email string) error
}
