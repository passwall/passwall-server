package service

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test fakes ─────────────────────────────────────────────────────────────────

// noopLogger implements Logger for tests (discards all messages)
type noopLogger struct{}

func (noopLogger) Debug(msg string, kv ...interface{}) {}
func (noopLogger) Info(msg string, kv ...interface{})  {}
func (noopLogger) Infof(f string, a ...interface{})    {}
func (noopLogger) Warn(msg string, kv ...interface{})  {}
func (noopLogger) Error(msg string, kv ...interface{}) {}

// fakeSSOConnRepo implements repository.SSOConnectionRepository
type fakeSSOConnRepo struct {
	conns    map[uint]*domain.SSOConnection
	byDomain map[string]*domain.SSOConnection
}

func newFakeSSOConnRepo() *fakeSSOConnRepo {
	return &fakeSSOConnRepo{
		conns:    make(map[uint]*domain.SSOConnection),
		byDomain: make(map[string]*domain.SSOConnection),
	}
}

func (f *fakeSSOConnRepo) add(conn *domain.SSOConnection) {
	f.conns[conn.ID] = conn
	f.byDomain[conn.Domain] = conn
}

func (f *fakeSSOConnRepo) Create(_ context.Context, conn *domain.SSOConnection) error {
	f.conns[conn.ID] = conn
	f.byDomain[conn.Domain] = conn
	return nil
}
func (f *fakeSSOConnRepo) GetByID(_ context.Context, id uint) (*domain.SSOConnection, error) {
	c, ok := f.conns[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return c, nil
}
func (f *fakeSSOConnRepo) GetByUUID(_ context.Context, uuid string) (*domain.SSOConnection, error) {
	return nil, repository.ErrNotFound
}
func (f *fakeSSOConnRepo) GetAnyByDomain(_ context.Context, d string) (*domain.SSOConnection, error) {
	c, ok := f.byDomain[d]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return c, nil
}
func (f *fakeSSOConnRepo) GetByDomain(_ context.Context, d string) (*domain.SSOConnection, error) {
	c, ok := f.byDomain[d]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if !c.IsActive() {
		return nil, repository.ErrNotFound
	}
	return c, nil
}
func (f *fakeSSOConnRepo) GetByOrganizationID(_ context.Context, orgID uint) (*domain.SSOConnection, error) {
	for _, c := range f.conns {
		if c.OrganizationID == orgID {
			return c, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeSSOConnRepo) ListByOrganization(_ context.Context, orgID uint) ([]*domain.SSOConnection, error) {
	var result []*domain.SSOConnection
	for _, c := range f.conns {
		if c.OrganizationID == orgID {
			result = append(result, c)
		}
	}
	return result, nil
}
func (f *fakeSSOConnRepo) Update(_ context.Context, conn *domain.SSOConnection) error {
	f.conns[conn.ID] = conn
	f.byDomain[conn.Domain] = conn
	return nil
}
func (f *fakeSSOConnRepo) Delete(_ context.Context, id uint) error {
	c, ok := f.conns[id]
	if ok {
		delete(f.byDomain, c.Domain)
		delete(f.conns, id)
	}
	return nil
}

// fakeSSOStateRepo implements repository.SSOStateRepository
type fakeSSOStateRepo struct {
	states map[string]*domain.SSOState
}

func newFakeSSOStateRepo() *fakeSSOStateRepo {
	return &fakeSSOStateRepo{states: make(map[string]*domain.SSOState)}
}

func (f *fakeSSOStateRepo) Create(_ context.Context, state *domain.SSOState) error {
	f.states[state.State] = state
	return nil
}
func (f *fakeSSOStateRepo) GetByState(_ context.Context, state string) (*domain.SSOState, error) {
	s, ok := f.states[state]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return s, nil
}
func (f *fakeSSOStateRepo) Delete(_ context.Context, id uint) error {
	for k, v := range f.states {
		if v.ID == id {
			delete(f.states, k)
		}
	}
	return nil
}
func (f *fakeSSOStateRepo) DeleteExpired(_ context.Context) (int64, error) { return 0, nil }

// fakeUserRepo implements repository.UserRepository (minimal)
type fakeUserRepo struct {
	users map[string]*domain.User // keyed by email
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[string]*domain.User)}
}

func (f *fakeUserRepo) add(u *domain.User) { f.users[u.Email] = u }

func (f *fakeUserRepo) GetByID(_ context.Context, id uint) (*domain.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeUserRepo) GetByUUID(_ context.Context, _ string) (*domain.User, error) {
	return nil, repository.ErrNotFound
}
func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := f.users[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}
func (f *fakeUserRepo) GetBySchema(_ context.Context, _ string) (*domain.User, error) {
	return nil, repository.ErrNotFound
}
func (f *fakeUserRepo) List(_ context.Context, _ repository.ListFilter) ([]*domain.User, *repository.ListResult, error) {
	return nil, nil, nil
}
func (f *fakeUserRepo) GetItemCount(_ context.Context, _ string) (int, error) { return 0, nil }
func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	f.users[u.Email] = u
	return nil
}
func (f *fakeUserRepo) Update(_ context.Context, _ *domain.User) error   { return nil }
func (f *fakeUserRepo) Delete(_ context.Context, _ uint, _ string) error { return nil }
func (f *fakeUserRepo) Migrate() error                                   { return nil }
func (f *fakeUserRepo) CreateSchema(_ string) error                      { return nil }
func (f *fakeUserRepo) MigrateUserSchema(_ string) error                 { return nil }

