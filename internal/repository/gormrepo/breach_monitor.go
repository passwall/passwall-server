package gormrepo

import (
	"context"
	"errors"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type breachMonitorRepository struct {
	db *gorm.DB
}

func NewBreachMonitorRepository(db *gorm.DB) repository.BreachMonitorRepository {
	return &breachMonitorRepository{db: db}
}

// ── MonitoredEmail ──────────────────────────────────────────

func (r *breachMonitorRepository) CreateEmail(ctx context.Context, email *domain.MonitoredEmail) error {
	return r.db.WithContext(ctx).Create(email).Error
}

func (r *breachMonitorRepository) GetEmailByID(ctx context.Context, id uint) (*domain.MonitoredEmail, error) {
	var email domain.MonitoredEmail
	err := r.db.WithContext(ctx).
		Preload("BreachRecords").
		First(&email, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &email, nil
}

func (r *breachMonitorRepository) GetEmailByOrgAndAddress(ctx context.Context, orgID uint, addr string) (*domain.MonitoredEmail, error) {
	var email domain.MonitoredEmail
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND email = ?", orgID, addr).
		First(&email).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &email, nil
}

func (r *breachMonitorRepository) ListEmailsByOrganization(ctx context.Context, orgID uint) ([]*domain.MonitoredEmail, error) {
	var emails []*domain.MonitoredEmail
	err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Preload("BreachRecords").
		Order("created_at DESC").
		Find(&emails).Error
	return emails, err
}

func (r *breachMonitorRepository) UpdateEmail(ctx context.Context, email *domain.MonitoredEmail) error {
	return r.db.WithContext(ctx).Save(email).Error
}

func (r *breachMonitorRepository) DeleteEmail(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.MonitoredEmail{}, id).Error
}

// ── BreachRecord ────────────────────────────────────────────

func (r *breachMonitorRepository) CreateBreachRecord(ctx context.Context, record *domain.BreachRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *breachMonitorRepository) GetBreachRecordByID(ctx context.Context, id uint) (*domain.BreachRecord, error) {
	var record domain.BreachRecord
	err := r.db.WithContext(ctx).First(&record, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &record, nil
}

func (r *breachMonitorRepository) ListBreachesByEmailID(ctx context.Context, emailID uint) ([]*domain.BreachRecord, error) {
	var records []*domain.BreachRecord
	err := r.db.WithContext(ctx).
		Where("monitored_email_id = ?", emailID).
		Order("breach_date DESC").
		Find(&records).Error
	return records, err
}

func (r *breachMonitorRepository) ListBreachesByOrganization(ctx context.Context, orgID uint) ([]*domain.BreachRecord, error) {
	var records []*domain.BreachRecord
	err := r.db.WithContext(ctx).
		Joins("JOIN monitored_emails ON monitored_emails.id = breach_records.monitored_email_id").
		Where("monitored_emails.organization_id = ?", orgID).
		Order("breach_records.breach_date DESC").
		Find(&records).Error
	return records, err
}

func (r *breachMonitorRepository) BreachExistsForEmail(ctx context.Context, emailID uint, breachName string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.BreachRecord{}).
		Where("monitored_email_id = ? AND breach_name = ?", emailID, breachName).
		Count(&count).Error
	return count > 0, err
}

func (r *breachMonitorRepository) UpdateBreachRecord(ctx context.Context, record *domain.BreachRecord) error {
	return r.db.WithContext(ctx).Save(record).Error
}

// ── Summary ─────────────────────────────────────────────────

func (r *breachMonitorRepository) GetSummary(ctx context.Context, orgID uint) (*domain.BreachMonitorSummaryDTO, error) {
	summary := &domain.BreachMonitorSummaryDTO{}

	// Total monitored emails
	var emailCount int64
	if err := r.db.WithContext(ctx).
		Model(&domain.MonitoredEmail{}).
		Where("organization_id = ?", orgID).
		Count(&emailCount).Error; err != nil {
		return nil, err
	}
	summary.MonitoredEmails = int(emailCount)

	// Total and active breaches
	type breachStats struct {
		Total  int
		Active int
	}
	var stats breachStats
	r.db.WithContext(ctx).
		Model(&domain.BreachRecord{}).
		Joins("JOIN monitored_emails ON monitored_emails.id = breach_records.monitored_email_id").
		Where("monitored_emails.organization_id = ?", orgID).
		Select("COUNT(*) as total, COUNT(CASE WHEN breach_records.is_dismissed = false THEN 1 END) as active").
		Scan(&stats)
	summary.TotalBreaches = stats.Total
	summary.ActiveBreaches = stats.Active

	// Last checked time (most recent across all emails)
	var lastChecked domain.MonitoredEmail
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND last_checked_at IS NOT NULL", orgID).
		Order("last_checked_at DESC").
		First(&lastChecked).Error
	if err == nil {
		summary.LastCheckedAt = lastChecked.LastCheckedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return summary, nil
}

// ── Bulk ────────────────────────────────────────────────────

func (r *breachMonitorRepository) ListAllMonitoredEmails(ctx context.Context) ([]*domain.MonitoredEmail, error) {
	var emails []*domain.MonitoredEmail
	err := r.db.WithContext(ctx).
		Order("organization_id, id").
		Find(&emails).Error
	return emails, err
}
