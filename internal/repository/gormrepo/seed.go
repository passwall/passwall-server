package gormrepo

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/constants"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedRolesAndPermissions creates initial roles and permissions if they don't exist
func SeedRolesAndPermissions(ctx context.Context, db *gorm.DB) error {
	// Check if roles already exist
	var count int64
	if err := db.WithContext(ctx).Model(&domain.Role{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check roles: %w", err)
	}

	// If roles exist, skip seeding
	if count > 0 {
		return nil
	}

	// Begin transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Create Permissions (using constants)
		permissions := []domain.Permission{
			// Users permissions
			{Name: constants.PermissionUsersRead, DisplayName: "View Users", Description: "Can view user list and details", Resource: "users", Action: "read"},
			{Name: constants.PermissionUsersCreate, DisplayName: "Create Users", Description: "Can create new users", Resource: "users", Action: "create"},
			{Name: constants.PermissionUsersUpdate, DisplayName: "Update Users", Description: "Can update user information", Resource: "users", Action: "update"},
			{Name: constants.PermissionUsersDelete, DisplayName: "Delete Users", Description: "Can delete users", Resource: "users", Action: "delete"},

			// Logins permissions
			{Name: constants.PermissionLoginsRead, DisplayName: "View Logins", Description: "Can view login credentials", Resource: "logins", Action: "read"},
			{Name: constants.PermissionLoginsCreate, DisplayName: "Create Logins", Description: "Can create new login credentials", Resource: "logins", Action: "create"},
			{Name: constants.PermissionLoginsUpdate, DisplayName: "Update Logins", Description: "Can update login credentials", Resource: "logins", Action: "update"},
			{Name: constants.PermissionLoginsDelete, DisplayName: "Delete Logins", Description: "Can delete login credentials", Resource: "logins", Action: "delete"},

			// Credit Cards permissions
			{Name: constants.PermissionCreditCardsRead, DisplayName: "View Credit Cards", Description: "Can view credit cards", Resource: "credit_cards", Action: "read"},
			{Name: constants.PermissionCreditCardsCreate, DisplayName: "Create Credit Cards", Description: "Can create credit cards", Resource: "credit_cards", Action: "create"},
			{Name: constants.PermissionCreditCardsUpdate, DisplayName: "Update Credit Cards", Description: "Can update credit cards", Resource: "credit_cards", Action: "update"},
			{Name: constants.PermissionCreditCardsDelete, DisplayName: "Delete Credit Cards", Description: "Can delete credit cards", Resource: "credit_cards", Action: "delete"},

			// Bank Accounts permissions
			{Name: constants.PermissionBankAccountsRead, DisplayName: "View Bank Accounts", Description: "Can view bank accounts", Resource: "bank_accounts", Action: "read"},
			{Name: constants.PermissionBankAccountsCreate, DisplayName: "Create Bank Accounts", Description: "Can create bank accounts", Resource: "bank_accounts", Action: "create"},
			{Name: constants.PermissionBankAccountsUpdate, DisplayName: "Update Bank Accounts", Description: "Can update bank accounts", Resource: "bank_accounts", Action: "update"},
			{Name: constants.PermissionBankAccountsDelete, DisplayName: "Delete Bank Accounts", Description: "Can delete bank accounts", Resource: "bank_accounts", Action: "delete"},

			// Notes permissions
			{Name: constants.PermissionNotesRead, DisplayName: "View Notes", Description: "Can view notes", Resource: "notes", Action: "read"},
			{Name: constants.PermissionNotesCreate, DisplayName: "Create Notes", Description: "Can create notes", Resource: "notes", Action: "create"},
			{Name: constants.PermissionNotesUpdate, DisplayName: "Update Notes", Description: "Can update notes", Resource: "notes", Action: "update"},
			{Name: constants.PermissionNotesDelete, DisplayName: "Delete Notes", Description: "Can delete notes", Resource: "notes", Action: "delete"},

			// Emails permissions
			{Name: constants.PermissionEmailsRead, DisplayName: "View Emails", Description: "Can view emails", Resource: "emails", Action: "read"},
			{Name: constants.PermissionEmailsCreate, DisplayName: "Create Emails", Description: "Can create emails", Resource: "emails", Action: "create"},
			{Name: constants.PermissionEmailsUpdate, DisplayName: "Update Emails", Description: "Can update emails", Resource: "emails", Action: "update"},
			{Name: constants.PermissionEmailsDelete, DisplayName: "Delete Emails", Description: "Can delete emails", Resource: "emails", Action: "delete"},
		}

		if err := tx.WithContext(ctx).Create(&permissions).Error; err != nil {
			return fmt.Errorf("failed to create permissions: %w", err)
		}

		// Create Roles with explicit IDs (using constants)
		roles := []domain.Role{
			{
				ID:          constants.RoleIDAdmin,
				Name:        constants.RoleAdmin,
				DisplayName: "Administrator",
				Description: "Full system access with all permissions",
			},
			{
				ID:          constants.RoleIDMember,
				Name:        constants.RoleMember,
				DisplayName: "Member",
				Description: "Standard user with limited access to own data",
			},
		}

		// Create roles
		for _, role := range roles {
			if err := tx.WithContext(ctx).Create(&role).Error; err != nil {
				return fmt.Errorf("failed to create role %s: %w", role.Name, err)
			}
		}

		// Fetch created roles for association
		var adminRole, memberRole domain.Role
		if err := tx.WithContext(ctx).Where("id = ?", constants.RoleIDAdmin).First(&adminRole).Error; err != nil {
			return fmt.Errorf("failed to fetch admin role: %w", err)
		}

		if err := tx.WithContext(ctx).Where("id = ?", constants.RoleIDMember).First(&memberRole).Error; err != nil {
			return fmt.Errorf("failed to fetch member role: %w", err)
		}

		// Assign ALL permissions to Admin
		var allPermissions []domain.Permission
		if err := tx.WithContext(ctx).Find(&allPermissions).Error; err != nil {
			return fmt.Errorf("failed to fetch permissions: %w", err)
		}

		if err := tx.WithContext(ctx).Model(&adminRole).Association("Permissions").Append(allPermissions); err != nil {
			return fmt.Errorf("failed to assign permissions to admin: %w", err)
		}

		// Assign limited permissions to Member (exclude users.*)
		var memberPermissions []domain.Permission
		if err := tx.WithContext(ctx).Where("resource IN ?", []string{"logins", "credit_cards", "bank_accounts", "notes", "emails"}).Find(&memberPermissions).Error; err != nil {
			return fmt.Errorf("failed to fetch member permissions: %w", err)
		}

		if err := tx.WithContext(ctx).Model(&memberRole).Association("Permissions").Append(memberPermissions); err != nil {
			return fmt.Errorf("failed to assign permissions to member: %w", err)
		}

		// Update existing users to have role_id = member if null
		if err := tx.WithContext(ctx).Model(&domain.User{}).Where("role_id IS NULL OR role_id = 0").Update("role_id", constants.RoleIDMember).Error; err != nil {
			return fmt.Errorf("failed to update existing users: %w", err)
		}

		return nil
	})
}

