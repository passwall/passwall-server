package constants

// Context keys for storing user information
const (
	ContextKeyUserID    = "user_id"
	ContextKeyEmail     = "email"
	ContextKeySchema    = "schema"
	ContextKeyUserRole  = "user_role"
	ContextKeyTokenUUID = "token_uuid"
)

// User role names
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// User role IDs (must match database)
const (
	RoleIDAdmin  uint = 1
	RoleIDMember uint = 2
)

// Permission names
const (
	// Users permissions
	PermissionUsersRead   = "users.read"
	PermissionUsersCreate = "users.create"
	PermissionUsersUpdate = "users.update"
	PermissionUsersDelete = "users.delete"

	// Logins permissions
	PermissionLoginsRead   = "logins.read"
	PermissionLoginsCreate = "logins.create"
	PermissionLoginsUpdate = "logins.update"
	PermissionLoginsDelete = "logins.delete"

	// Credit Cards permissions
	PermissionCreditCardsRead   = "credit_cards.read"
	PermissionCreditCardsCreate = "credit_cards.create"
	PermissionCreditCardsUpdate = "credit_cards.update"
	PermissionCreditCardsDelete = "credit_cards.delete"

	// Bank Accounts permissions
	PermissionBankAccountsRead   = "bank_accounts.read"
	PermissionBankAccountsCreate = "bank_accounts.create"
	PermissionBankAccountsUpdate = "bank_accounts.update"
	PermissionBankAccountsDelete = "bank_accounts.delete"

	// Notes permissions
	PermissionNotesRead   = "notes.read"
	PermissionNotesCreate = "notes.create"
	PermissionNotesUpdate = "notes.update"
	PermissionNotesDelete = "notes.delete"

	// Emails permissions
	PermissionEmailsRead   = "emails.read"
	PermissionEmailsCreate = "emails.create"
	PermissionEmailsUpdate = "emails.update"
	PermissionEmailsDelete = "emails.delete"
)

// IsValidRole checks if a role is valid
func IsValidRole(role string) bool {
	return role == RoleAdmin || role == RoleMember
}

// IsAdmin checks if a role is admin
func IsAdmin(role string) bool {
	return role == RoleAdmin
}

// GetRoleID returns role ID from role name
func GetRoleID(roleName string) uint {
	if roleName == RoleAdmin {
		return RoleIDAdmin
	}
	return RoleIDMember
}

// GetRoleName returns role name from role ID
func GetRoleName(roleID uint) string {
	if roleID == RoleIDAdmin {
		return RoleAdmin
	}
	return RoleMember
}
