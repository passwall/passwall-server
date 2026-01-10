package domain

// Permission constants
const (
	// Organization permissions
	PermOrgView              = "org:view"
	PermOrgUpdate            = "org:update"
	PermOrgDelete            = "org:delete"
	PermOrgTransferOwnership = "org:transfer_ownership"
	PermOrgManageSettings    = "org:manage_settings"

	// Member permissions
	PermMemberView       = "member:view"
	PermMemberInvite     = "member:invite"
	PermMemberRemove     = "member:remove"
	PermMemberUpdateRole = "member:update_role"

	// Billing permissions
	PermBillingView            = "billing:view"
	PermBillingUpdate          = "billing:update"
	PermBillingCancel          = "billing:cancel"
	PermBillingDownloadInvoice = "billing:download_invoice"

	// Collection permissions
	PermCollectionCreate = "collection:create"
	PermCollectionView   = "collection:view"
	PermCollectionUpdate = "collection:update"
	PermCollectionDelete = "collection:delete"

	// Item permissions
	PermItemCreate = "item:create"
	PermItemView   = "item:view"
	PermItemUpdate = "item:update"
	PermItemDelete = "item:delete"
	PermItemShare  = "item:share"
	PermItemExport = "item:export"

	// Activity/Audit permissions
	PermActivityView   = "activity:view"
	PermActivityExport = "activity:export"

	// Security permissions
	PermSecurityRotateKeys     = "security:rotate_keys"
	PermSecurityRevokeSessions = "security:revoke_sessions"
)

// PermissionMatrix defines permissions for each role
var PermissionMatrix = map[OrganizationRole][]string{
	OrgRoleOwner: {
		// Organization
		PermOrgView, PermOrgUpdate, PermOrgDelete, PermOrgTransferOwnership, PermOrgManageSettings,
		// Members
		PermMemberView, PermMemberInvite, PermMemberRemove, PermMemberUpdateRole,
		// Billing
		PermBillingView, PermBillingUpdate, PermBillingCancel, PermBillingDownloadInvoice,
		// Collections
		PermCollectionCreate, PermCollectionView, PermCollectionUpdate, PermCollectionDelete,
		// Items
		PermItemCreate, PermItemView, PermItemUpdate, PermItemDelete, PermItemShare, PermItemExport,
		// Activity
		PermActivityView, PermActivityExport,
		// Security
		PermSecurityRotateKeys, PermSecurityRevokeSessions,
	},
	OrgRoleAdmin: {
		// Organization
		PermOrgView, PermOrgUpdate,
		// Members
		PermMemberView, PermMemberInvite, PermMemberRemove,
		// Collections
		PermCollectionCreate, PermCollectionView, PermCollectionUpdate, PermCollectionDelete,
		// Items
		PermItemCreate, PermItemView, PermItemUpdate, PermItemDelete, PermItemShare, PermItemExport,
		// Activity
		PermActivityView,
		// Security
		PermSecurityRevokeSessions,
	},
	OrgRoleManager: {
		// Organization
		PermOrgView,
		// Members
		PermMemberView,
		// Collections
		PermCollectionView, PermCollectionUpdate,
		// Items
		PermItemCreate, PermItemView, PermItemUpdate, PermItemDelete, PermItemShare,
	},
	OrgRoleMember: {
		// Collections
		PermCollectionView,
		// Items
		PermItemCreate, PermItemView, PermItemUpdate, PermItemDelete,
	},
	OrgRoleBilling: {
		// Organization
		PermOrgView,
		// Billing
		PermBillingView, PermBillingUpdate, PermBillingDownloadInvoice,
	},
	// Read-only role (subscription expired override)
	"read_only": {
		// Organization
		PermOrgView,
		// Collections
		PermCollectionView,
		// Items
		PermItemView,
	},
}

// Can checks if a role has a specific permission
func Can(role OrganizationRole, permission string) bool {
	perms, ok := PermissionMatrix[role]
	if !ok {
		return false
	}

	for _, p := range perms {
		if p == permission {
			return true
		}
	}

	return false
}

// GetPermissions returns all permissions for a role
func GetPermissions(role OrganizationRole) []string {
	perms, ok := PermissionMatrix[role]
	if !ok {
		return []string{}
	}
	return perms
}

// IsWritePermission checks if a permission is a write operation
func IsWritePermission(permission string) bool {
	writePerms := []string{
		PermOrgUpdate, PermOrgDelete, PermOrgTransferOwnership, PermOrgManageSettings,
		PermMemberInvite, PermMemberRemove, PermMemberUpdateRole,
		PermBillingUpdate, PermBillingCancel,
		PermCollectionCreate, PermCollectionUpdate, PermCollectionDelete,
		PermItemCreate, PermItemUpdate, PermItemDelete, PermItemShare,
		PermSecurityRotateKeys, PermSecurityRevokeSessions,
	}

	for _, wp := range writePerms {
		if permission == wp {
			return true
		}
	}

	return false
}