// SeedSuperAdmin creates the super admin user if it doesn't exist
func SeedSuperAdmin(ctx context.Context, db *gorm.DB, cfg *config.SuperAdminConfig) error {
	if cfg.Email == "" || cfg.Password == "" {
		return fmt.Errorf("super admin email and password must be configured")
	}

	// Check if super admin already exists
	var existingUser domain.User
	err := db.WithContext(ctx).Where("email = ?", cfg.Email).First(&existingUser).Error
	
	if err == nil {
		// Super admin exists, ensure it's marked as system user and is admin
		updates := map[string]interface{}{
			"is_system_user": true,
			"role_id":        constants.RoleIDAdmin,
		}
		if err := db.WithContext(ctx).Model(&existingUser).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update existing super admin: %w", err)
		}
		fmt.Printf("✓ Super admin already exists: %s (updated flags)\n", cfg.Email)
		return nil
	}
	
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check super admin existence: %w", err)
	}

	// Hash password (using bcrypt for compatibility with zero-knowledge auth)
	// For super admin, we use a simpler auth flow similar to old system
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash super admin password: %w", err)
	}

	// Generate schema name for super admin
	schema := fmt.Sprintf("user_%s", uuid.NewV4().String()[:8])

	// Create super admin user
	superAdmin := &domain.User{
		UUID:               uuid.NewV4(),
		Name:               cfg.Name,
		Email:              cfg.Email,
		MasterPasswordHash: string(hashedPassword),
		ProtectedUserKey:   "system",                   // Placeholder for system user
		Schema:             schema,
		RoleID:             constants.RoleIDAdmin,
		IsVerified:         true,
		IsSystemUser:       true, // Mark as system user (cannot be deleted)
		Language:           "en",
		KdfType:            domain.KdfTypePBKDF2,
		KdfIterations:      600000,
		KdfSalt:            "system", // Placeholder for system user
	}

	// Begin transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Create schema
		if err := tx.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)).Error; err != nil {
			return fmt.Errorf("failed to create super admin schema: %w", err)
		}

		// Create super admin user
		if err := tx.WithContext(ctx).Create(superAdmin).Error; err != nil {
			return fmt.Errorf("failed to create super admin user: %w", err)
		}

		fmt.Printf("✓ Super admin created: %s (email: %s)\n", superAdmin.Name, superAdmin.Email)
		return nil
	})
}
