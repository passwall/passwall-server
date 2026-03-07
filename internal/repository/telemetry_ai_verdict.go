package repository

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// TelemetryAIVerdictRepository handles persistence of AI-generated telemetry verdicts.
type TelemetryAIVerdictRepository interface {
	// Upsert inserts or updates a verdict by its dedupe key.
	Upsert(ctx context.Context, verdict *domain.TelemetryAIVerdict) error
	// UpsertBatch inserts or updates multiple verdicts.
	UpsertBatch(ctx context.Context, verdicts []*domain.TelemetryAIVerdict) error
	// ListAll returns all stored verdicts (optionally limited).
	ListAll(ctx context.Context, limit int) ([]*domain.TelemetryAIVerdict, error)
	// FindByKeys returns existing verdicts matching the given dedupe keys.
	FindByKeys(ctx context.Context, keys []VerdictKey) ([]*domain.TelemetryAIVerdict, error)
	// DeleteAll removes all verdicts (for re-analysis).
	DeleteAll(ctx context.Context) (int64, error)
}

// VerdictKey identifies a unique telemetry event group for verdict lookup.
type VerdictKey struct {
	DomainETLD1 string
	PagePath    string
	EventName   string
	ErrorCode   string
	FlowType    string
	Surface     string
	Succeeded   bool
}
