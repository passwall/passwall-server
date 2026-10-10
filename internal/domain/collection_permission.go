package domain

// CollectionPermission is the access level a user or team has on a
// collection. It is derived from (and stored as) the can_read / can_write /
// can_admin / hide_passwords flags so existing clients keep working.
type CollectionPermission string

const (
	CollectionPermissionNone                CollectionPermission = ""
	CollectionPermissionView                CollectionPermission = "view"
	CollectionPermissionViewExceptPasswords CollectionPermission = "view_except_passwords"
	CollectionPermissionEdit                CollectionPermission = "edit"
	CollectionPermissionEditExceptPasswords CollectionPermission = "edit_except_passwords"
	CollectionPermissionManage              CollectionPermission = "manage"
)

// IsValid reports whether p is a grantable permission.
func (p CollectionPermission) IsValid() bool {
	switch p {
	case CollectionPermissionView, CollectionPermissionViewExceptPasswords,
		CollectionPermissionEdit, CollectionPermissionEditExceptPasswords,
		CollectionPermissionManage:
		return true
	}
	return false
}

// CollectionPermissionFromFlags maps the stored flags to a permission level.
// Manage always includes passwords.
func CollectionPermissionFromFlags(canRead, canWrite, canAdmin, hidePasswords bool) CollectionPermission {
	switch {
	case canAdmin:
		return CollectionPermissionManage
	case canWrite && hidePasswords:
		return CollectionPermissionEditExceptPasswords
	case canWrite:
		return CollectionPermissionEdit
	case canRead && hidePasswords:
		return CollectionPermissionViewExceptPasswords
	case canRead:
		return CollectionPermissionView
	}
	return CollectionPermissionNone
}

// Flags returns the stored flag values for p.
func (p CollectionPermission) Flags() (canRead, canWrite, canAdmin, hidePasswords bool) {
	switch p {
	case CollectionPermissionView:
		return true, false, false, false
	case CollectionPermissionViewExceptPasswords:
		return true, false, false, true
	case CollectionPermissionEdit:
		return true, true, false, false
	case CollectionPermissionEditExceptPasswords:
		return true, true, false, true
	case CollectionPermissionManage:
		return true, true, true, false
	}
	return false, false, false, false
}

// ItemPermissions is what the caller may do with an organization item. The
// server computes it so clients render it instead of re-deriving access.
type ItemPermissions struct {
	View         bool `json:"view"`
	ViewPassword bool `json:"view_password"`
	Edit         bool `json:"edit"`
	EditPassword bool `json:"edit_password"`
	Manage       bool `json:"manage"`
	Delete       bool `json:"delete"`
	Share        bool `json:"share"`
}

// FullItemPermissions is used for owners, admins and access-all members.
func FullItemPermissions() *ItemPermissions {
	return &ItemPermissions{View: true, ViewPassword: true, Edit: true, EditPassword: true, Manage: true, Delete: true, Share: true}
}

// ItemPermissionsFor maps a collection permission to item permissions.
// Deleting and sharing follow today's rules: they need edit access.
func ItemPermissionsFor(p CollectionPermission) *ItemPermissions {
	canRead, canWrite, canAdmin, hide := p.Flags()
	edit := canWrite || canAdmin
	return &ItemPermissions{
		View:         canRead,
		ViewPassword: canRead && !hide,
		Edit:         edit,
		EditPassword: edit && !hide,
		Manage:       canAdmin,
		Delete:       edit,
		Share:        edit && !hide,
	}
}
