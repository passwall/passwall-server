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
	// Validate plan configs
	if len(planConfigs) == 0 {
		return fmt.Errorf("no plan configurations provided in config file")
	}

	// Load all existing plans once (we upsert by code)
	var existingPlans []domain.Plan
	if err := db.WithContext(ctx).Find(&existingPlans).Error; err != nil {
		return fmt.Errorf("failed to load existing plans: %w", err)
	}
	existingByCode := make(map[string]*domain.Plan, len(existingPlans))
	for i := range existingPlans {
		p := &existingPlans[i]
		existingByCode[p.Code] = p
	}

	configCodes := make(map[string]struct{}, len(planConfigs))

	// Begin transaction
	return db.Transaction(func(tx *gorm.DB) error {
		upserted := 0
		created := 0
		deactivated := 0

		for _, pc := range planConfigs {
			configCodes[pc.Code] = struct{}{}

			// Validate billing cycle
			if pc.BillingCycle != "monthly" && pc.BillingCycle != "yearly" {
				return fmt.Errorf("invalid billing_cycle for plan %s: %s (must be monthly or yearly)", pc.Code, pc.BillingCycle)
			}

			if existing, ok := existingByCode[pc.Code]; ok && existing != nil {
				// Update existing plan in place
				existing.Name = pc.Name
				existing.BillingCycle = domain.BillingCycle(pc.BillingCycle)
				existing.PriceCents = pc.PriceCents
				existing.Currency = pc.Currency
				existing.TrialDays = pc.TrialDays
				existing.MaxUsers = pc.MaxUsers
				existing.MaxCollections = pc.MaxCollections
				existing.MaxItems = pc.MaxItems
				existing.Features = domain.PlanFeatures{
					Items:           pc.MaxItems, // Same as MaxItems for backward compatibility
					Sharing:         pc.Features.Sharing,
					Teams:           pc.Features.Teams,
					Audit:           pc.Features.Audit,
					SSO:             pc.Features.SSO,
					APIAccess:       pc.Features.APIAccess,
					PrioritySupport: pc.Features.PrioritySupport,
				}
				existing.IsActive = true

				if pc.StripePriceID != "" {
					existing.StripePriceID = &pc.StripePriceID
				} else {
					existing.StripePriceID = nil
				}

				if err := tx.WithContext(ctx).Save(existing).Error; err != nil {
					return fmt.Errorf("failed to update plan %s: %w", pc.Code, err)
				}
				upserted++
				continue
			}

			// Create new plan
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
			if pc.StripePriceID != "" {
				plan.StripePriceID = &pc.StripePriceID
			}

			if err := tx.WithContext(ctx).Create(&plan).Error; err != nil {
				return fmt.Errorf("failed to create plan %s: %w", plan.Code, err)
			}
			created++
			upserted++
		}

		// Deactivate plans that are not in config anymore (keeps history but hides from clients)
		for i := range existingPlans {
			p := &existingPlans[i]
			if _, ok := configCodes[p.Code]; ok {
				continue
			}
			if !p.IsActive {
				continue
			}
			p.IsActive = false
			if err := tx.WithContext(ctx).Save(p).Error; err != nil {
				return fmt.Errorf("failed to deactivate plan %s: %w", p.Code, err)
			}
			deactivated++
		}

		logger.Infof("✓ Seeded subscription plans (upsert=%d, created=%d, deactivated=%d)", upserted, created, deactivated)
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
