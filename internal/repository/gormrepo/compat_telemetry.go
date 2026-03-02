package gormrepo

import (
	"context"
	"strings"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

type compatTelemetryRepository struct {
	db *gorm.DB
}

// NewCompatTelemetryRepository creates a new compatibility telemetry repository.
func NewCompatTelemetryRepository(db *gorm.DB) repository.CompatTelemetryRepository {
	return &compatTelemetryRepository{db: db}
}

func (r *compatTelemetryRepository) CreateBatch(ctx context.Context, events []*domain.CompatTelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Create(&events).Error
}

func (r *compatTelemetryRepository) List(
	ctx context.Context,
	filter repository.CompatTelemetryListFilter,
) ([]*domain.CompatTelemetryEvent, int64, int64, error) {
	var (
		items    []*domain.CompatTelemetryEvent
		total    int64
		filtered int64
	)

	baseQuery := r.db.WithContext(ctx).Model(&domain.CompatTelemetryEvent{})
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	query := r.db.WithContext(ctx).Model(&domain.CompatTelemetryEvent{})
	if filter.Domain != "" {
		query = query.Where("domain_etld1 = ?", filter.Domain)
	}
	if filter.EventName != "" {
		query = query.Where("event_name = ?", filter.EventName)
	}
	if filter.FlowType != "" {
		query = query.Where("flow_type = ?", filter.FlowType)
	}
	if filter.Surface != "" {
		query = query.Where("surface = ?", filter.Surface)
	}
	if filter.ErrorCode != "" {
		query = query.Where("error_code = ?", filter.ErrorCode)
	}
	if filter.Succeeded != nil {
		query = query.Where("succeeded = ?", *filter.Succeeded)
	}
	if filter.Search != "" {
		like := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			`LOWER(domain_etld1) LIKE ? OR LOWER(event_name) LIKE ? OR LOWER(flow_type) LIKE ? OR LOWER(surface) LIKE ? OR LOWER(error_code) LIKE ?`,
			like, like, like, like, like,
		)
	}

	if err := query.Count(&filtered).Error; err != nil {
		return nil, 0, 0, err
	}

	order := strings.ToLower(strings.TrimSpace(filter.Order))
	if order == "asc" {
		query = query.Order("created_at ASC")
	} else {
		query = query.Order("created_at DESC")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, 0, err
	}

	return items, total, filtered, nil
}
