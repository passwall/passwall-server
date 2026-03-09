package gormrepo

import (
	"context"
	"strings"
	"time"

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

func (r *compatTelemetryRepository) ListExistingCompatKeys(ctx context.Context, since time.Time) ([]repository.CompatTelemetryDedupeKey, error) {
	var rows []repository.CompatTelemetryDedupeKey
	err := r.db.WithContext(ctx).Model(&domain.CompatTelemetryEvent{}).
		Select(`domain_etld1, page_path, event_name,
			COALESCE(NULLIF(TRIM(error_code),''), 'none') AS error_code,
			flow_type, surface, succeeded`).
		Where("created_at >= ?", since).
		Distinct().
		Scan(&rows).Error
	return rows, err
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
	if filter.PagePath != "" {
		query = query.Where("page_path = ?", filter.PagePath)
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
	if filter.CreatedAfter != nil {
		query = query.Where("created_at >= ?", *filter.CreatedAfter)
	}
	if filter.Search != "" {
		like := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			`LOWER(domain_etld1) LIKE ? OR LOWER(page_path) LIKE ? OR LOWER(event_name) LIKE ? OR LOWER(flow_type) LIKE ? OR LOWER(surface) LIKE ? OR LOWER(error_code) LIKE ?`,
			like, like, like, like, like, like,
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

func (r *compatTelemetryRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("created_at < ?", before).Delete(&domain.CompatTelemetryEvent{})
	return res.RowsAffected, res.Error
}

func (r *compatTelemetryRepository) ListSummary(
	ctx context.Context,
	filter repository.CompatTelemetryListFilter,
) ([]*domain.CompatTelemetrySummaryRow, int64, error) {
	base := r.db.WithContext(ctx).Model(&domain.CompatTelemetryEvent{}).Select(
		`domain_etld1, COALESCE(NULLIF(TRIM(page_path),''), domain_etld1) AS page_path, event_name, COALESCE(NULLIF(error_code,''), 'none') AS error_code, flow_type, surface, succeeded,
		 COUNT(*) AS count, MIN(created_at) AS first_seen, MAX(created_at) AS last_seen,
		 MAX(step_index) AS max_step_index,
		 BOOL_OR(prev_step_had_identifier) AS has_prev_step_identifier,
		 BOOL_OR(curr_step_has_password) AS has_curr_step_password,
		 BOOL_OR(field_visibility_issue) AS has_field_visibility_issue,
		 MODE() WITHIN GROUP (ORDER BY form_method) AS top_form_method,
		 MODE() WITHIN GROUP (ORDER BY detected_field_signature) AS top_field_signature`,
	).Group(
		`domain_etld1, COALESCE(NULLIF(TRIM(page_path),''), domain_etld1), event_name, COALESCE(NULLIF(error_code,''), 'none'), flow_type, surface, succeeded`,
	)
	if filter.Domain != "" {
		base = base.Where("domain_etld1 = ?", filter.Domain)
	}
	if filter.PagePath != "" {
		base = base.Where("page_path = ?", filter.PagePath)
	}
	if filter.EventName != "" {
		base = base.Where("event_name = ?", filter.EventName)
	}
	if filter.FlowType != "" {
		base = base.Where("flow_type = ?", filter.FlowType)
	}
	if filter.Surface != "" {
		base = base.Where("surface = ?", filter.Surface)
	}
	if filter.ErrorCode != "" {
		base = base.Where("COALESCE(NULLIF(error_code,''), 'none') = ?", filter.ErrorCode)
	}
	if filter.Succeeded != nil {
		base = base.Where("succeeded = ?", *filter.Succeeded)
	}
	if filter.CreatedAfter != nil {
		base = base.Where("created_at >= ?", *filter.CreatedAfter)
	}
	if filter.Search != "" {
		like := "%" + strings.ToLower(filter.Search) + "%"
		base = base.Where(
			`LOWER(domain_etld1) LIKE ? OR LOWER(page_path) LIKE ? OR LOWER(event_name) LIKE ? OR LOWER(flow_type) LIKE ? OR LOWER(surface) LIKE ? OR LOWER(error_code) LIKE ?`,
			like, like, like, like, like, like,
		)
	}

	var total int64
	countQuery := r.db.WithContext(ctx).Table("(?) AS sub", base)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := "last_seen DESC"
	if strings.ToLower(strings.TrimSpace(filter.Order)) == "asc" {
		order = "last_seen ASC"
	}
	base = base.Order(order)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	base = base.Limit(limit).Offset(offset)

	var rows []*domain.CompatTelemetrySummaryRow
	if err := base.Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		if row.ErrorCode == "none" {
			row.ErrorCode = ""
		}
		if row.PagePath == "" {
			row.PagePath = row.DomainETLD1
		}
	}
	return rows, total, nil
}
