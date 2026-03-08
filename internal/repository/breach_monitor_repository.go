package repository

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// BreachMonitorRepository defines data access methods for breach monitoring.
type BreachMonitorRepository interface {
	// MonitoredEmail CRUD
	CreateEmail(ctx context.Context, email *domain.MonitoredEmail) error
	GetEmailByID(ctx context.Context, id uint) (*domain.MonitoredEmail, error)
	GetEmailByOrgAndAddress(ctx context.Context, orgID uint, email string) (*domain.MonitoredEmail, error)
	ListEmailsByOrganization(ctx context.Context, orgID uint) ([]*domain.MonitoredEmail, error)
	UpdateEmail(ctx context.Context, email *domain.MonitoredEmail) error
	DeleteEmail(ctx context.Context, id uint) error

	// BreachRecord CRUD
	CreateBreachRecord(ctx context.Context, record *domain.BreachRecord) error
	GetBreachRecordByID(ctx context.Context, id uint) (*domain.BreachRecord, error)
	ListBreachesByEmailID(ctx context.Context, emailID uint) ([]*domain.BreachRecord, error)
	ListBreachesByOrganization(ctx context.Context, orgID uint) ([]*domain.BreachRecord, error)
	BreachExistsForEmail(ctx context.Context, emailID uint, breachName string) (bool, error)
	UpdateBreachRecord(ctx context.Context, record *domain.BreachRecord) error

	// Summary
	GetSummary(ctx context.Context, orgID uint) (*domain.BreachMonitorSummaryDTO, error)

	// Bulk operations for background worker
	ListAllMonitoredEmails(ctx context.Context) ([]*domain.MonitoredEmail, error)
}
