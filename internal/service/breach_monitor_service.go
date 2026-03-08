package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/hibp"
	"github.com/passwall/passwall-server/pkg/logger"
)

// BreachMonitorService handles dark-web / breach monitoring logic.
type BreachMonitorService interface {
	AddEmail(ctx context.Context, orgID uint, userID uint, email string) (*domain.MonitoredEmailDTO, error)
	RemoveEmail(ctx context.Context, orgID uint, userID uint, emailID uint) error
	ListEmails(ctx context.Context, orgID uint, userID uint) ([]*domain.MonitoredEmailDTO, error)
	CheckEmails(ctx context.Context, orgID uint, userID uint) error
	ListBreaches(ctx context.Context, orgID uint, userID uint) ([]*domain.BreachRecordDTO, error)
	DismissBreach(ctx context.Context, orgID uint, userID uint, breachID uint) error
	GetSummary(ctx context.Context, orgID uint, userID uint) (*domain.BreachMonitorSummaryDTO, error)

	// For background worker
	CheckSingleEmail(ctx context.Context, email *domain.MonitoredEmail) (int, error)
}

type breachMonitorService struct {
	repo        repository.BreachMonitorRepository
	hibpClient  *hibp.Client
	featureSvc  FeatureService
	orgUserRepo interface {
		GetByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error)
	}
}

// NewBreachMonitorService creates a new breach monitoring service.
func NewBreachMonitorService(
	repo repository.BreachMonitorRepository,
	hibpClient *hibp.Client,
	featureSvc FeatureService,
	orgUserRepo interface {
		GetByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error)
	},
) BreachMonitorService {
	return &breachMonitorService{
		repo:        repo,
		hibpClient:  hibpClient,
		featureSvc:  featureSvc,
		orgUserRepo: orgUserRepo,
	}
}

func (s *breachMonitorService) AddEmail(ctx context.Context, orgID uint, userID uint, emailAddr string) (*domain.MonitoredEmailDTO, error) {
	if err := s.checkAccess(ctx, orgID, userID); err != nil {
		return nil, err
	}

	// Check for duplicate
	existing, err := s.repo.GetEmailByOrgAndAddress(ctx, orgID, emailAddr)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("failed to check existing email: %w", err)
	}
	if existing != nil {
		return nil, repository.ErrAlreadyExists
	}

	email := &domain.MonitoredEmail{
		OrganizationID: orgID,
		Email:          emailAddr,
	}
	if err := s.repo.CreateEmail(ctx, email); err != nil {
		return nil, fmt.Errorf("failed to create monitored email: %w", err)
	}

	// Run initial breach check if HIBP is configured
	if s.hibpClient.Enabled() {
		newBreaches, err := s.CheckSingleEmail(ctx, email)
		if err != nil {
			logger.Errorf("initial breach check failed for %s: %v", emailAddr, err)
		} else {
			email.BreachCount = newBreaches
		}
	}

	// Reload with breach records
	email, err = s.repo.GetEmailByID(ctx, email.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload email: %w", err)
	}
	return domain.ToMonitoredEmailDTO(email), nil
}

func (s *breachMonitorService) RemoveEmail(ctx context.Context, orgID uint, userID uint, emailID uint) error {
	if err := s.checkAccess(ctx, orgID, userID); err != nil {
		return err
	}

	email, err := s.repo.GetEmailByID(ctx, emailID)
	if err != nil {
		return err
	}
	if email.OrganizationID != orgID {
		return repository.ErrForbidden
	}
	return s.repo.DeleteEmail(ctx, emailID)
}

func (s *breachMonitorService) ListEmails(ctx context.Context, orgID uint, userID uint) ([]*domain.MonitoredEmailDTO, error) {
	if err := s.checkAccess(ctx, orgID, userID); err != nil {
		return nil, err
	}

	emails, err := s.repo.ListEmailsByOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list emails: %w", err)
	}

	dtos := make([]*domain.MonitoredEmailDTO, 0, len(emails))
	for _, e := range emails {
		dtos = append(dtos, domain.ToMonitoredEmailDTO(e))
	}
	return dtos, nil
}

