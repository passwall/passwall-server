package core

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/passwall/passwall-server/pkg/database"
	"github.com/passwall/passwall-server/pkg/logger"
)

// SeedDatabase seeds all necessary initial data
// This is idempotent - safe to run multiple times
func SeedDatabase(ctx context.Context, db database.Database, cfg *config.Config) error {
	logger.Infof("🌱 Seeding database...")
	logger.Infof("📦 Found %d plans in config", len(cfg.Stripe.Plans))

	// 1. Seed roles and permissions
	logger.Infof("👥 Seeding roles and permissions...")
	if err := gormrepo.SeedRolesAndPermissions(ctx, db.DB()); err != nil {
		logger.Warnf("⚠️  Roles/permissions seeding issue: %v", err)
		// Don't fail - might already exist
	} else {
		logger.Infof("✓ Roles and permissions seeded")
	}

	// 2. Seed subscription plans from config
	logger.Infof("💳 Seeding subscription plans from config...")
	if err := gormrepo.SeedPlans(ctx, db.DB(), cfg.Stripe.Plans); err != nil {
		return fmt.Errorf("failed to seed plans: %w", err)
	}
	logger.Infof("✓ Subscription plans seeded successfully")

	// 3. Create default subscriptions for existing organizations
	logger.Infof("🏢 Creating default subscriptions for existing organizations...")
	if err := gormrepo.SeedDefaultSubscriptions(ctx, db.DB()); err != nil {
		logger.Warnf("⚠️  Default subscriptions issue: %v", err)
		// Don't fail - might not have organizations yet
	} else {
		logger.Infof("✓ Default subscriptions created")
	}

	logger.Infof("✅ Database seeding completed successfully")
	return nil
}
