package domain

import "time"

// Permission represents a specific action/access right
type Permission struct {
	ID          uint      `gorm:"primary_key" json:"id"`
	Name        string    `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	DisplayName string    `json:"display_name" gorm:"type:varchar(100);not null"`
	Description string    `json:"description" gorm:"type:text"`
	Resource    string    `json:"resource" gorm:"type:varchar(50);not null"` // e.g., "users", "logins"
	Action      string    `json:"action" gorm:"type:varchar(50);not null"`   // e.g., "read", "create", "update", "delete"
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName specifies the table name for Permission
func (Permission) TableName() string {
	return "permissions"
}

// Permission name constants
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

