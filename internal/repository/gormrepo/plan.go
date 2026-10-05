package gormrepo

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
	"gorm.io/gorm"
)

type planRepository struct {
	db *gorm.DB
}

// NewPlanRepository creates a new plan repository
func NewPlanRepository(db *gorm.DB) *planRepository {
	return &planRepository{db: db}
}

// Create creates a new plan
func (r *planRepository) Create(ctx context.Context, plan *domain.Plan) error {
	return dbFromContext(ctx, r.db).Create(plan).Error
}

// GetByID retrieves a plan by ID
func (r *planRepository) GetByID(ctx context.Context, id uint) (*domain.Plan, error) {
	var plan domain.Plan
	err := dbFromContext(ctx, r.db).First(&plan, id).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// GetByUUID retrieves a plan by UUID
func (r *planRepository) GetByUUID(ctx context.Context, uuid string) (*domain.Plan, error) {
	var plan domain.Plan
	err := dbFromContext(ctx, r.db).Where("uuid = ?", uuid).First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// GetByCode retrieves a plan by code
func (r *planRepository) GetByCode(ctx context.Context, code string) (*domain.Plan, error) {
	var plan domain.Plan
	err := dbFromContext(ctx, r.db).Where("code = ?", code).First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// GetByStripePriceID retrieves a plan by Stripe price ID
func (r *planRepository) GetByStripePriceID(ctx context.Context, stripePriceID string) (*domain.Plan, error) {
	var plan domain.Plan
	err := dbFromContext(ctx, r.db).Where("stripe_price_id = ?", stripePriceID).First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// Update updates a plan
func (r *planRepository) Update(ctx context.Context, plan *domain.Plan) error {
	return dbFromContext(ctx, r.db).Save(plan).Error
}

// Delete soft deletes a plan
func (r *planRepository) Delete(ctx context.Context, id uint) error {
	return dbFromContext(ctx, r.db).Delete(&domain.Plan{}, id).Error
}

// List retrieves all plans
func (r *planRepository) List(ctx context.Context) ([]*domain.Plan, error) {
	var plans []*domain.Plan
	err := dbFromContext(ctx, r.db).Where("is_active = ?", true).Order("price_cents ASC").Find(&plans).Error
	return plans, err
}

// ListAll retrieves all plans including inactive ones
func (r *planRepository) ListAll(ctx context.Context) ([]*domain.Plan, error) {
	var plans []*domain.Plan
	err := dbFromContext(ctx, r.db).Order("price_cents ASC").Find(&plans).Error
	return plans, err
}

// ListByBillingCycle retrieves plans by billing cycle
func (r *planRepository) ListByBillingCycle(ctx context.Context, cycle domain.BillingCycle) ([]*domain.Plan, error) {
	var plans []*domain.Plan
	err := dbFromContext(ctx, r.db).
		Where("billing_cycle = ? AND is_active = ?", cycle, true).
		Order("price_cents ASC").
		Find(&plans).Error
	return plans, err
}
