package gormrepo

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type telemetryAIVerdictRepository struct {
	db *gorm.DB
}

// NewTelemetryAIVerdictRepository creates a new verdict repository.
func NewTelemetryAIVerdictRepository(db *gorm.DB) repository.TelemetryAIVerdictRepository {
	return &telemetryAIVerdictRepository{db: db}
}

func (r *telemetryAIVerdictRepository) Upsert(ctx context.Context, verdict *domain.TelemetryAIVerdict) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "domain_etld1"},
				{Name: "page_path"},
				{Name: "event_name"},
				{Name: "error_code"},
				{Name: "flow_type"},
				{Name: "surface"},
				{Name: "succeeded"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"classification", "severity", "reasoning", "suggested_action",
				"model", "event_count", "updated_at",
			}),
		}).
		Create(verdict).Error
}

func (r *telemetryAIVerdictRepository) UpsertBatch(ctx context.Context, verdicts []*domain.TelemetryAIVerdict) error {
	if len(verdicts) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "domain_etld1"},
				{Name: "page_path"},
				{Name: "event_name"},
				{Name: "error_code"},
				{Name: "flow_type"},
				{Name: "surface"},
				{Name: "succeeded"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"classification", "severity", "reasoning", "suggested_action",
				"model", "event_count", "updated_at",
			}),
		}).
		Create(&verdicts).Error
}

func (r *telemetryAIVerdictRepository) ListAll(ctx context.Context, limit int) ([]*domain.TelemetryAIVerdict, error) {
	var verdicts []*domain.TelemetryAIVerdict
	q := r.db.WithContext(ctx).Order("updated_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&verdicts).Error
	return verdicts, err
}

func (r *telemetryAIVerdictRepository) FindByKeys(
	ctx context.Context,
	keys []repository.VerdictKey,
) ([]*domain.TelemetryAIVerdict, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	// Build WHERE (domain_etld1, page_path, event_name, error_code, flow_type, surface, succeeded) IN (...)
	// GORM doesn't natively support tuple-IN, so we use OR conditions.
	q := r.db.WithContext(ctx).Model(&domain.TelemetryAIVerdict{})

	// For performance, limit to first 500 keys
	searchKeys := keys
	if len(searchKeys) > 500 {
		searchKeys = searchKeys[:500]
	}

	orQuery := r.db.WithContext(ctx)
	for i, k := range searchKeys {
		cond := r.db.Where(
			"domain_etld1 = ? AND page_path = ? AND event_name = ? AND error_code = ? AND flow_type = ? AND surface = ? AND succeeded = ?",
			k.DomainETLD1, k.PagePath, k.EventName, k.ErrorCode, k.FlowType, k.Surface, k.Succeeded,
		)
		if i == 0 {
			orQuery = cond
		} else {
			orQuery = orQuery.Or(cond)
		}
	}

	var verdicts []*domain.TelemetryAIVerdict
	err := q.Where(orQuery).Find(&verdicts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find verdicts by keys: %w", err)
	}
	return verdicts, nil
}

func (r *telemetryAIVerdictRepository) DeleteAll(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Where("1 = 1").Delete(&domain.TelemetryAIVerdict{})
	return result.RowsAffected, result.Error
}
