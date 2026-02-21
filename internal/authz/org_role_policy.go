package authz

import "github.com/passwall/passwall-server/internal/domain"

func CanViewBilling(role domain.OrganizationRole) bool {
	return role == domain.OrgRoleOwner ||
		role == domain.OrgRoleAdmin ||
		role == domain.OrgRoleManager ||
		role == domain.OrgRoleBilling
}

func CanManageBilling(role domain.OrganizationRole) bool {
	return role == domain.OrgRoleOwner ||
		role == domain.OrgRoleAdmin ||
		role == domain.OrgRoleBilling
}

func CanAccessOrganizationSettings(role domain.OrganizationRole) bool {
	return role == domain.OrgRoleOwner ||
		role == domain.OrgRoleAdmin ||
		role == domain.OrgRoleManager
}

func CanViewManagementOverview(role domain.OrganizationRole) bool {
	return CanAccessOrganizationSettings(role)
}

func CanViewMemberDirectory(role domain.OrganizationRole) bool {
	return role == domain.OrgRoleOwner || role == domain.OrgRoleAdmin
}

func CanViewSecurityAndAudit(role domain.OrganizationRole) bool {
	return role == domain.OrgRoleOwner || role == domain.OrgRoleAdmin
}
