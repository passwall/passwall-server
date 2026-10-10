package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/passwall/passwall-server/internal/domain"
)

// ─── Login code exchange ────────────────────────────────────────────────────────

func TestExchangeLoginCode_HappyPathAndSingleUse(t *testing.T) {
	t.Parallel()
	svc, code := newExchangeEnv(t, nil)
	ctx := context.Background()

	resp, err := svc.ExchangeLoginCode(ctx, &domain.SSOExchangeRequest{Code: code, CodeVerifier: testVerifier})
	require.NoError(t, err)
	assert.Equal(t, "test-access-token", resp.AccessToken)
	assert.Equal(t, "acmePublic01", resp.Organization.PublicID)

	_, err = svc.ExchangeLoginCode(ctx, &domain.SSOExchangeRequest{Code: code, CodeVerifier: testVerifier})
	assert.ErrorIs(t, err, ErrSSOInvalidLoginCode, "a code works once")
}

func TestExchangeLoginCode_WrongVerifierBurnsCode(t *testing.T) {
	t.Parallel()
	svc, code := newExchangeEnv(t, nil)
	ctx := context.Background()

	wrong := "x" + testVerifier[1:]
	_, err := svc.ExchangeLoginCode(ctx, &domain.SSOExchangeRequest{Code: code, CodeVerifier: wrong})
	assert.ErrorIs(t, err, ErrSSOInvalidLoginCode)

	_, err = svc.ExchangeLoginCode(ctx, &domain.SSOExchangeRequest{Code: code, CodeVerifier: testVerifier})
	assert.ErrorIs(t, err, ErrSSOInvalidLoginCode, "a stolen code cannot be retried")
}

func TestExchangeLoginCode_ExpiredCode(t *testing.T) {
	t.Parallel()
	svc, code := newExchangeEnv(t, nil)
	for _, c := range svc.codeRepo.(*fakeSSOLoginCodeRepo).codes {
		c.ExpiresAt = time.Now().Add(-time.Second)
	}
	_, err := svc.ExchangeLoginCode(context.Background(), &domain.SSOExchangeRequest{Code: code, CodeVerifier: testVerifier})
	assert.ErrorIs(t, err, ErrSSOInvalidLoginCode)
}

func TestExchangeLoginCode_TwoFactorUserGetsChallenge(t *testing.T) {
	t.Parallel()
	svc, code := newExchangeEnv(t, &fakeAuthService{twoFactor: true})
	resp, err := svc.ExchangeLoginCode(context.Background(), &domain.SSOExchangeRequest{Code: code, CodeVerifier: testVerifier})
	require.NoError(t, err)
	assert.True(t, resp.TwoFactorRequired)
	assert.Empty(t, resp.AccessToken, "no session before the second factor")
}

func TestExchangeLoginCode_ConnectionDeactivatedAfterCallback(t *testing.T) {
	t.Parallel()
	svc, code := newExchangeEnv(t, nil)
	conn, _ := svc.connRepo.GetByID(context.Background(), testConnID)
	conn.Status = domain.SSOStatusInactive
	_, err := svc.ExchangeLoginCode(context.Background(), &domain.SSOExchangeRequest{Code: code, CodeVerifier: testVerifier})
	assert.ErrorIs(t, err, ErrSSOConnectionInactive)
}

// ─── Domain verification ────────────────────────────────────────────────────────

type fakeTXTResolver struct {
	records map[string][]string
	err     error
}

func (f fakeTXTResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.records[name], nil
}

func newVerifyEnv(t *testing.T, records map[string][]string) (*ssoService, *fakeSSOConnRepo, *domain.SSOConnection) {
	t.Helper()
	connRepo := newFakeSSOConnRepo()
	conn := &domain.SSOConnection{
		ID:                      testConnID,
		OrganizationID:          testOrgID,
		Protocol:                domain.SSOProtocolOIDC,
		Domain:                  "acme.com",
		DomainVerificationToken: "abc123",
		Status:                  domain.SSOStatusDraft,
		OIDCConfig:              &domain.OIDCConfig{Issuer: "https://idp.acme.com", ClientID: "client", ClientSecret: "secret"},
	}
	connRepo.addUnverified(conn)
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)
	svc.resolver = fakeTXTResolver{records: records}
	return svc, connRepo, conn
}

func TestVerifyDomain_RecordMissing(t *testing.T) {
	t.Parallel()
	svc, _, conn := newVerifyEnv(t, map[string][]string{"_passwall-verification.acme.com": {"something-else"}})
	_, err := svc.VerifyDomain(context.Background(), conn.ID, testUserID)
	assert.ErrorIs(t, err, ErrSSODomainNotVerified)
	assert.False(t, conn.IsDomainVerified())
}

func TestVerifyDomain_LookupError(t *testing.T) {
	t.Parallel()
	svc, _, conn := newVerifyEnv(t, nil)
	svc.resolver = fakeTXTResolver{err: errors.New("NXDOMAIN")}
	_, err := svc.VerifyDomain(context.Background(), conn.ID, testUserID)
	assert.ErrorIs(t, err, ErrSSODomainNotVerified)
}