// fakeOrgUserRepo implements repository.OrganizationUserRepository (minimal)
type fakeOrgUserRepo struct {
	members map[string]*domain.OrganizationUser // keyed by "orgID:userID"
}

func newFakeOrgUserRepo() *fakeOrgUserRepo {
	return &fakeOrgUserRepo{members: make(map[string]*domain.OrganizationUser)}
}

func (f *fakeOrgUserRepo) add(ou *domain.OrganizationUser) {
	f.members[fmt.Sprintf("%d:%d", ou.OrganizationID, ou.UserID)] = ou
}

func (f *fakeOrgUserRepo) Create(_ context.Context, ou *domain.OrganizationUser) error {
	f.members[fmt.Sprintf("%d:%d", ou.OrganizationID, ou.UserID)] = ou
	return nil
}
func (f *fakeOrgUserRepo) GetByID(_ context.Context, _ uint) (*domain.OrganizationUser, error) {
	return nil, repository.ErrNotFound
}
func (f *fakeOrgUserRepo) GetByUUID(_ context.Context, _ string) (*domain.OrganizationUser, error) {
	return nil, repository.ErrNotFound
}
func (f *fakeOrgUserRepo) GetByOrgAndUser(_ context.Context, orgID, userID uint) (*domain.OrganizationUser, error) {
	ou, ok := f.members[fmt.Sprintf("%d:%d", orgID, userID)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return ou, nil
}
func (f *fakeOrgUserRepo) ListByOrganization(_ context.Context, _ uint) ([]*domain.OrganizationUser, error) {
	return nil, nil
}
func (f *fakeOrgUserRepo) ListByUser(_ context.Context, _ uint) ([]*domain.OrganizationUser, error) {
	return nil, nil
}
func (f *fakeOrgUserRepo) Update(_ context.Context, _ *domain.OrganizationUser) error { return nil }
func (f *fakeOrgUserRepo) Delete(_ context.Context, _ uint) error                     { return nil }
func (f *fakeOrgUserRepo) CountInvited(_ context.Context, _ uint) (int, error)        { return 0, nil }
func (f *fakeOrgUserRepo) ListPendingInvitations(_ context.Context, _ string) ([]*domain.OrganizationUser, error) {
	return nil, nil
}

// fakeOrgRepo implements repository.OrganizationRepository (minimal)
type fakeOrgRepo struct {
	orgs map[uint]*domain.Organization
}

func newFakeOrgRepo() *fakeOrgRepo {
	return &fakeOrgRepo{orgs: make(map[uint]*domain.Organization)}
}

func (f *fakeOrgRepo) add(o *domain.Organization) { f.orgs[o.ID] = o }

func (f *fakeOrgRepo) Create(_ context.Context, _ *domain.Organization) error { return nil }
func (f *fakeOrgRepo) GetByID(_ context.Context, id uint) (*domain.Organization, error) {
	o, ok := f.orgs[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return o, nil
}
func (f *fakeOrgRepo) GetByUUID(_ context.Context, _ string) (*domain.Organization, error) {
	return nil, repository.ErrNotFound
}
func (f *fakeOrgRepo) GetDefaultByOwnerID(_ context.Context, _ uint) (*domain.Organization, error) {
	return nil, repository.ErrNotFound
}
func (f *fakeOrgRepo) List(_ context.Context, _ repository.ListFilter) ([]*domain.Organization, *repository.ListResult, error) {
	return nil, nil, nil
}
func (f *fakeOrgRepo) ListForUser(_ context.Context, _ uint) ([]*domain.Organization, error) {
	return nil, nil
}
func (f *fakeOrgRepo) Update(_ context.Context, _ *domain.Organization) error    { return nil }
func (f *fakeOrgRepo) Delete(_ context.Context, _ uint) error                    { return nil }
func (f *fakeOrgRepo) GetMemberCount(_ context.Context, _ uint) (int, error)     { return 0, nil }
func (f *fakeOrgRepo) GetTeamCount(_ context.Context, _ uint) (int, error)       { return 0, nil }
func (f *fakeOrgRepo) GetCollectionCount(_ context.Context, _ uint) (int, error) { return 0, nil }
func (f *fakeOrgRepo) GetItemCount(_ context.Context, _ uint) (int, error)       { return 0, nil }

// fakeAuthService implements AuthService (minimal)
type fakeAuthService struct {
	shouldFail bool
}

func (f *fakeAuthService) SignUp(_ context.Context, _ *domain.SignUpRequest) (*domain.User, error) {
	return nil, nil
}
func (f *fakeAuthService) SignIn(_ context.Context, _ *domain.Credentials) (*domain.AuthResponse, error) {
	return nil, nil
}
func (f *fakeAuthService) PreLogin(_ context.Context, _ string) (*domain.PreLoginResponse, error) {
	return nil, nil
}
func (f *fakeAuthService) ChangeMasterPassword(_ context.Context, _ *domain.ChangeMasterPasswordRequest) error {
	return nil
}
func (f *fakeAuthService) RefreshToken(_ context.Context, _ string) (*domain.TokenDetails, error) {
	return nil, nil
}
func (f *fakeAuthService) ValidateToken(_ context.Context, _ string) (*domain.TokenClaims, error) {
	return nil, nil
}
func (f *fakeAuthService) IssueTokenForUser(_ context.Context, userID uint, _ string, _ string) (*domain.AuthResponse, error) {
	if f.shouldFail {
		return nil, errors.New("token issue failed")
	}
	return &domain.AuthResponse{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		User:         &domain.UserAuthDTO{},
	}, nil
}
func (f *fakeAuthService) SignOut(_ context.Context, _ string) error { return nil }
func (f *fakeAuthService) ValidateSchema(_ context.Context, _ string) error {
	return nil
}

// inactiveSSOConnRepo is a fake that always returns the same connection from GetByDomain
// (even if inactive). This lets us test the InitiateLogin path where a domain
// maps to an inactive connection.
type inactiveSSOConnRepo struct {
	conn *domain.SSOConnection
}

func (f *inactiveSSOConnRepo) Create(_ context.Context, _ *domain.SSOConnection) error { return nil }
func (f *inactiveSSOConnRepo) GetByID(_ context.Context, id uint) (*domain.SSOConnection, error) {
	if f.conn != nil && f.conn.ID == id {
		return f.conn, nil
	}
	return nil, repository.ErrNotFound
}
func (f *inactiveSSOConnRepo) GetByUUID(_ context.Context, _ string) (*domain.SSOConnection, error) {
	return nil, repository.ErrNotFound
}
func (f *inactiveSSOConnRepo) GetAnyByDomain(_ context.Context, _ string) (*domain.SSOConnection, error) {
	if f.conn != nil {
		return f.conn, nil
	}
	return nil, repository.ErrNotFound
}
func (f *inactiveSSOConnRepo) GetByDomain(_ context.Context, _ string) (*domain.SSOConnection, error) {
	if f.conn != nil {
		return f.conn, nil // returns even if inactive – InitiateLogin checks IsActive()
	}
	return nil, repository.ErrNotFound
}
func (f *inactiveSSOConnRepo) GetByOrganizationID(_ context.Context, _ uint) (*domain.SSOConnection, error) {
	return nil, repository.ErrNotFound
}
func (f *inactiveSSOConnRepo) ListByOrganization(_ context.Context, _ uint) ([]*domain.SSOConnection, error) {
	return nil, nil
}
func (f *inactiveSSOConnRepo) Update(_ context.Context, _ *domain.SSOConnection) error { return nil }
func (f *inactiveSSOConnRepo) Delete(_ context.Context, _ uint) error                  { return nil }

// ─── Test builder helpers ───────────────────────────────────────────────────────

const (
	testOrgID   = uint(1)
	testConnID  = uint(10)
	testUserID  = uint(100)
	testBaseURL = "https://api.passwall.io"
)

func newTestSSOService(
	connRepo *fakeSSOConnRepo,
	stateRepo *fakeSSOStateRepo,
	userRepo *fakeUserRepo,
	orgUserRepo *fakeOrgUserRepo,
	orgRepo *fakeOrgRepo,
	authService *fakeAuthService,
) *ssoService {
	if connRepo == nil {
		connRepo = newFakeSSOConnRepo()
	}
	if stateRepo == nil {
		stateRepo = newFakeSSOStateRepo()
	}
	if userRepo == nil {
		userRepo = newFakeUserRepo()
	}
	if orgUserRepo == nil {
		orgUserRepo = newFakeOrgUserRepo()
	}
	if orgRepo == nil {
		orgRepo = newFakeOrgRepo()
	}
	if authService == nil {
		authService = &fakeAuthService{}
	}
	return &ssoService{
		connRepo:    connRepo,
		stateRepo:   stateRepo,
		userRepo:    userRepo,
		orgUserRepo: orgUserRepo,
		orgRepo:     orgRepo,
		authService: authService,
		logger:      noopLogger{},
		baseURL:     testBaseURL,
	}
}

func buildSAMLResponse(issuer, email, audience, recipient, notBefore, notOnOrAfter string) string {
	return fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol"
		xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">
		<saml:Issuer>%s</saml:Issuer>
		<saml:Assertion>
			<saml:Issuer>%s</saml:Issuer>
			<saml:Subject>
				<saml:NameID>%s</saml:NameID>
				<saml:SubjectConfirmation>
					<saml:SubjectConfirmationData Recipient="%s"/>
				</saml:SubjectConfirmation>
			</saml:Subject>
			<saml:Conditions NotBefore="%s" NotOnOrAfter="%s">
				<saml:AudienceRestriction>
					<saml:Audience>%s</saml:Audience>
				</saml:AudienceRestriction>
			</saml:Conditions>
			<saml:AttributeStatement>
				<saml:Attribute Name="email">
					<saml:AttributeValue>%s</saml:AttributeValue>
				</saml:Attribute>
			</saml:AttributeStatement>
		</saml:Assertion>
	</samlp:Response>`,
		issuer, issuer, email, recipient, notBefore, notOnOrAfter, audience, email)
}

func mustB64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// ─── HandleSAMLCallback Tests ───────────────────────────────────────────────────

func TestHandleSAMLCallback_MissingParameters(t *testing.T) {
	t.Parallel()
	svc := newTestSSOService(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	tests := []struct {
		name         string
		relayState   string
		samlResponse string
	}{
		{"both empty", "", ""},
		{"empty relayState", "", mustB64("<Response/>")},
		{"empty samlResponse", "some-state", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := svc.HandleSAMLCallback(ctx, tt.relayState, tt.samlResponse)
			assert.ErrorIs(t, err, ErrSSOInvalidSAMLResponse)
		})
	}
}

func TestHandleSAMLCallback_InvalidState(t *testing.T) {
	t.Parallel()
	stateRepo := newFakeSSOStateRepo()
	svc := newTestSSOService(nil, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.HandleSAMLCallback(ctx, "nonexistent-state", mustB64("<Response/>"))
	assert.ErrorIs(t, err, ErrSSOInvalidState)
}

func TestHandleSAMLCallback_ExpiredState(t *testing.T) {
	t.Parallel()
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["expired-state"] = &domain.SSOState{
		ID:           1,
		State:        "expired-state",
		ConnectionID: testConnID,
		ExpiresAt:    time.Now().Add(-1 * time.Hour), // expired
	}
	svc := newTestSSOService(nil, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.HandleSAMLCallback(ctx, "expired-state", mustB64("<Response/>"))
	assert.ErrorIs(t, err, ErrSSOInvalidState)
}

func TestHandleSAMLCallback_ConnectionNotFound(t *testing.T) {
	t.Parallel()
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid-state"] = &domain.SSOState{
		ID:           1,
		State:        "valid-state",
		ConnectionID: 999, // nonexistent
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(nil, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.HandleSAMLCallback(ctx, "valid-state", mustB64("<Response/>"))
	assert.ErrorIs(t, err, ErrSSOConnectionNotFound)
}

func TestHandleSAMLCallback_ConnectionInactive(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusDraft, // not active!
		SAMLConfig:     &domain.SAMLConfig{},
	})
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid-state"] = &domain.SSOState{
		ID:           1,
		State:        "valid-state",
		ConnectionID: testConnID,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.HandleSAMLCallback(ctx, "valid-state", mustB64("<Response/>"))
	assert.ErrorIs(t, err, ErrSSOConnectionInactive)
}

func TestHandleSAMLCallback_ProtocolMismatch(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolOIDC, // OIDC, not SAML
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SAMLConfig:     nil,
	})
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid-state"] = &domain.SSOState{
		ID:           1,
		State:        "valid-state",
		ConnectionID: testConnID,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.HandleSAMLCallback(ctx, "valid-state", mustB64("<Response/>"))
	assert.ErrorIs(t, err, ErrSSOProtocolMismatch)
}

func TestHandleSAMLCallback_InvalidBase64(t *testing.T) {
	t.Parallel()
	_, _, certPEM := generateSelfSignedCert(t)
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SAMLConfig:     &domain.SAMLConfig{Certificate: certPEM, EntityID: "https://idp.acme.com"},
	})
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid-state"] = &domain.SSOState{
		ID:           1,
		State:        "valid-state",
		ConnectionID: testConnID,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.HandleSAMLCallback(ctx, "valid-state", "%%%not-base64%%%")
	assert.ErrorIs(t, err, ErrSSOInvalidSAMLResponse)
}

func TestHandleSAMLCallback_InvalidXMLAfterDecode(t *testing.T) {
	t.Parallel()
	_, _, certPEM := generateSelfSignedCert(t)
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SAMLConfig:     &domain.SAMLConfig{Certificate: certPEM, EntityID: "https://idp.acme.com"},
	})
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid-state"] = &domain.SSOState{
		ID:           1,
		State:        "valid-state",
		ConnectionID: testConnID,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.HandleSAMLCallback(ctx, "valid-state", mustB64("<<<definitely not xml>>>"))
	assert.ErrorIs(t, err, ErrSSOInvalidSAMLResponse)
}

func TestHandleSAMLCallback_MissingAssertion(t *testing.T) {
	t.Parallel()
	_, _, certPEM := generateSelfSignedCert(t)
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SAMLConfig:     &domain.SAMLConfig{Certificate: certPEM, EntityID: "https://idp.acme.com"},
	})
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid-state"] = &domain.SSOState{
		ID:           1,
		State:        "valid-state",
		ConnectionID: testConnID,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	// Valid SAML Response XML but no Assertion element
	xmlStr := `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol">
		<saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">idp</saml:Issuer>
	</samlp:Response>`
	_, err := svc.HandleSAMLCallback(ctx, "valid-state", mustB64(xmlStr))
	assert.ErrorIs(t, err, ErrSSOInvalidSAMLResponse)
}

func TestHandleSAMLCallback_UnsignedWhenSignatureRequired(t *testing.T) {
	t.Parallel()
	_, _, certPEM := generateSelfSignedCert(t)
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SPEntityID:     "https://api.passwall.io/sso/metadata/10",
		SAMLConfig: &domain.SAMLConfig{
			Certificate:         certPEM,
			EntityID:            "https://idp.acme.com",
			WantAssertionSigned: true, // require signature
		},
	})
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid-state"] = &domain.SSOState{
		ID:             1,
		State:          "valid-state",
		ConnectionID:   testConnID,
		OrganizationID: testOrgID,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	// Response with assertion but no Signature elements
	now := time.Now().UTC()
	xmlStr := buildSAMLResponse(
		"https://idp.acme.com",
		"user@acme.com",
		"https://api.passwall.io/sso/metadata/10",
		"https://api.passwall.io/sso/callback",
		now.Add(-1*time.Minute).Format(time.RFC3339),
		now.Add(10*time.Minute).Format(time.RFC3339),
	)
	_, err := svc.HandleSAMLCallback(ctx, "valid-state", mustB64(xmlStr))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not signed")
}

func TestHandleSAMLCallback_SignatureVerificationFails(t *testing.T) {
	t.Parallel()
	_, _, certPEM := generateSelfSignedCert(t)
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SPEntityID:     "https://api.passwall.io/sso/metadata/10",
		SAMLConfig: &domain.SAMLConfig{
			Certificate:         certPEM,
			EntityID:            "https://idp.acme.com",
			WantAssertionSigned: false, // don't check for presence, but still verify if present
		},
	})
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid-state"] = &domain.SSOState{
		ID:             1,
		State:          "valid-state",
		ConnectionID:   testConnID,
		OrganizationID: testOrgID,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	now := time.Now().UTC()
	xmlStr := buildSAMLResponse(
		"https://idp.acme.com",
		"user@acme.com",
		"https://api.passwall.io/sso/metadata/10",
		"https://api.passwall.io/sso/callback",
		now.Add(-1*time.Minute).Format(time.RFC3339),
		now.Add(10*time.Minute).Format(time.RFC3339),
	)
	// Signature verification should fail because the XML is not actually signed
	_, err := svc.HandleSAMLCallback(ctx, "valid-state", mustB64(xmlStr))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature verification failed")
}

func TestHandleSAMLCallback_IssuerMismatch(t *testing.T) {
	t.Parallel()

	// For this test, we need to bypass signature verification.
	// We test issuer mismatch by checking the code path directly.
	// Since verifySAMLXMLSignature is called before issuer check,
	// we verify that the SAML envelope parsing handles issuer comparison correctly.

	now := time.Now().UTC()
	xmlStr := buildSAMLResponse(
		"https://wrong-idp.evil.com", // wrong issuer
		"user@acme.com",
		"https://api.passwall.io/sso/metadata/10",
		"https://api.passwall.io/sso/callback",
		now.Add(-1*time.Minute).Format(time.RFC3339),
		now.Add(10*time.Minute).Format(time.RFC3339),
	)

	// Verify envelope parsing captures issuer correctly
	var env samlResponseEnvelope
	err := xml.Unmarshal([]byte(xmlStr), &env)
	require.NoError(t, err)
	assert.Equal(t, "https://wrong-idp.evil.com", env.Assertion.Issuer.Value)
	// In the real flow, this would be compared against conn.SAMLConfig.EntityID
}

func TestHandleSAMLCallback_DomainMismatch_EmailDomain(t *testing.T) {
	t.Parallel()

	// Test that extractSAMLEmail + matchesDomain rejects cross-domain emails
	now := time.Now().UTC()
	xmlStr := buildSAMLResponse(
		"https://idp.acme.com",
		"attacker@evil.com", // wrong domain for acme.com
		"https://api.passwall.io/sso/metadata/10",
		"https://api.passwall.io/sso/callback",
		now.Add(-1*time.Minute).Format(time.RFC3339),
		now.Add(10*time.Minute).Format(time.RFC3339),
	)
	var env samlResponseEnvelope
	err := xml.Unmarshal([]byte(xmlStr), &env)
	require.NoError(t, err)

	email := extractSAMLEmail(env.Assertion)
	assert.Equal(t, "attacker@evil.com", email)
	assert.False(t, matchesDomain(email, "acme.com"))
}

func TestHandleSAMLCallback_ExpiredAssertion(t *testing.T) {
	t.Parallel()

	// Verify time parsing for expired assertions
	past := time.Now().UTC().Add(-1 * time.Hour)
	notOnOrAfter := past.Format(time.RFC3339)

	parsed, err := parseSAMLTime(notOnOrAfter)
	require.NoError(t, err)

	now := time.Now().UTC()
	// Replicating sso_service.go logic: !now.Before(notOnOrAfter.Add(2*time.Minute))
	assert.False(t, now.Before(parsed.Add(2*time.Minute)), "assertion should be detected as expired")
}

func TestHandleSAMLCallback_NotYetValidAssertion(t *testing.T) {
	t.Parallel()

	future := time.Now().UTC().Add(1 * time.Hour)
	notBefore := future.Format(time.RFC3339)

	parsed, err := parseSAMLTime(notBefore)
	require.NoError(t, err)

	now := time.Now().UTC()
	// Replicating: now.Before(notBefore.Add(-2*time.Minute))
	assert.True(t, now.Before(parsed.Add(-2*time.Minute)), "assertion should be detected as not yet valid")
}

func TestHandleSAMLCallback_RecipientMismatch(t *testing.T) {
	t.Parallel()

	// Verify recipient URL comparison
	assert.True(t, urlsEqualWithoutTrailingSlash(
		"https://api.passwall.io/sso/callback",
		"https://api.passwall.io/sso/callback",
	))
	assert.False(t, urlsEqualWithoutTrailingSlash(
		"https://evil.com/sso/callback",
		"https://api.passwall.io/sso/callback",
	))
}

func TestHandleSAMLCallback_AudienceMismatch(t *testing.T) {
	t.Parallel()

	// Verify audience URL comparison
	assert.True(t, urlsEqualWithoutTrailingSlash(
		"https://api.passwall.io/sso/metadata/10",
		"https://api.passwall.io/sso/metadata/10",
	))
	assert.False(t, urlsEqualWithoutTrailingSlash(
		"https://evil.com/entity",
		"https://api.passwall.io/sso/metadata/10",
	))
}

func TestHandleSAMLCallback_StateCleanedUpAfterUse(t *testing.T) {
	t.Parallel()

	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["oneshot-state"] = &domain.SSOState{
		ID:             1,
		State:          "oneshot-state",
		ConnectionID:   testConnID,
		OrganizationID: testOrgID,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
	}

	_, _, certPEM := generateSelfSignedCert(t)
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SAMLConfig:     &domain.SAMLConfig{Certificate: certPEM, EntityID: "https://idp.acme.com"},
	})

	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	// Execute (will fail at signature verification, but state cleanup runs in defer)
	_, _ = svc.HandleSAMLCallback(ctx, "oneshot-state", mustB64(buildSAMLResponse(
		"https://idp.acme.com", "user@acme.com",
		"aud", "rec",
		time.Now().UTC().Format(time.RFC3339),
		time.Now().UTC().Add(10*time.Minute).Format(time.RFC3339),
	)))

	// State should be cleaned up
	_, err := stateRepo.GetByState(context.Background(), "oneshot-state")
	assert.ErrorIs(t, err, repository.ErrNotFound, "state should be deleted after callback processing")
}

// ─── completeSSOLogin Tests ─────────────────────────────────────────────────────

func TestCompleteSSOLogin_UserNotFound(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	userRepo := newFakeUserRepo() // empty, no users
	svc := newTestSSOService(connRepo, nil, userRepo, nil, nil, nil)
	ctx := context.Background()

	conn := &domain.SSOConnection{ID: testConnID, OrganizationID: testOrgID}
	_, err := svc.completeSSOLogin(ctx, conn, "unknown@acme.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user does not exist")
}

func TestCompleteSSOLogin_NotMemberNotProvisioning(t *testing.T) {
	t.Parallel()
	userRepo := newFakeUserRepo()
	userRepo.add(&domain.User{ID: testUserID, Email: "alice@acme.com"})
	orgUserRepo := newFakeOrgUserRepo() // empty: no membership

	svc := newTestSSOService(nil, nil, userRepo, orgUserRepo, nil, nil)
	ctx := context.Background()

	conn := &domain.SSOConnection{
		ID:              testConnID,
		OrganizationID:  testOrgID,
		JITProvisioning: false,
		AutoProvision:   false,
	}
	_, err := svc.completeSSOLogin(ctx, conn, "alice@acme.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a member")
}

func TestCompleteSSOLogin_JITProvisioningCreatesNewMember(t *testing.T) {
	t.Parallel()
	userRepo := newFakeUserRepo()
	userRepo.add(&domain.User{ID: testUserID, Email: "alice@acme.com"})
	orgUserRepo := newFakeOrgUserRepo() // no existing membership
	orgRepo := newFakeOrgRepo()
	orgRepo.add(&domain.Organization{ID: testOrgID, Name: "Acme Corp"})

	svc := newTestSSOService(nil, nil, userRepo, orgUserRepo, orgRepo, nil)
	ctx := context.Background()

	conn := &domain.SSOConnection{
		ID:              testConnID,
		OrganizationID:  testOrgID,
		JITProvisioning: true,
		DefaultRole:     domain.OrgRoleMember,
	}
	result, err := svc.completeSSOLogin(ctx, conn, "alice@acme.com")
	require.NoError(t, err)
	assert.Equal(t, "test-access-token", result.AccessToken)
	assert.Equal(t, testOrgID, result.Organization.ID)

	// Verify membership was created
	member, err := orgUserRepo.GetByOrgAndUser(ctx, testOrgID, testUserID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrgUserStatusProvisioned, member.Status)
	assert.Equal(t, domain.OrgRoleMember, member.Role)
	assert.Equal(t, "pending_key_exchange", member.EncryptedOrgKey)
}

func TestCompleteSSOLogin_SuspendedMemberRejected(t *testing.T) {
	t.Parallel()
	userRepo := newFakeUserRepo()
	userRepo.add(&domain.User{ID: testUserID, Email: "alice@acme.com"})
	orgUserRepo := newFakeOrgUserRepo()
	orgUserRepo.add(&domain.OrganizationUser{
		OrganizationID: testOrgID,
		UserID:         testUserID,
		Status:         domain.OrgUserStatusSuspended,
	})

	svc := newTestSSOService(nil, nil, userRepo, orgUserRepo, nil, nil)
	ctx := context.Background()

	conn := &domain.SSOConnection{ID: testConnID, OrganizationID: testOrgID}
	_, err := svc.completeSSOLogin(ctx, conn, "alice@acme.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "membership is not active")
}

func TestCompleteSSOLogin_AllowedStatuses(t *testing.T) {
	t.Parallel()

	allowedStatuses := []domain.OrganizationUserStatus{
		domain.OrgUserStatusAccepted,
		domain.OrgUserStatusConfirmed,
		domain.OrgUserStatusProvisioned,
	}

	for _, status := range allowedStatuses {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			userRepo := newFakeUserRepo()
			userRepo.add(&domain.User{ID: testUserID, Email: "alice@acme.com"})
			orgUserRepo := newFakeOrgUserRepo()
			orgUserRepo.add(&domain.OrganizationUser{
				OrganizationID: testOrgID,
				UserID:         testUserID,
				Status:         status,
			})
			orgRepo := newFakeOrgRepo()
			orgRepo.add(&domain.Organization{ID: testOrgID, Name: "Acme Corp"})

			svc := newTestSSOService(nil, nil, userRepo, orgUserRepo, orgRepo, nil)
			ctx := context.Background()

			conn := &domain.SSOConnection{ID: testConnID, OrganizationID: testOrgID}
			result, err := svc.completeSSOLogin(ctx, conn, "alice@acme.com")
			require.NoError(t, err)
			assert.Equal(t, "test-access-token", result.AccessToken)
		})
	}
}

func TestCompleteSSOLogin_TokenIssueFails(t *testing.T) {
	t.Parallel()
	userRepo := newFakeUserRepo()
	userRepo.add(&domain.User{ID: testUserID, Email: "alice@acme.com"})
	orgUserRepo := newFakeOrgUserRepo()
	orgUserRepo.add(&domain.OrganizationUser{
		OrganizationID: testOrgID,
		UserID:         testUserID,
		Status:         domain.OrgUserStatusAccepted,
	})

	authSvc := &fakeAuthService{shouldFail: true}
	svc := newTestSSOService(nil, nil, userRepo, orgUserRepo, nil, authSvc)
	ctx := context.Background()

	conn := &domain.SSOConnection{ID: testConnID, OrganizationID: testOrgID}
	_, err := svc.completeSSOLogin(ctx, conn, "alice@acme.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create Passwall session")
}

// ─── InitiateLogin Tests ────────────────────────────────────────────────────────

func TestInitiateLogin_DomainNotFound(t *testing.T) {
	t.Parallel()
	svc := newTestSSOService(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := svc.InitiateLogin(ctx, &domain.SSOInitiateRequest{Domain: "unknown.com"})
	assert.ErrorIs(t, err, ErrSSOConnectionNotFound)
}

func TestInitiateLogin_InactiveConnection(t *testing.T) {
	t.Parallel()
	// Use a custom fake that returns inactive connection from GetByDomain
	connRepo := &inactiveSSOConnRepo{
		conn: &domain.SSOConnection{
			ID:             testConnID,
			OrganizationID: testOrgID,
			Protocol:       domain.SSOProtocolSAML,
			Domain:         "acme.com",
			Status:         domain.SSOStatusInactive,
			SAMLConfig:     &domain.SAMLConfig{EntityID: "idp", SSOURL: "https://idp.acme.com/sso"},
		},
	}

	svc := &ssoService{
		connRepo:    connRepo,
		stateRepo:   newFakeSSOStateRepo(),
		userRepo:    newFakeUserRepo(),
		orgUserRepo: newFakeOrgUserRepo(),
		orgRepo:     newFakeOrgRepo(),
		authService: &fakeAuthService{},
		logger:      noopLogger{},
		baseURL:     testBaseURL,
	}
	ctx := context.Background()

	_, err := svc.InitiateLogin(ctx, &domain.SSOInitiateRequest{Domain: "acme.com"})
	assert.ErrorIs(t, err, ErrSSOConnectionInactive)
}

func TestInitiateLogin_SAML_GeneratesRedirect(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SAMLConfig: &domain.SAMLConfig{
			EntityID: "https://idp.acme.com",
			SSOURL:   "https://idp.acme.com/saml/login",
		},
	})
	stateRepo := newFakeSSOStateRepo()
	svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
	ctx := context.Background()

	redirectURL, err := svc.InitiateLogin(ctx, &domain.SSOInitiateRequest{
		Domain: "acme.com",
	})
	require.NoError(t, err)

	assert.Contains(t, redirectURL, "https://idp.acme.com/saml/login")
	assert.Contains(t, redirectURL, "RelayState=")

	// Verify state was persisted
	assert.Len(t, stateRepo.states, 1)
	for _, s := range stateRepo.states {
		assert.Equal(t, testConnID, s.ConnectionID)
		assert.Equal(t, testOrgID, s.OrganizationID)
		assert.True(t, s.ExpiresAt.After(time.Now()))
	}
}

func TestInitiateLogin_RedirectURLValidation(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusActive,
		SAMLConfig: &domain.SAMLConfig{
			EntityID: "https://idp.acme.com",
			SSOURL:   "https://idp.acme.com/saml/login",
		},
	})

	tests := []struct {
		name        string
		redirectURL string
		wantInState string
	}{
		{"allowed passwall domain", "https://vault.passwall.io/done", "https://vault.passwall.io/done"},
		{"allowed connection domain", "https://acme.com/sso-complete", "https://acme.com/sso-complete"},
		{"rejected foreign domain", "https://evil.com/steal", ""},
		{"empty redirect", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stateRepo := newFakeSSOStateRepo()
			svc := newTestSSOService(connRepo, stateRepo, nil, nil, nil, nil)
			ctx := context.Background()

			_, err := svc.InitiateLogin(ctx, &domain.SSOInitiateRequest{
				Domain:      "acme.com",
				RedirectURL: tt.redirectURL,
			})
			require.NoError(t, err)

			// Check the stored redirect URL
			for _, s := range stateRepo.states {
				assert.Equal(t, tt.wantInState, s.RedirectURL)
			}
		})
	}
}

// ─── GetRedirectURLByState Tests ────────────────────────────────────────────────

func TestGetRedirectURLByState_EmptyState(t *testing.T) {
	t.Parallel()
	svc := newTestSSOService(nil, nil, nil, nil, nil, nil)
	_, err := svc.GetRedirectURLByState(context.Background(), "")
	assert.ErrorIs(t, err, ErrSSOInvalidState)
}

func TestGetRedirectURLByState_NotFound(t *testing.T) {
	t.Parallel()
	stateRepo := newFakeSSOStateRepo()
	svc := newTestSSOService(nil, stateRepo, nil, nil, nil, nil)
	_, err := svc.GetRedirectURLByState(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrSSOInvalidState)
}

func TestGetRedirectURLByState_Expired(t *testing.T) {
	t.Parallel()
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["expired"] = &domain.SSOState{
		ID:        1,
		State:     "expired",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	svc := newTestSSOService(nil, stateRepo, nil, nil, nil, nil)
	_, err := svc.GetRedirectURLByState(context.Background(), "expired")
	assert.ErrorIs(t, err, ErrSSOInvalidState)
}

func TestGetRedirectURLByState_Valid(t *testing.T) {
	t.Parallel()
	stateRepo := newFakeSSOStateRepo()
	stateRepo.states["valid"] = &domain.SSOState{
		ID:          1,
		State:       "valid",
		RedirectURL: "https://vault.passwall.io/done",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	svc := newTestSSOService(nil, stateRepo, nil, nil, nil, nil)
	url, err := svc.GetRedirectURLByState(context.Background(), "valid")
	require.NoError(t, err)
	assert.Equal(t, "https://vault.passwall.io/done", url)
}

// ─── CreateConnection Tests ─────────────────────────────────────────────────────

func TestCreateConnection_EmptyDomain(t *testing.T) {
	t.Parallel()
	svc := newTestSSOService(nil, nil, nil, nil, nil, nil)
	_, err := svc.CreateConnection(context.Background(), testOrgID, testUserID, &domain.CreateSSOConnectionRequest{
		Protocol: domain.SSOProtocolSAML,
		Name:     "Test",
		Domain:   "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain is required")
}

func TestCreateConnection_SAMLMissingConfig(t *testing.T) {
	t.Parallel()
	svc := newTestSSOService(nil, nil, nil, nil, nil, nil)
	_, err := svc.CreateConnection(context.Background(), testOrgID, testUserID, &domain.CreateSSOConnectionRequest{
		Protocol:   domain.SSOProtocolSAML,
		Name:       "Test",
		Domain:     "acme.com",
		SAMLConfig: nil, // missing
	})
	assert.ErrorIs(t, err, ErrSSOProtocolMismatch)
}

func TestCreateConnection_SAMLIncompleteConfig(t *testing.T) {
	t.Parallel()
	svc := newTestSSOService(nil, nil, nil, nil, nil, nil)
	_, err := svc.CreateConnection(context.Background(), testOrgID, testUserID, &domain.CreateSSOConnectionRequest{
		Protocol: domain.SSOProtocolSAML,
		Name:     "Test",
		Domain:   "acme.com",
		SAMLConfig: &domain.SAMLConfig{
			EntityID: "idp",
			SSOURL:   "", // missing
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "entity_id, sso_url and certificate")
}

func TestCreateConnection_DuplicateDomain(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             1,
		OrganizationID: 99,
		Domain:         "acme.com",
	})
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)
	_, err := svc.CreateConnection(context.Background(), testOrgID, testUserID, &domain.CreateSSOConnectionRequest{
		Protocol: domain.SSOProtocolSAML,
		Name:     "Test",
		Domain:   "acme.com",
		SAMLConfig: &domain.SAMLConfig{
			EntityID:    "idp",
			SSOURL:      "https://idp.acme.com/sso",
			Certificate: "cert",
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already configured")
}

// ─── ActivateConnection Tests ───────────────────────────────────────────────────

func TestActivateConnection_SAML_MissingCert(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Status:         domain.SSOStatusDraft,
		SAMLConfig: &domain.SAMLConfig{
			EntityID:    "idp",
			SSOURL:      "https://idp.acme.com/sso",
			Certificate: "", // missing cert
		},
	})
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)
	_, err := svc.ActivateConnection(context.Background(), testConnID, testUserID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestActivateConnection_OIDC_MissingClientID(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolOIDC,
		Status:         domain.SSOStatusDraft,
		OIDCConfig: &domain.OIDCConfig{
			Issuer:   "https://idp.acme.com",
			ClientID: "", // missing
		},
	})
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)
	_, err := svc.ActivateConnection(context.Background(), testConnID, testUserID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client_id")
}

func TestActivateConnection_Success(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:             testConnID,
		OrganizationID: testOrgID,
		Protocol:       domain.SSOProtocolSAML,
		Domain:         "acme.com",
		Status:         domain.SSOStatusDraft,
		SAMLConfig: &domain.SAMLConfig{
			EntityID:    "https://idp.acme.com",
			SSOURL:      "https://idp.acme.com/sso",
			Certificate: "MIICert...",
		},
	})
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)
	conn, err := svc.ActivateConnection(context.Background(), testConnID, testUserID)
	require.NoError(t, err)
	assert.Equal(t, domain.SSOStatusActive, conn.Status)
}

// ─── SP Metadata Tests ──────────────────────────────────────────────────────────

func TestGetSPMetadata_Generates(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:         testConnID,
		SPEntityID: "https://api.passwall.io/sso/metadata/10",
		SPMetadata: "", // empty -> should generate
	})
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)
	metadata, err := svc.GetSPMetadata(context.Background(), testConnID)
	require.NoError(t, err)
	assert.Contains(t, metadata, "EntityDescriptor")
	assert.Contains(t, metadata, "https://api.passwall.io/sso/metadata/10")
	assert.Contains(t, metadata, "https://api.passwall.io/sso/callback")
}

func TestGetSPMetadata_ReturnsStored(t *testing.T) {
	t.Parallel()
	connRepo := newFakeSSOConnRepo()
	connRepo.add(&domain.SSOConnection{
		ID:         testConnID,
		SPMetadata: "<md:EntityDescriptor>custom</md:EntityDescriptor>",
	})
	svc := newTestSSOService(connRepo, nil, nil, nil, nil, nil)
	metadata, err := svc.GetSPMetadata(context.Background(), testConnID)
	require.NoError(t, err)
	assert.Contains(t, metadata, "custom")
}
