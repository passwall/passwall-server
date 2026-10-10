package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository/gormrepo"
	"github.com/stretchr/testify/require"
)

func enablePolicy(t *testing.T, f *accessFixture, orgID uint, policyType domain.PolicyType) {
	t.Helper()
	require.NoError(t, f.db.Exec(
		`INSERT INTO organization_policies (uuid, organization_id, type, enabled, data, created_at, updated_at)
		 VALUES (?, ?, ?, 1, CAST('{}' AS BLOB), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		uuid.New().String(), orgID, string(policyType),
	).Error)
}

func TestSingleOrganizationIgnoresPersonalVaults(t *testing.T) {
	f := newAccessFixture(t)
	require.NoError(t, f.db.AutoMigrate(&domain.OrganizationPolicy{}))
	f.svc.policyRepo = gormrepo.NewOrganizationPolicyRepository(f.db)
	ctx := context.Background()

	enablePolicy(t, f, f.org.ID, domain.PolicySingleOrganization)

	// Every user owns a personal vault; that alone must not block joining.
	user, _ := f.member(t, &domain.Organization{ID: 0}, "solo@example.com", domain.OrgRoleOwner, domain.OrgUserStatusConfirmed)
	personal := &domain.Organization{UUID: uuid.New(), PublicID: "personal0001", Name: "Personal Vault", IsPersonal: true}
	require.NoError(t, f.db.Create(personal).Error)
	require.NoError(t, f.db.Model(&domain.OrganizationUser{}).Where("user_id = ?", user.ID).Update("organization_id", personal.ID).Error)
	require.NoError(t, f.svc.checkSingleOrganizationPolicy(ctx, f.org.ID, user.ID))

	// A membership in another shared organization still blocks it.
	_, _ = f.member(t, f.otherOrg, "busy@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	var busy domain.User
	require.NoError(t, f.db.Where("email = ?", "busy@example.com").First(&busy).Error)
	require.Error(t, f.svc.checkSingleOrganizationPolicy(ctx, f.org.ID, busy.ID))

	// And a shared organization enforcing the policy blocks joining others.
	enablePolicy(t, f, f.otherOrg.ID, domain.PolicySingleOrganization)
	third := &domain.Organization{UUID: uuid.New(), PublicID: "orgpublic003", Name: "Third"}
	require.NoError(t, f.db.Create(third).Error)
	require.Error(t, f.svc.checkSingleOrganizationPolicy(ctx, third.ID, busy.ID))
}

func TestRemoveSendAppliesToPersonalSends(t *testing.T) {
	f := newAccessFixture(t)
	require.NoError(t, f.db.AutoMigrate(&domain.OrganizationPolicy{}))
	sends := &sendService{
		orgUserRepo: gormrepo.NewOrganizationUserRepository(f.db),
		policyRepo:  gormrepo.NewOrganizationPolicyRepository(f.db),
	}
	ctx := context.Background()
	member, _ := f.member(t, f.org, "sender@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	admin, _ := f.member(t, f.org, "sendadmin@example.com", domain.OrgRoleAdmin, domain.OrgUserStatusConfirmed)

	require.NoError(t, sends.checkRemoveSendPolicy(ctx, member.ID))
	enablePolicy(t, f, f.org.ID, domain.PolicyRemoveSend)

	require.ErrorIs(t, sends.checkRemoveSendPolicy(ctx, member.ID), ErrSendDisabledByPolicy)
	require.NoError(t, sends.checkRemoveSendPolicy(ctx, admin.ID), "owners and admins are exempt")
}

func TestDisablePersonalVaultBlocksPersonalItems(t *testing.T) {
	f := newAccessFixture(t)
	require.NoError(t, f.db.AutoMigrate(&domain.OrganizationPolicy{}))
	policyRepo := gormrepo.NewOrganizationPolicyRepository(f.db)
	orgUserRepo := gormrepo.NewOrganizationUserRepository(f.db)
	enforcement := NewPolicyEnforcementService(NewOrganizationPolicyService(policyRepo, orgUserRepo, nil, noopLogger{}), orgUserRepo)
	ctx := context.Background()

	user, _ := f.member(t, f.org, "pv@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	personal := &domain.Organization{UUID: uuid.New(), PublicID: "personal0002", Name: "Personal Vault", IsPersonal: true}
	require.NoError(t, f.db.Create(personal).Error)
	require.NoError(t, f.db.Create(&domain.OrganizationUser{UUID: uuid.New(), OrganizationID: personal.ID, UserID: user.ID, Role: domain.OrgRoleOwner, Status: domain.OrgUserStatusConfirmed, EncryptedOrgKey: "2.k"}).Error)

	require.NoError(t, enforcement.CheckPersonalVaultItemAllowed(ctx, personal.ID, user.ID))
	enablePolicy(t, f, f.org.ID, domain.PolicyDisablePersonalVault)
	require.ErrorIs(t, enforcement.CheckPersonalVaultItemAllowed(ctx, personal.ID, user.ID), ErrPersonalVaultDisabled)
	require.NoError(t, enforcement.CheckPersonalVaultItemAllowed(ctx, f.org.ID, user.ID), "organization items stay allowed")
}

func TestGetEffectivePoliciesMergesAndExemptsAdmins(t *testing.T) {
	f := newAccessFixture(t)
	require.NoError(t, f.db.AutoMigrate(&domain.OrganizationPolicy{}))
	policyRepo := gormrepo.NewOrganizationPolicyRepository(f.db)
	orgUserRepo := gormrepo.NewOrganizationUserRepository(f.db)
	svc := NewOrganizationPolicyService(policyRepo, orgUserRepo, nil, noopLogger{})
	ctx := context.Background()

	member, _ := f.member(t, f.org, "eff@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	require.NoError(t, f.db.Create(&domain.OrganizationUser{UUID: uuid.New(), OrganizationID: f.otherOrg.ID, UserID: member.ID, Role: domain.OrgRoleAdmin, Status: domain.OrgUserStatusConfirmed, EncryptedOrgKey: "2.k"}).Error)

	insert := func(orgID uint, policyType domain.PolicyType, data string) {
		require.NoError(t, f.db.Exec(
			`INSERT INTO organization_policies (uuid, organization_id, type, enabled, data, created_at, updated_at)
			 VALUES (?, ?, ?, 1, CAST(? AS BLOB), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			uuid.New().String(), orgID, string(policyType), data,
		).Error)
	}
	insert(f.org.ID, domain.PolicySessionTimeout, `{"max_timeout_minutes":60,"timeout_action":"lock"}`)
	insert(f.otherOrg.ID, domain.PolicySessionTimeout, `{"max_timeout_minutes":10,"timeout_action":"lock"}`)
	insert(f.otherOrg.ID, domain.PolicyRemoveSend, `{}`)         // admin there: exempt
	insert(f.org.ID, domain.PolicyFirewallRules, `{"rules":[]}`) // server-only
	insert(f.org.ID, domain.PolicyAccountRecovery, `{}`)         // not available

	resp, err := svc.GetEffectivePolicies(ctx, member.ID)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1, "%+v", resp.Policies)
	p := resp.Policies[0]
	require.Equal(t, domain.PolicySessionTimeout, p.Type)
	require.EqualValues(t, 10, p.Data["max_timeout_minutes"])
	require.Len(t, p.Organizations, 2)

	insert(f.org.ID, domain.PolicyRemoveSend, `{}`) // member here: applies
	resp, err = svc.GetEffectivePolicies(ctx, member.ID)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 2)
	require.Equal(t, domain.PolicyRemoveSend, resp.Policies[0].Type)
	require.Len(t, resp.Policies[0].Organizations, 1)
	require.Equal(t, f.org.ID, resp.Policies[0].Organizations[0].ID)
}

func TestSingleOrganizationBlocksCreatingAnotherOrg(t *testing.T) {
	f := newAccessFixture(t)
	require.NoError(t, f.db.AutoMigrate(&domain.OrganizationPolicy{}))
	f.svc.policyRepo = gormrepo.NewOrganizationPolicyRepository(f.db)
	ctx := context.Background()

	member, _ := f.member(t, f.org, "creator@example.com", domain.OrgRoleMember, domain.OrgUserStatusConfirmed)
	admin, _ := f.member(t, f.org, "creatoradmin@example.com", domain.OrgRoleAdmin, domain.OrgUserStatusConfirmed)
	require.NoError(t, f.svc.checkCreateAllowedBySingleOrgPolicy(ctx, member.ID))

	enablePolicy(t, f, f.org.ID, domain.PolicySingleOrganization)
	require.ErrorIs(t, f.svc.checkCreateAllowedBySingleOrgPolicy(ctx, member.ID), ErrSingleOrganizationPolicy)
	require.NoError(t, f.svc.checkCreateAllowedBySingleOrgPolicy(ctx, admin.ID), "owners and admins are exempt")
}