func TestVerifyDomain_Success(t *testing.T) {
	t.Parallel()
	svc, _, conn := newVerifyEnv(t, map[string][]string{
		"_passwall-verification.acme.com": {"v=spf1 -all", "passwall-verification=abc123"},
	})
	got, err := svc.VerifyDomain(context.Background(), conn.ID, testUserID)
	require.NoError(t, err)
	assert.True(t, got.IsDomainVerified())

	activated, err := svc.ActivateConnection(context.Background(), conn.ID, testUserID)
	require.NoError(t, err)
	assert.Equal(t, domain.SSOStatusActive, activated.Status)
}

func TestVerifyDomain_AlreadyVerifiedElsewhere(t *testing.T) {
	t.Parallel()
	svc, connRepo, conn := newVerifyEnv(t, map[string][]string{
		"_passwall-verification.acme.com": {"passwall-verification=abc123"},
	})
	connRepo.add(&domain.SSOConnection{ID: 77, OrganizationID: 99, Domain: "acme.com"})
	_, err := svc.VerifyDomain(context.Background(), conn.ID, testUserID)
	assert.ErrorIs(t, err, ErrSSODomainTaken)
}

func TestActivateConnection_RequiresVerifiedDomain(t *testing.T) {
	t.Parallel()
	svc, _, conn := newVerifyEnv(t, nil)
	_, err := svc.ActivateConnection(context.Background(), conn.ID, testUserID)
	assert.ErrorIs(t, err, ErrSSODomainNotVerified)
}

func TestUnverifiedConnection_CannotSignIn(t *testing.T) {
	t.Parallel()
	svc, _, conn := newVerifyEnv(t, nil)
	conn.Status = domain.SSOStatusActive // e.g. activated before verification existed

	_, err := svc.InitiateLogin(context.Background(), &domain.SSOInitiateRequest{Domain: "acme.com", CodeChallenge: testChallenge})
	assert.ErrorIs(t, err, ErrSSOConnectionNotFound)

	svc.stateRepo.(*fakeSSOStateRepo).states["s1"] = &domain.SSOState{State: "s1", ConnectionID: conn.ID, ExpiresAt: time.Now().Add(time.Minute)}
	_, err = svc.HandleOIDCCallback(context.Background(), "s1", "code")
	assert.ErrorIs(t, err, ErrSSOConnectionInactive)
}

func TestCreateConnection_UnverifiedClaimDoesNotBlockDomain(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.addUnverified(&domain.SSOConnection{ID: 1, OrganizationID: 99, Domain: "acme.com"})
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)
	conn, err := svc.CreateConnection(context.Background(), testOrgID, testUserID, &domain.CreateSSOConnectionRequest{
		Protocol:   domain.SSOProtocolOIDC,
		Name:       "Acme",
		Domain:     " ACME.com ",
		OIDCConfig: &domain.OIDCConfig{Issuer: "https://idp.acme.com", ClientID: "c"},
	})
	require.NoError(t, err)
	assert.Equal(t, "acme.com", conn.Domain)
	assert.NotEmpty(t, conn.DomainVerificationToken)
	assert.False(t, conn.AutoProvision, "provisioning is opt-in")
	assert.False(t, conn.JITProvisioning, "provisioning is opt-in")
	dto := domain.ToSSOConnectionDTO(conn)
	require.NotNil(t, dto.DomainVerification)
	assert.Equal(t, "_passwall-verification.acme.com", dto.DomainVerification.Name)
	assert.Equal(t, "passwall-verification="+conn.DomainVerificationToken, dto.DomainVerification.Value)
}

func TestCreateConnection_RejectsElevatedDefaultRoleAndBadDomain(t *testing.T) {
	t.Parallel()
	svc := newTestSSOService(nil, nil, nil, nil, nil, nil)
	oidc := &domain.OIDCConfig{Issuer: "https://idp.acme.com", ClientID: "c"}
	for _, role := range []domain.OrganizationRole{domain.OrgRoleAdmin, domain.OrgRoleOwner, domain.OrgRoleBilling} {
		_, err := svc.CreateConnection(context.Background(), testOrgID, testUserID, &domain.CreateSSOConnectionRequest{
			Protocol: domain.SSOProtocolOIDC, Name: "x", Domain: "acme.com", OIDCConfig: oidc, DefaultRole: role,
		})
		assert.ErrorIs(t, err, ErrSSODefaultRoleNotAllowed, role)
	}
	for _, d := range []string{"localhost", "https://acme.com", "acme..com", "-acme.com", "ac me.com"} {
		_, err := svc.CreateConnection(context.Background(), testOrgID, testUserID, &domain.CreateSSOConnectionRequest{
			Protocol: domain.SSOProtocolOIDC, Name: "x", Domain: d, OIDCConfig: oidc,
		})
		assert.ErrorIs(t, err, ErrSSOInvalidDomain, d)
	}
}

