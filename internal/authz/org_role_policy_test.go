package authz

import (
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
)

func TestCanViewBilling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role domain.OrganizationRole
		want bool
	}{
		{name: "owner", role: domain.OrgRoleOwner, want: true},
		{name: "admin", role: domain.OrgRoleAdmin, want: true},
		{name: "manager", role: domain.OrgRoleManager, want: true},
		{name: "billing", role: domain.OrgRoleBilling, want: true},
		{name: "member", role: domain.OrgRoleMember, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CanViewBilling(tt.role); got != tt.want {
				t.Fatalf("CanViewBilling(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestCanManageBilling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role domain.OrganizationRole
		want bool
	}{
		{name: "owner", role: domain.OrgRoleOwner, want: true},
		{name: "admin", role: domain.OrgRoleAdmin, want: true},
		{name: "billing", role: domain.OrgRoleBilling, want: true},
		{name: "manager", role: domain.OrgRoleManager, want: false},
		{name: "member", role: domain.OrgRoleMember, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CanManageBilling(tt.role); got != tt.want {
				t.Fatalf("CanManageBilling(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestOverviewVisibilityMatrix(t *testing.T) {
	t.Parallel()

	type expected struct {
		management bool
		members    bool
		security   bool
		settings   bool
	}

	tests := []struct {
		name string
		role domain.OrganizationRole
		want expected
	}{
		{
			name: "owner",
			role: domain.OrgRoleOwner,
			want: expected{management: true, members: true, security: true, settings: true},
		},
		{
			name: "admin",
			role: domain.OrgRoleAdmin,
			want: expected{management: true, members: true, security: true, settings: true},
		},
		{
			name: "manager",
			role: domain.OrgRoleManager,
			want: expected{management: true, members: false, security: false, settings: true},
		},
		{
			name: "billing",
			role: domain.OrgRoleBilling,
			want: expected{management: false, members: false, security: false, settings: false},
		},
		{
			name: "member",
			role: domain.OrgRoleMember,
			want: expected{management: false, members: false, security: false, settings: false},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := CanViewManagementOverview(tt.role); got != tt.want.management {
				t.Fatalf("CanViewManagementOverview(%q) = %v, want %v", tt.role, got, tt.want.management)
			}
			if got := CanViewMemberDirectory(tt.role); got != tt.want.members {
				t.Fatalf("CanViewMemberDirectory(%q) = %v, want %v", tt.role, got, tt.want.members)
			}
			if got := CanViewSecurityAndAudit(tt.role); got != tt.want.security {
				t.Fatalf("CanViewSecurityAndAudit(%q) = %v, want %v", tt.role, got, tt.want.security)
			}
			if got := CanAccessOrganizationSettings(tt.role); got != tt.want.settings {
				t.Fatalf("CanAccessOrganizationSettings(%q) = %v, want %v", tt.role, got, tt.want.settings)
			}
		})
	}
}
