package gormrepo

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/logger"
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
				// Versioned migrations own the commercial contract. Runtime
				// configuration may only attach deployment-specific Stripe IDs.
				if pc.StripePriceID != "" {
					existing.StripePriceID = &pc.StripePriceID
				}

				if err := tx.WithContext(ctx).Save(existing).Error; err != nil {
					return fmt.Errorf("failed to update plan %s: %w", pc.Code, err)
				}
				upserted++
				continue
			}

			// Create new plan
			plan := domain.Plan{
				UUID:           uuid.New(),
				Code:           pc.Code,
				Name:           pc.Name,
				BillingCycle:   domain.BillingCycle(pc.BillingCycle),
				PriceCents:     pc.PriceCents,
				Currency:       pc.Currency,
				TrialDays:      pc.TrialDays,
				MaxUsers:       pc.MaxUsers,
				MaxCollections: pc.MaxCollections,
				MaxItems:       pc.MaxItems,
				ExpiryBehavior: defaultExpiryBehavior(pc.Code),
				GraceDays:      defaultGraceDays(pc.Code),
				Features: domain.PlanFeatures{
					Items:            pc.MaxItems, // Same as MaxItems for backward compatibility
					Sharing:          pc.Features.Sharing,
					SharedItems:      pc.Features.SharedItems,
					SecureSend:       pc.Features.SecureSend,
					Passkeys:         pc.Features.Passkeys,
					EmergencyAccess:  pc.Features.EmergencyAccess,
					Teams:            pc.Features.Teams,
					Audit:            pc.Features.Audit,
					SSO:              pc.Features.SSO,
					APIAccess:        pc.Features.APIAccess,
					PrioritySupport:  pc.Features.PrioritySupport,
					Policies:         pc.Features.Policies,
					SecurityInsights: pc.Features.SecurityInsights,
					BreachMonitoring: pc.Features.BreachMonitoring,
				},
				IsActive: true,
			}
			if pc.StripePriceID != "" {
				plan.StripePriceID = &pc.StripePriceID
			}
			applyCanonicalPlanContract(&plan)

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

func applyCanonicalPlanContract(plan *domain.Plan) {
	baseCode := strings.Split(plan.Code, "-")[0]
	unlimitedFeatures := domain.PlanFeatures{
		Sharing:          true,
		SharedItems:      true,
		SecureSend:       true,
		Passkeys:         true,
		EmergencyAccess:  true,
		SecurityInsights: true,
		BreachMonitoring: true,
	}
	switch baseCode {
	case "free":
		plan.MaxUsers = intValue(1)
		plan.MaxCollections = intValue(10)
		plan.MaxItems = intValue(100)
		plan.MaxDevices = nil
		plan.ExpiryBehavior = domain.ExpiryBehaviorDowngradeToFree
		plan.GraceDays = 0
		plan.Features = domain.PlanFeatures{Items: intValue(100)}
	case "pro":
		plan.MaxUsers = intValue(1)
		plan.MaxCollections = nil
		plan.MaxItems = nil
		plan.MaxDevices = nil
		plan.ExpiryBehavior = domain.ExpiryBehaviorDowngradeToFree
		plan.GraceDays = 14
		plan.Features = unlimitedFeatures
	case "family":
		plan.MaxUsers = intValue(6)
		plan.MaxCollections = nil
		plan.MaxItems = nil
		plan.MaxDevices = nil
		plan.ExpiryBehavior = domain.ExpiryBehaviorFreeze
		plan.GraceDays = 14
		plan.Features = unlimitedFeatures
	case "team":
		plan.MaxUsers = intValue(10)
		plan.MaxCollections = nil
		plan.MaxItems = nil
		plan.MaxDevices = nil
		plan.ExpiryBehavior = domain.ExpiryBehaviorFreeze
		plan.GraceDays = 14
		plan.Features = unlimitedFeatures
		plan.Features.Teams = true
		plan.Features.Policies = true
	case "business":
		plan.MaxUsers = nil
		plan.MaxCollections = nil
		plan.MaxItems = nil
		plan.MaxDevices = nil
		plan.ExpiryBehavior = domain.ExpiryBehaviorFreeze
		plan.GraceDays = 14
		plan.Features = unlimitedFeatures
		plan.Features.Teams = true
		plan.Features.Audit = true
		plan.Features.SSO = true
		plan.Features.Policies = true
		plan.Features.BusinessPolicies = true
	case "enterprise":
		plan.MaxUsers = nil
		plan.MaxCollections = nil
		plan.MaxItems = nil
		plan.MaxDevices = nil
		plan.ExpiryBehavior = domain.ExpiryBehaviorFreeze
		plan.GraceDays = 14
		plan.Features = unlimitedFeatures
		plan.Features.Teams = true
		plan.Features.Audit = true
		plan.Features.SSO = true
		plan.Features.Policies = true
		plan.Features.BusinessPolicies = true
		plan.Features.EnterprisePolicies = true
	}
	plan.Features.APIAccess = false
	plan.Features.PrioritySupport = false
}

func intValue(value int) *int {
	return &value
}

func defaultExpiryBehavior(code string) domain.ExpiryBehavior {
	if code == "free-monthly" || strings.HasPrefix(code, "pro-") {
		return domain.ExpiryBehaviorDowngradeToFree
	}
	return domain.ExpiryBehaviorFreeze
}

func defaultGraceDays(code string) int {
	if code == "free-monthly" {
		return 0
	}
	return 14
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
				UUID:           uuid.New(),
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