// ─── UpdateConnection rules ─────────────────────────────────────────────────────

func TestUpdateConnection_CannotActivateDirectly(t *testing.T) {
	t.Parallel()
	svc, _, conn := newVerifyEnv(t, nil)
	active := domain.SSOStatusActive
	_, err := svc.UpdateConnection(context.Background(), conn.ID, testUserID, &domain.UpdateSSOConnectionRequest{Status: &active})
	assert.ErrorIs(t, err, ErrSSOConfigInvalid)
	assert.Equal(t, domain.SSOStatusDraft, conn.Status)
}

func TestUpdateConnection_DomainChangeResetsVerification(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	conn := &domain.SSOConnection{ID: testConnID, OrganizationID: testOrgID, Domain: "acme.com", DomainVerificationToken: "old", Status: domain.SSOStatusActive}
	connRepo.add(conn)
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)

	newDomain := "acme.io"
	got, err := svc.UpdateConnection(context.Background(), conn.ID, testUserID, &domain.UpdateSSOConnectionRequest{Domain: &newDomain})
	require.NoError(t, err)
	assert.Equal(t, "acme.io", got.Domain)
	assert.False(t, got.IsDomainVerified())
	assert.NotEqual(t, "old", got.DomainVerificationToken)
	assert.Equal(t, domain.SSOStatusDraft, got.Status)
}

func TestUpdateConnection_KeepsSecretsWhenOmitted(t *testing.T) {
	t.Parallel()
	svc, _, conn := newVerifyEnv(t, nil)
	_, err := svc.UpdateConnection(context.Background(), conn.ID, testUserID, &domain.UpdateSSOConnectionRequest{
		OIDCConfig: &domain.OIDCConfig{Issuer: "https://idp2.acme.com", ClientID: "client2"},
	})
	require.NoError(t, err)
	assert.Equal(t, "secret", conn.OIDCConfig.ClientSecret)
	assert.Equal(t, "client2", conn.OIDCConfig.ClientID)

	admin := domain.OrgRoleAdmin
	_, err = svc.UpdateConnection(context.Background(), conn.ID, testUserID, &domain.UpdateSSOConnectionRequest{DefaultRole: &admin})
	assert.ErrorIs(t, err, ErrSSODefaultRoleNotAllowed)
}

// ─── JIT provisioning ───────────────────────────────────────────────────────────

type denyJoinPolicies struct{}

func (denyJoinPolicies) CheckJoinPolicies(context.Context, uint, uint) error {
	return errors.New("single organization policy")
}

func TestCompleteSSOLogin_JITRespectsJoinPolicies(t *testing.T) {
	t.Parallel()
	userRepo := newFakeUserRepo()
	userRepo.add(&domain.User{ID: testUserID, IsVerified: true, Email: "alice@acme.com"})
	orgUserRepo := newFakeOrgUserRepo()
	svc := newTestSSOService(nil, nil, userRepo, orgUserRepo, nil, nil)
	svc.joinPolicies = denyJoinPolicies{}

	conn := &domain.SSOConnection{ID: testConnID, OrganizationID: testOrgID, JITProvisioning: true, DefaultRole: domain.OrgRoleOwner}
	_, err := svc.completeSSOLogin(context.Background(), conn, testLoginState(), "alice@acme.com")
	assert.Error(t, err)
	_, err = orgUserRepo.GetByOrgAndUser(context.Background(), testOrgID, testUserID)
	assert.Error(t, err, "no membership is created")
}

func TestCompleteSSOLogin_JITAlwaysMember(t *testing.T) {
	t.Parallel()
	userRepo := newFakeUserRepo()
	userRepo.add(&domain.User{ID: testUserID, IsVerified: true, Email: "alice@acme.com"})
	orgUserRepo := newFakeOrgUserRepo()
	svc := newTestSSOService(nil, nil, userRepo, orgUserRepo, nil, nil)

	// A legacy row with an elevated default role must not grant it.
	conn := &domain.SSOConnection{ID: testConnID, OrganizationID: testOrgID, JITProvisioning: true, DefaultRole: domain.OrgRoleOwner}
	_, err := svc.completeSSOLogin(context.Background(), conn, testLoginState(), "alice@acme.com")
	require.NoError(t, err)
	member, err := orgUserRepo.GetByOrgAndUser(context.Background(), testOrgID, testUserID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrgRoleMember, member.Role)
}

func TestCompleteSSOLogin_UnverifiedPasswallAccount(t *testing.T) {
	t.Parallel()
	userRepo := newFakeUserRepo()
	userRepo.add(&domain.User{ID: testUserID, Email: "alice@acme.com"})
	svc := newTestSSOService(nil, nil, userRepo, nil, nil, nil)
	conn := &domain.SSOConnection{ID: testConnID, OrganizationID: testOrgID, JITProvisioning: true}
	_, err := svc.completeSSOLogin(context.Background(), conn, testLoginState(), "alice@acme.com")
	assert.ErrorIs(t, err, ErrSSOUserNotFound)
}