func (s *breachMonitorService) CheckEmails(ctx context.Context, orgID uint, userID uint) error {
	if err := s.checkAccess(ctx, orgID, userID); err != nil {
		return err
	}

	if !s.hibpClient.Enabled() {
		return fmt.Errorf("breach monitoring is not configured (missing HIBP API key)")
	}

	emails, err := s.repo.ListEmailsByOrganization(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to list emails: %w", err)
	}

	for _, email := range emails {
		if _, checkErr := s.CheckSingleEmail(ctx, email); checkErr != nil {
			logger.Errorf("breach check failed for email ID %d: %v", email.ID, checkErr)
		}
	}
	return nil
}

func (s *breachMonitorService) ListBreaches(ctx context.Context, orgID uint, userID uint) ([]*domain.BreachRecordDTO, error) {
	if err := s.checkAccess(ctx, orgID, userID); err != nil {
		return nil, err
	}

	records, err := s.repo.ListBreachesByOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list breaches: %w", err)
	}

	dtos := make([]*domain.BreachRecordDTO, 0, len(records))
	for _, r := range records {
		dtos = append(dtos, domain.ToBreachRecordDTO(r))
	}
	return dtos, nil
}

func (s *breachMonitorService) DismissBreach(ctx context.Context, orgID uint, userID uint, breachID uint) error {
	if err := s.checkAccess(ctx, orgID, userID); err != nil {
		return err
	}

	record, err := s.repo.GetBreachRecordByID(ctx, breachID)
	if err != nil {
		return err
	}

	// Verify the breach belongs to this organization
	email, err := s.repo.GetEmailByID(ctx, record.MonitoredEmailID)
	if err != nil {
		return err
	}
	if email.OrganizationID != orgID {
		return repository.ErrForbidden
	}

	record.IsDismissed = true
	return s.repo.UpdateBreachRecord(ctx, record)
}

func (s *breachMonitorService) GetSummary(ctx context.Context, orgID uint, userID uint) (*domain.BreachMonitorSummaryDTO, error) {
	if err := s.checkAccess(ctx, orgID, userID); err != nil {
		return nil, err
	}
	return s.repo.GetSummary(ctx, orgID)
}

// CheckSingleEmail checks a single monitored email against HIBP and stores
// any new breach records. Returns the count of new breaches found.
func (s *breachMonitorService) CheckSingleEmail(ctx context.Context, email *domain.MonitoredEmail) (int, error) {
	breaches, err := s.hibpClient.CheckBreachedAccount(email.Email)
	if err != nil {
		return 0, fmt.Errorf("HIBP check failed: %w", err)
	}

	newCount := 0
	for _, b := range breaches {
		exists, err := s.repo.BreachExistsForEmail(ctx, email.ID, b.Name)
		if err != nil {
			logger.Errorf("failed to check breach existence: %v", err)
			continue
		}
		if exists {
			continue
		}

		record := &domain.BreachRecord{
			MonitoredEmailID: email.ID,
			BreachName:       b.Name,
			BreachDomain:     b.Domain,
			BreachDate:       b.BreachDate,
			AddedDate:        b.AddedDate,
			DataClasses:      domain.StringSliceJSON(b.DataClasses),
			Description:      b.Description,
			LogoPath:         b.LogoPath,
			PwnCount:         b.PwnCount,
			IsVerified:       b.IsVerified,
			IsSensitive:      b.IsSensitive,
			DiscoveredAt:     time.Now(),
		}
		if err := s.repo.CreateBreachRecord(ctx, record); err != nil {
			logger.Errorf("failed to create breach record: %v", err)
			continue
		}
		newCount++
	}

	// Update email metadata
	now := time.Now()
	email.LastCheckedAt = &now
	email.BreachCount = len(breaches)
	if err := s.repo.UpdateEmail(ctx, email); err != nil {
		logger.Errorf("failed to update monitored email: %v", err)
	}

	return newCount, nil
}

func (s *breachMonitorService) checkFeatureAccess(ctx context.Context, orgID uint) error {
	ok, err := s.featureSvc.CanUseBreachMonitoring(ctx, orgID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrFeatureNotAvailable
	}
	return nil
}

func (s *breachMonitorService) checkAccess(ctx context.Context, orgID uint, userID uint) error {
	if _, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID); err != nil {
		return repository.ErrForbidden
	}
	return s.checkFeatureAccess(ctx, orgID)
}
