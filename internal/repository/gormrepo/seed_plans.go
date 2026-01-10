package gormrepo

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/logger"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

// SeedPlans creates subscription plans from config if they don't exist
func SeedPlans(ctx context.Context, db *gorm.DB, planConfigs []config.PlanConfig) error {
	// Check if plans already exist
	var count int64
	if err := db.WithContext(ctx).Model(&domain.Plan{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check plans: %w", err)
	}

	// If plans exist, skip seeding
	if count > 0 {
		logger.Infof("✓ Plans already seeded (%d existing)", count)
		return nil
	}

	// Validate plan configs
	if len(planConfigs) == 0 {
		return fmt.Errorf("no plan configurations provided in config file")
	}

	// Convert config plans to domain plans
	plans := make([]domain.Plan, 0, len(planConfigs))
	for _, pc := range planConfigs {
		// Validate billing cycle
		if pc.BillingCycle != "monthly" && pc.BillingCycle != "yearly" {
			return fmt.Errorf("invalid billing_cycle for plan %s: %s (must be monthly or yearly)", pc.Code, pc.BillingCycle)
		}

		plan := domain.Plan{
			UUID:           uuid.NewV4(),
			Code:           pc.Code,
			Name:           pc.Name,
			BillingCycle:   domain.BillingCycle(pc.BillingCycle),
			PriceCents:     pc.PriceCents,
			Currency:       pc.Currency,
			TrialDays:      pc.TrialDays,
			MaxUsers:       pc.MaxUsers,
			MaxCollections: pc.MaxCollections,
			MaxItems:       pc.MaxItems,
			Features: domain.PlanFeatures{
				Items:           pc.MaxItems, // Same as MaxItems for backward compatibility
				Sharing:         pc.Features.Sharing,
				Teams:           pc.Features.Teams,
				Audit:           pc.Features.Audit,
				SSO:             pc.Features.SSO,
				APIAccess:       pc.Features.APIAccess,
				PrioritySupport: pc.Features.PrioritySupport,
			},
			IsActive: true,
		}

		// Set Stripe price ID if provided
		if pc.StripePriceID != "" {
			plan.StripePriceID = &pc.StripePriceID
		}

		plans = append(plans, plan)
	}

	// Begin transaction
	return db.Transaction(func(tx *gorm.DB) error {
		for _, plan := range plans {
			if err := tx.WithContext(ctx).Create(&plan).Error; err != nil {
				return fmt.Errorf("failed to create plan %s: %w", plan.Code, err)
			}
		}

		logger.Infof("✓ Seeded %d subscription plans", len(plans))
		return nil
	})
}

// SeedDefaultSubscriptions creates free subscriptions for existing organizations
func SeedDefaultSubscriptions(ctx context.Context, db *gorm.DB) error {
	// Get free plan
	var freePlan domain.Plan
	if err := db.WithContext(ctx).Where("code = ?", "free-monthly").First(&freePlan).Error; err != nil {
		// If free plan doesn't exist, skip (plans should be seeded first)
		return nil
	}

	// Find organizations without subscriptions
	var orgs []domain.Organization
	if err := db.WithContext(ctx).
		Joins("LEFT JOIN subscriptions ON subscriptions.organization_id = organizations.id").
		Where("subscriptions.id IS NULL").
		Find(&orgs).Error; err != nil {
		return fmt.Errorf("failed to find organizations: %w", err)
	}

	if len(orgs) == 0 {
		return nil
	}

	// Create free subscriptions for them
	return db.Transaction(func(tx *gorm.DB) error {
		for _, org := range orgs {
			sub := &domain.Subscription{
				UUID:           uuid.NewV4(),
				OrganizationID: org.ID,
				PlanID:         freePlan.ID,
				State:          domain.SubStateActive,
			}

			if err := tx.WithContext(ctx).Create(sub).Error; err != nil {
				return fmt.Errorf("failed to create subscription for org %d: %w", org.ID, err)
			}
		}

		logger.Infof("✓ Created %d default free subscriptions", len(orgs))
		return nil
	})
}
