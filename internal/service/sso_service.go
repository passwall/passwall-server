package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	saml2 "github.com/russellhaering/gosaml2"
	dsig "github.com/russellhaering/goxmldsig"
	"golang.org/x/oauth2"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

var (
	ErrSSOConnectionNotFound    = errors.New("sso connection not found")
	ErrSSOConnectionInactive    = errors.New("sso connection is not active")
	ErrSSOInvalidState          = errors.New("invalid or expired SSO state")
	ErrSSODomainMismatch        = errors.New("email domain does not match SSO connection")
	ErrSSOProtocolMismatch      = errors.New("protocol config missing for connection type")
	ErrSSOInvalidSAMLResponse   = errors.New("invalid SAML response")
	ErrSSOInvalidDomain         = errors.New("invalid SSO domain")
	ErrSSODomainTaken           = errors.New("domain is already verified by another SSO connection")
	ErrSSODomainNotVerified     = errors.New("domain ownership is not verified")
	ErrSSODefaultRoleNotAllowed = errors.New("only the member role can be assigned automatically")
	ErrSSOInvalidCodeChallenge  = errors.New("invalid code challenge")
	ErrSSOInvalidLoginCode      = errors.New("invalid or expired SSO login code")
	ErrSSOUserNotFound          = errors.New("user does not have a Passwall account")
	ErrSSONotMember             = errors.New("user is not a member of this organization")
	ErrSSOMembershipInactive    = errors.New("organization membership is not active")
	ErrSSOConfigInvalid         = errors.New("invalid SSO configuration")
)

const (
	ssoStateTTL     = 10 * time.Minute
	ssoLoginCodeTTL = 2 * time.Minute
	ssoDNSTimeout   = 10 * time.Second
)

// TXTResolver looks up DNS TXT records (net.Resolver satisfies it).
type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// SSOService handles SSO connection management and authentication flows
type SSOService interface {
	// Admin operations
	CreateConnection(ctx context.Context, orgID, userID uint, req *domain.CreateSSOConnectionRequest) (*domain.SSOConnection, error)
	GetConnection(ctx context.Context, id uint) (*domain.SSOConnection, error)
	ListConnections(ctx context.Context, orgID uint) ([]*domain.SSOConnection, error)
	UpdateConnection(ctx context.Context, id, userID uint, req *domain.UpdateSSOConnectionRequest) (*domain.SSOConnection, error)
	DeleteConnection(ctx context.Context, id, userID uint) error
	ActivateConnection(ctx context.Context, id, userID uint) (*domain.SSOConnection, error)
	VerifyDomain(ctx context.Context, id, userID uint) (*domain.SSOConnection, error)

	// Authentication flows
	InitiateLogin(ctx context.Context, req *domain.SSOInitiateRequest) (redirectURL string, err error)
	HandleOIDCCallback(ctx context.Context, stateParam, code string) (*domain.SSOCallbackResult, error)
	HandleSAMLCallback(ctx context.Context, relayState, samlResponse string) (*domain.SSOCallbackResult, error)
	GetRedirectURLByState(ctx context.Context, state string) (string, error)
	ExchangeLoginCode(ctx context.Context, req *domain.SSOExchangeRequest) (*domain.SSOExchangeResponse, error)

	// SP metadata
	GetSPMetadata(ctx context.Context, connID uint) (string, error)
}

// SSOServiceDeps holds the SSO service dependencies.
type SSOServiceDeps struct {
	ConnRepo      repository.SSOConnectionRepository
	StateRepo     repository.SSOStateRepository
	LoginCodeRepo repository.SSOLoginCodeRepository
	UserRepo      repository.UserRepository
	OrgUserRepo   repository.OrganizationUserRepository
	OrgRepo       repository.OrganizationRepository
	AuthService   AuthService
	Logger        Logger
	// BaseURL is the public API origin (callback and SP metadata URLs).
	BaseURL string
	// RedirectOrigins are the client origins a login may return to
	// (frontend URL plus allowed origins).
	RedirectOrigins []string
	// AllowLocalhostRedirect permits http://localhost redirects (dev only).
	AllowLocalhostRedirect bool
	Entitlements           OrganizationEntitlementService
	// JoinPolicies checks single-organization policies before JIT provisioning.
	JoinPolicies interface {
		CheckJoinPolicies(ctx context.Context, orgID, userID uint) error
	}
	Resolver TXTResolver
}

type ssoService struct {
	connRepo        repository.SSOConnectionRepository
	stateRepo       repository.SSOStateRepository
	codeRepo        repository.SSOLoginCodeRepository
	userRepo        repository.UserRepository
	orgUserRepo     repository.OrganizationUserRepository
	orgRepo         repository.OrganizationRepository
	authService     AuthService
	logger          Logger
	baseURL         string
	redirectOrigins []string
	allowLocalhost  bool
	entitlements    OrganizationEntitlementService
	joinPolicies    interface {
		CheckJoinPolicies(ctx context.Context, orgID, userID uint) error
	}
	resolver TXTResolver
	now      func() time.Time
}

// NewSSOService creates a new SSO service
func NewSSOService(deps SSOServiceDeps) SSOService {
	resolver := deps.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &ssoService{
		connRepo:        deps.ConnRepo,
		stateRepo:       deps.StateRepo,
		codeRepo:        deps.LoginCodeRepo,
		userRepo:        deps.UserRepo,
		orgUserRepo:     deps.OrgUserRepo,
		orgRepo:         deps.OrgRepo,
		authService:     deps.AuthService,
		logger:          deps.Logger,
		baseURL:         deps.BaseURL,
		redirectOrigins: deps.RedirectOrigins,
		allowLocalhost:  deps.AllowLocalhostRedirect,
		entitlements:    deps.Entitlements,
		joinPolicies:    deps.JoinPolicies,
		resolver:        resolver,
	}
}

func (s *ssoService) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *ssoService) CreateConnection(ctx context.Context, orgID, userID uint, req *domain.CreateSSOConnectionRequest) (*domain.SSOConnection, error) {
	if err := s.authorizeSSOManagement(ctx, orgID); err != nil {
		return nil, err
	}
	normalizedDomain, err := normalizeSSODomain(req.Domain)
	if err != nil {
		return nil, err
	}

	if req.Protocol == domain.SSOProtocolSAML && req.SAMLConfig == nil {
		return nil, ErrSSOProtocolMismatch
	}
	if req.Protocol == domain.SSOProtocolOIDC && req.OIDCConfig == nil {
		return nil, ErrSSOProtocolMismatch
	}
	if req.Protocol == domain.SSOProtocolSAML {
		if req.SAMLConfig.EntityID == "" || req.SAMLConfig.SSOURL == "" || req.SAMLConfig.Certificate == "" {
			return nil, fmt.Errorf("%w: SAML connection requires entity_id, sso_url and certificate", ErrSSOConfigInvalid)
		}
		if _, err := parseIdPCertificate(req.SAMLConfig.Certificate); err != nil {
			return nil, fmt.Errorf("%w: IdP certificate: %v", ErrSSOConfigInvalid, err)
		}
	}
	if req.DefaultRole != "" && !domain.SSODefaultRoleAllowed(req.DefaultRole) {
		return nil, ErrSSODefaultRoleNotAllowed
	}
	if err := s.ensureDomainAvailable(ctx, normalizedDomain, 0); err != nil {
		return nil, err
	}

	token, err := generateDomainVerificationToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate verification token: %w", err)
	}

	conn := &domain.SSOConnection{
		UUID:                    uuid.New(),
		OrganizationID:          orgID,
		Protocol:                req.Protocol,
		Name:                    req.Name,
		Domain:                  normalizedDomain,
		DomainVerificationToken: token,
		SAMLConfig:              req.SAMLConfig,
		OIDCConfig:              req.OIDCConfig,
		DefaultRole:             domain.OrgRoleMember,
		Status:                  domain.SSOStatusDraft,
	}
	if req.AutoProvision != nil {
		conn.AutoProvision = *req.AutoProvision
	}
	if req.JITProvisioning != nil {
		conn.JITProvisioning = *req.JITProvisioning
	}

	if err := s.connRepo.Create(ctx, conn); err != nil {
		s.logger.Error("SSO create connection repository create failed", "org_id", orgID, "user_id", userID, "err", err)
		return nil, fmt.Errorf("failed to create SSO connection: %w", err)
	}
	// Generate SP metadata URLs (stable callback path for simpler IdP setup).
	conn.SPEntityID = fmt.Sprintf("%s/sso/metadata/%d", s.baseURL, conn.ID)
	conn.SPAcsURL = s.callbackURL()
	if err := s.connRepo.Update(ctx, conn); err != nil {
		s.logger.Error("SSO create connection metadata update failed", "org_id", orgID, "user_id", userID, "conn_id", conn.ID, "err", err)
		return nil, fmt.Errorf("failed to persist generated SP metadata URLs: %w", err)
	}

	s.logger.Info("SSO connection created", "org_id", orgID, "conn_id", conn.ID, "protocol", req.Protocol, "domain", normalizedDomain)
	return conn, nil
}

func (s *ssoService) GetConnection(ctx context.Context, id uint) (*domain.SSOConnection, error) {
	conn, err := s.connRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSSOConnectionNotFound
		}
		s.logger.Error("SSO get connection failed", "conn_id", id, "err", err)
		return nil, err
	}
	return conn, nil
}

func (s *ssoService) ListConnections(ctx context.Context, orgID uint) ([]*domain.SSOConnection, error) {
	return s.connRepo.ListByOrganization(ctx, orgID)
}

func (s *ssoService) UpdateConnection(ctx context.Context, id, userID uint, req *domain.UpdateSSOConnectionRequest) (*domain.SSOConnection, error) {
	conn, err := s.GetConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSSOManagement(ctx, conn.OrganizationID); err != nil {
		return nil, err
	}

	if req.Name != nil {
		conn.Name = *req.Name
	}
	if req.Domain != nil {
		d, err := normalizeSSODomain(*req.Domain)
		if err != nil {
			return nil, err
		}
		if d != conn.Domain {
			if err := s.ensureDomainAvailable(ctx, d, conn.ID); err != nil {
				return nil, err
			}
			token, err := generateDomainVerificationToken()
			if err != nil {
				return nil, fmt.Errorf("failed to generate verification token: %w", err)
			}
			// A new domain needs a new proof; sign-ins pause until then.
			conn.Domain = d
			conn.DomainVerificationToken = token
			conn.DomainVerifiedAt = nil
			if conn.Status == domain.SSOStatusActive {
				conn.Status = domain.SSOStatusDraft
			}
		}
	}
	if req.SAMLConfig != nil {
		next := *req.SAMLConfig
		// The certificate is never returned to clients; keep it when omitted.
		if strings.TrimSpace(next.Certificate) == "" && conn.SAMLConfig != nil {
			next.Certificate = conn.SAMLConfig.Certificate
		}
		if next.Certificate != "" {
			if _, err := parseIdPCertificate(next.Certificate); err != nil {
				return nil, fmt.Errorf("%w: IdP certificate: %v", ErrSSOConfigInvalid, err)
			}
		}
		conn.SAMLConfig = &next
	}
	if req.OIDCConfig != nil {
		next := *req.OIDCConfig
		// The client secret is never returned to clients; keep it when omitted.
		if next.ClientSecret == "" && conn.OIDCConfig != nil {
			next.ClientSecret = conn.OIDCConfig.ClientSecret
		}
		conn.OIDCConfig = &next
	}
	if req.AutoProvision != nil {
		conn.AutoProvision = *req.AutoProvision
	}
	if req.DefaultRole != nil {
		if !domain.SSODefaultRoleAllowed(*req.DefaultRole) {
			return nil, ErrSSODefaultRoleNotAllowed
		}
		conn.DefaultRole = domain.OrgRoleMember
	}
	if req.JITProvisioning != nil {
		conn.JITProvisioning = *req.JITProvisioning
	}
	if req.Status != nil {
		if *req.Status == domain.SSOStatusActive {
			return nil, fmt.Errorf("%w: use the activate endpoint", ErrSSOConfigInvalid)
		}
		conn.Status = *req.Status
	}

	if err := s.connRepo.Update(ctx, conn); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSSODomainTaken
		}
		s.logger.Error("SSO update connection repository update failed", "conn_id", id, "user_id", userID, "err", err)
		return nil, fmt.Errorf("failed to update SSO connection: %w", err)
	}

	s.logger.Info("SSO connection updated", "conn_id", id, "user_id", userID, "org_id", conn.OrganizationID)
	return conn, nil
}

func (s *ssoService) DeleteConnection(ctx context.Context, id, userID uint) error {
	conn, err := s.GetConnection(ctx, id)
	if err != nil {
		return err
	}
	if err := s.authorizeSSOManagement(ctx, conn.OrganizationID); err != nil {
		return err
	}
	s.logger.Info("SSO connection delete requested", "conn_id", id, "user_id", userID)
	return s.connRepo.Delete(ctx, id)
}

func (s *ssoService) ActivateConnection(ctx context.Context, id, userID uint) (*domain.SSOConnection, error) {
	conn, err := s.GetConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSSOManagement(ctx, conn.OrganizationID); err != nil {
		return nil, err
	}

	if conn.Protocol == domain.SSOProtocolOIDC {
		if conn.OIDCConfig == nil || conn.OIDCConfig.ClientID == "" || conn.OIDCConfig.Issuer == "" {
			return nil, fmt.Errorf("%w: OIDC connection requires issuer and client_id before activation", ErrSSOConfigInvalid)
		}
	}
	if conn.Protocol == domain.SSOProtocolSAML {
		if conn.SAMLConfig == nil || conn.SAMLConfig.EntityID == "" || conn.SAMLConfig.SSOURL == "" || conn.SAMLConfig.Certificate == "" {
			return nil, fmt.Errorf("%w: SAML connection requires entity_id, sso_url and certificate before activation", ErrSSOConfigInvalid)
		}
	}
	if !conn.IsDomainVerified() {
		return nil, ErrSSODomainNotVerified
	}

	conn.Status = domain.SSOStatusActive
	if err := s.connRepo.Update(ctx, conn); err != nil {
		s.logger.Error("SSO activate connection update failed", "conn_id", id, "user_id", userID, "err", err)
		return nil, fmt.Errorf("failed to activate SSO connection: %w", err)
	}

	s.logger.Info("SSO connection activated", "conn_id", id)
	return conn, nil
}

// VerifyDomain checks the DNS TXT record that proves the organization owns
// the connection's domain.
func (s *ssoService) VerifyDomain(ctx context.Context, id, userID uint) (*domain.SSOConnection, error) {
	conn, err := s.GetConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSSOManagement(ctx, conn.OrganizationID); err != nil {
		return nil, err
	}
	if conn.IsDomainVerified() {
		return conn, nil
	}
	if conn.DomainVerificationToken == "" {
		token, err := generateDomainVerificationToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate verification token: %w", err)
		}
		conn.DomainVerificationToken = token
		if err := s.connRepo.Update(ctx, conn); err != nil {
			return nil, fmt.Errorf("failed to store verification token: %w", err)
		}
		return nil, ErrSSODomainNotVerified
	}

	lookupCtx, cancel := context.WithTimeout(ctx, ssoDNSTimeout)
	defer cancel()
	records, err := s.resolver.LookupTXT(lookupCtx, conn.DomainVerificationRecordName())
	if err != nil {
		s.logger.Warn("SSO domain verification lookup failed", "conn_id", id, "domain", conn.Domain, "err", err)
		return nil, ErrSSODomainNotVerified
	}
	expected := conn.DomainVerificationRecordValue()
	found := false
	for _, record := range records {
		if strings.TrimSpace(strings.Trim(record, `"`)) == expected {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrSSODomainNotVerified
	}
	if err := s.ensureDomainAvailable(ctx, conn.Domain, conn.ID); err != nil {
		return nil, err
	}

	now := s.clock()
	conn.DomainVerifiedAt = &now
	if err := s.connRepo.Update(ctx, conn); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSSODomainTaken
		}
		return nil, fmt.Errorf("failed to store domain verification: %w", err)
	}
	s.logger.Info("SSO domain verified", "conn_id", id, "org_id", conn.OrganizationID, "domain", conn.Domain, "user_id", userID)
	return conn, nil
}

// ensureDomainAvailable fails when another connection already verified domain.
func (s *ssoService) ensureDomainAvailable(ctx context.Context, d string, selfID uint) error {
	existing, err := s.connRepo.GetVerifiedByDomain(ctx, d)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("failed to check domain: %w", err)
	}
	if existing != nil && existing.ID != selfID {
		return ErrSSODomainTaken
	}
	return nil
}

func (s *ssoService) authorizeSSOManagement(ctx context.Context, orgID uint) error {
	if s.entitlements == nil {
		return nil
	}
	return s.entitlements.Authorize(ctx, orgID, domain.CapabilitySSOManage)
}

// InitiateLogin starts the SSO authentication flow by generating the IdP redirect URL
func (s *ssoService) InitiateLogin(ctx context.Context, req *domain.SSOInitiateRequest) (string, error) {
	s.purgeExpired(ctx)

	lookup := strings.ToLower(strings.TrimSpace(req.Domain))
	if at := strings.LastIndex(lookup, "@"); at >= 0 {
		lookup = lookup[at+1:]
	}
	if !isValidCodeChallenge(req.CodeChallenge) {
		return "", ErrSSOInvalidCodeChallenge
	}
	conn, err := s.connRepo.GetByDomain(ctx, lookup)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrSSOConnectionNotFound
		}
		s.logger.Error("SSO initiate login domain lookup failed", "domain", lookup, "err", err)
		return "", err
	}
	if !conn.IsActive() || !conn.IsDomainVerified() {
		return "", ErrSSOConnectionInactive
	}

	stateToken, err := generateRandomState()
	if err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	validatedRedirect := s.validateRedirectURL(req.RedirectURL)
	if req.RedirectURL != "" && validatedRedirect == "" {
		s.logger.Warn("SSO initiate login rejected redirect URL", "conn_id", conn.ID)
	}

	ssoState := &domain.SSOState{
		State:               stateToken,
		ConnectionID:        conn.ID,
		OrganizationID:      conn.OrganizationID,
		RedirectURL:         validatedRedirect,
		ClientCodeChallenge: req.CodeChallenge,
		ExpiresAt:           s.clock().Add(ssoStateTTL),
	}

	switch conn.Protocol {
	case domain.SSOProtocolOIDC:
		return s.initiateOIDC(ctx, conn, ssoState)
	case domain.SSOProtocolSAML:
		return s.initiateSAML(ctx, conn, ssoState)
	default:
		return "", fmt.Errorf("unsupported SSO protocol: %s", conn.Protocol)
	}
}

// purgeExpired removes stale states and login codes. Best effort.
func (s *ssoService) purgeExpired(ctx context.Context) {
	if s.stateRepo != nil {
		_, _ = s.stateRepo.DeleteExpired(ctx)
	}
	if s.codeRepo != nil {
		_, _ = s.codeRepo.DeleteExpired(ctx)
	}
}

func (s *ssoService) initiateOIDC(ctx context.Context, conn *domain.SSOConnection, ssoState *domain.SSOState) (string, error) {
	cfg := conn.OIDCConfig
	if cfg == nil {
		s.logger.Error("SSO OIDC initiate missing protocol config", "conn_id", conn.ID)
		return "", ErrSSOProtocolMismatch
	}
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}

	// Generate PKCE code verifier/challenge if enabled
	var codeVerifier, codeChallenge string
	if cfg.PKCEEnabled {
		v, ch, err := generatePKCE()
		if err != nil {
			s.logger.Error("SSO OIDC initiate PKCE generation failed", "conn_id", conn.ID, "err", err)
			return "", fmt.Errorf("failed to generate PKCE: %w", err)
		}
		codeVerifier = v
		codeChallenge = ch
		ssoState.CodeVerifier = codeVerifier
	}
	nonce, err := generateRandomState()
	if err != nil {
		s.logger.Error("SSO OIDC initiate nonce generation failed", "conn_id", conn.ID, "err", err)
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	ssoState.Nonce = nonce

	// Persist state
	if err := s.stateRepo.Create(ctx, ssoState); err != nil {
		s.logger.Error("SSO OIDC initiate persist state failed", "conn_id", conn.ID, "err", err)
		return "", fmt.Errorf("failed to persist SSO state: %w", err)
	}

	var endpoint oauth2.Endpoint
	if cfg.UseDiscovery || (cfg.AuthURL == "" || cfg.TokenURL == "") {
		provider, err := oidc.NewProvider(ctx, cfg.Issuer)
		if err != nil {
			s.logger.Error("SSO OIDC discovery failed", "conn_id", conn.ID, "issuer", cfg.Issuer, "err", err)
			return "", fmt.Errorf("OIDC discovery failed: %w", err)
		}
		endpoint = provider.Endpoint()
	} else {
		endpoint = oauth2.Endpoint{AuthURL: cfg.AuthURL, TokenURL: cfg.TokenURL}
	}
	oauthCfg := oauth2.Config{
		ClientID:    cfg.ClientID,
		RedirectURL: s.callbackURL(),
		Endpoint:    endpoint,
		Scopes:      scopes,
	}
	opts := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("nonce", ssoState.Nonce)}
	if codeChallenge != "" {
		opts = append(opts, oauth2.SetAuthURLParam("code_challenge", codeChallenge))
		opts = append(opts, oauth2.SetAuthURLParam("code_challenge_method", "S256"))
	}
	s.logger.Info("SSO OIDC authorize URL generated", "conn_id", conn.ID, "use_discovery", cfg.UseDiscovery, "pkce_enabled", cfg.PKCEEnabled, "redirect_url", s.callbackURL(), "client_id", cfg.ClientID)
	return oauthCfg.AuthCodeURL(ssoState.State, opts...), nil
}

func (s *ssoService) initiateSAML(ctx context.Context, conn *domain.SSOConnection, ssoState *domain.SSOState) (string, error) {
	cfg := conn.SAMLConfig
	if cfg == nil {
		s.logger.Error("SSO SAML initiate missing protocol config", "conn_id", conn.ID)
		return "", ErrSSOProtocolMismatch
	}
	if cfg.SSOURL == "" {
		s.logger.Error("SSO SAML initiate missing SSO URL", "conn_id", conn.ID)
		return "", fmt.Errorf("SAML SSO URL is not configured")
	}

	sp, err := s.buildSAMLServiceProvider(conn)
	if err != nil {
		s.logger.Error("SSO SAML initiate build service provider failed", "conn_id", conn.ID, "err", err)
		return "", err
	}

	if err := s.stateRepo.Create(ctx, ssoState); err != nil {
		s.logger.Error("SSO SAML initiate persist state failed", "conn_id", conn.ID, "err", err)
		return "", fmt.Errorf("failed to persist SSO state: %w", err)
	}

	// Build a standards-compliant AuthnRequest (HTTP-Redirect binding):
	// deflated + base64 + URL-encoded SAMLRequest with RelayState. Required by
	// most IdPs (Okta, Entra/Azure AD, Google, OneLogin) for SP-initiated SSO.
	authURL, err := sp.BuildAuthURL(ssoState.State)
	if err != nil {
		s.logger.Error("SSO SAML initiate build auth URL failed", "conn_id", conn.ID, "err", err)
		return "", fmt.Errorf("failed to build SAML AuthnRequest: %w", err)
	}
	s.logger.Info("SSO SAML redirect URL generated", "conn_id", conn.ID)
	return authURL, nil
}

// buildSAMLServiceProvider constructs a gosaml2 SP bound to a single SSO
// connection's IdP configuration. Signature validation is always enabled
// (SkipSignatureValidation defaults to false), and gosaml2 enforces issuer,
// recipient, audience, status and time conditions while protecting against
// XML Signature Wrapping (XSW) attacks.
func (s *ssoService) buildSAMLServiceProvider(conn *domain.SSOConnection) (*saml2.SAMLServiceProvider, error) {
	cfg := conn.SAMLConfig
	if cfg == nil {
		return nil, ErrSSOProtocolMismatch
	}
	cert, err := parseIdPCertificate(cfg.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IdP certificate: %w", err)
	}
	certStore := &dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{cert},
	}
	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:      cfg.SSOURL,
		IdentityProviderIssuer:      cfg.EntityID,
		AssertionConsumerServiceURL: s.callbackURL(),
		ServiceProviderIssuer:       conn.SPEntityID,
		AudienceURI:                 conn.SPEntityID,
		IDPCertificateStore:         certStore,
		// SP request signing is not honored yet: no SP key material is
		// provisioned. Most IdPs accept unsigned AuthnRequests.
		SignAuthnRequests: false,
		NameIdFormat:      cfg.NameIDFormat,
		Clock:             dsig.NewRealClock(),
	}
	return sp, nil
}

// HandleOIDCCallback processes the IdP's authorization code callback
func (s *ssoService) HandleOIDCCallback(ctx context.Context, stateParam, code string) (*domain.SSOCallbackResult, error) {
	ssoState, err := s.stateRepo.Consume(ctx, stateParam)
	if err != nil {
		return nil, ErrSSOInvalidState
	}

	conn, err := s.connRepo.GetByID(ctx, ssoState.ConnectionID)
	if err != nil {
		s.logger.Error("SSO OIDC callback connection lookup failed", "state_id", ssoState.ID, "conn_id", ssoState.ConnectionID, "err", err)
		return nil, fmt.Errorf("SSO connection not found for state: %w", err)
	}
	if !conn.IsActive() || !conn.IsDomainVerified() {
		s.logger.Warn("SSO OIDC callback connection inactive", "conn_id", conn.ID)
		return nil, ErrSSOConnectionInactive
	}

	cfg := conn.OIDCConfig
	if cfg == nil {
		s.logger.Error("SSO OIDC callback missing protocol config", "conn_id", conn.ID)
		return nil, ErrSSOProtocolMismatch
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		s.logger.Error("SSO OIDC callback provider discovery failed", "conn_id", conn.ID, "issuer", cfg.Issuer, "err", err)
		return nil, fmt.Errorf("OIDC provider discovery failed: %w", err)
	}
	endpoint := provider.Endpoint()
	if cfg.AuthURL != "" && cfg.TokenURL != "" {
		endpoint = oauth2.Endpoint{AuthURL: cfg.AuthURL, TokenURL: cfg.TokenURL}
	}
	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  s.callbackURL(),
		Endpoint:     endpoint,
		Scopes:       defaultScopes(cfg.Scopes),
	}
	exchangeOpts := []oauth2.AuthCodeOption{}
	if ssoState.CodeVerifier != "" {
		exchangeOpts = append(exchangeOpts, oauth2.SetAuthURLParam("code_verifier", ssoState.CodeVerifier))
	}
	token, err := oauthCfg.Exchange(ctx, code, exchangeOpts...)
	if err != nil {
		s.logger.Error("SSO OIDC callback code exchange failed", "conn_id", conn.ID, "err", err)
		return nil, fmt.Errorf("OIDC code exchange failed: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		s.logger.Error("SSO OIDC callback missing id_token", "conn_id", conn.ID)
		return nil, fmt.Errorf("OIDC response did not include id_token")
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		s.logger.Error("SSO OIDC callback id_token verify failed", "conn_id", conn.ID, "err", err)
		return nil, fmt.Errorf("OIDC id_token verification failed: %w", err)
	}
	claims := map[string]interface{}{}
	if err := idToken.Claims(&claims); err != nil {
		s.logger.Error("SSO OIDC callback claims parse failed", "conn_id", conn.ID, "err", err)
		return nil, fmt.Errorf("failed to parse id_token claims: %w", err)
	}
	if ssoState.Nonce != "" {
		nonce, _ := claims["nonce"].(string)
		if nonce != ssoState.Nonce {
			s.logger.Error("SSO OIDC callback nonce mismatch", "conn_id", conn.ID)
			return nil, fmt.Errorf("invalid nonce in id_token")
		}
	}
	emailClaim := cfg.EmailClaim
	if emailClaim == "" {
		emailClaim = "email"
	}
	email, _ := claims[emailClaim].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		s.logger.Error("SSO OIDC callback missing email claim", "conn_id", conn.ID, "email_claim", emailClaim)
		return nil, fmt.Errorf("email claim is missing in id_token")
	}
	// Reject only when the IdP explicitly reports the email as unverified.
	// Handles both the boolean form and the string form ("false"/"0"/"no")
	// some IdPs emit. A missing claim is accepted for broad IdP compatibility
	// (e.g. Azure AD does not always include email_verified).
	if verified, exists := claims["email_verified"]; exists && isEmailVerifiedFalse(verified) {
		s.logger.Warn("SSO OIDC callback email not verified", "conn_id", conn.ID, "email", email)
		return nil, fmt.Errorf("email is not verified by identity provider")
	}
	if !matchesDomain(email, conn.Domain) {
		s.logger.Warn("SSO OIDC callback domain mismatch", "conn_id", conn.ID, "email", email, "expected_domain", conn.Domain)
		return nil, ErrSSODomainMismatch
	}
	s.logger.Info("SSO OIDC callback validated", "conn_id", conn.ID, "email", email)
	return s.completeSSOLogin(ctx, conn, ssoState, email)
}

func (s *ssoService) HandleSAMLCallback(ctx context.Context, relayState, samlResponse string) (*domain.SSOCallbackResult, error) {
	if relayState == "" || samlResponse == "" {
		s.logger.Warn("SSO SAML callback missing parameters", "has_relay_state", relayState != "", "has_saml_response", samlResponse != "")
		return nil, ErrSSOInvalidSAMLResponse
	}
	ssoState, err := s.stateRepo.Consume(ctx, relayState)
	if err != nil {
		return nil, ErrSSOInvalidState
	}

	conn, err := s.connRepo.GetByID(ctx, ssoState.ConnectionID)
	if err != nil {
		s.logger.Error("SSO SAML callback connection lookup failed", "state_id", ssoState.ID, "conn_id", ssoState.ConnectionID, "err", err)
		return nil, ErrSSOConnectionNotFound
	}
	if !conn.IsActive() || !conn.IsDomainVerified() {
		s.logger.Warn("SSO SAML callback connection inactive", "conn_id", conn.ID)
		return nil, ErrSSOConnectionInactive
	}
	if conn.Protocol != domain.SSOProtocolSAML || conn.SAMLConfig == nil {
		s.logger.Error("SSO SAML callback protocol mismatch", "conn_id", conn.ID, "protocol", conn.Protocol)
		return nil, ErrSSOProtocolMismatch
	}

	sp, err := s.buildSAMLServiceProvider(conn)
	if err != nil {
		s.logger.Error("SSO SAML callback build service provider failed", "conn_id", conn.ID, "err", err)
		return nil, err
	}

	// gosaml2 performs XSW-safe processing: it validates the XML digital
	// signature, then extracts claims ONLY from the signature-validated
	// element (also running an xml-roundtrip-validator). Issuer, recipient,
	// status and SubjectConfirmation NotOnOrAfter are enforced internally.
	assertionInfo, err := sp.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		s.logger.Error("SSO SAML callback assertion validation failed", "conn_id", conn.ID, "err", err)
		return nil, fmt.Errorf("%w: %v", ErrSSOInvalidSAMLResponse, err)
	}
	if wi := assertionInfo.WarningInfo; wi != nil {
		if wi.InvalidTime {
			s.logger.Error("SSO SAML callback assertion outside valid time window", "conn_id", conn.ID)
			return nil, fmt.Errorf("%w: assertion outside valid time window", ErrSSOInvalidSAMLResponse)
		}
		if wi.NotInAudience {
			s.logger.Error("SSO SAML callback assertion audience mismatch", "conn_id", conn.ID, "expected", conn.SPEntityID)
			return nil, fmt.Errorf("%w: assertion audience mismatch", ErrSSOInvalidSAMLResponse)
		}
	}

	email := extractSAMLEmailFromAssertion(assertionInfo)
	if email == "" {
		s.logger.Error("SSO SAML callback missing email in assertion", "conn_id", conn.ID)
		return nil, fmt.Errorf("email is missing in SAML assertion")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !matchesDomain(email, conn.Domain) {
		s.logger.Warn("SSO SAML callback domain mismatch", "conn_id", conn.ID, "email", email, "expected_domain", conn.Domain)
		return nil, ErrSSODomainMismatch
	}
	s.logger.Info("SSO SAML callback validated", "conn_id", conn.ID, "email", email)
	return s.completeSSOLogin(ctx, conn, ssoState, email)
}

func (s *ssoService) GetRedirectURLByState(ctx context.Context, state string) (string, error) {
	if strings.TrimSpace(state) == "" {
		return "", ErrSSOInvalidState
	}
	ssoState, err := s.stateRepo.GetByState(ctx, state)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrSSOInvalidState
		}
		return "", err
	}
	if ssoState.IsExpired() {
		return "", ErrSSOInvalidState
	}
	return strings.TrimSpace(ssoState.RedirectURL), nil
}

func (s *ssoService) completeSSOLogin(ctx context.Context, conn *domain.SSOConnection, ssoState *domain.SSOState, email string) (*domain.SSOCallbackResult, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSSOUserNotFound
		}
		return nil, err
	}
	if !user.IsVerified {
		return nil, ErrSSOUserNotFound
	}

	provisioned := false
	membership, err := s.orgUserRepo.GetByOrgAndUser(ctx, conn.OrganizationID, user.ID)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		if !conn.ProvisionsMembers() {
			return nil, ErrSSONotMember
		}
		membership, err = s.jitProvisionMember(ctx, conn, user)
		if err != nil {
			s.logger.Error("SSO JIT provisioning failed", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID, "err", err)
			return nil, err
		}
		provisioned = true
	case err != nil:
		return nil, err
	}

	switch membership.Status {
	case domain.OrgUserStatusAccepted, domain.OrgUserStatusConfirmed, domain.OrgUserStatusProvisioned:
	default:
		return nil, ErrSSOMembershipInactive
	}

	code, err := generateRandomState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate login code: %w", err)
	}
	if err := s.codeRepo.Create(ctx, &domain.SSOLoginCode{
		CodeHash:            hashToken(code),
		UserID:              user.ID,
		ConnectionID:        conn.ID,
		OrganizationID:      conn.OrganizationID,
		ClientCodeChallenge: ssoState.ClientCodeChallenge,
		ExpiresAt:           s.clock().Add(ssoLoginCodeTTL),
	}); err != nil {
		return nil, fmt.Errorf("failed to store login code: %w", err)
	}

	s.logger.Info("SSO callback complete", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID, "provisioned", provisioned)
	return &domain.SSOCallbackResult{
		Code:        code,
		RedirectURL: strings.TrimSpace(ssoState.RedirectURL),
		UserID:      user.ID,
		OrgID:       conn.OrganizationID,
		Provisioned: provisioned,
	}, nil
}

// ExchangeLoginCode trades a single-use login code for a Passwall session.
// The verifier binds the code to the browser that started the flow.
func (s *ssoService) ExchangeLoginCode(ctx context.Context, req *domain.SSOExchangeRequest) (*domain.SSOExchangeResponse, error) {
	loginCode, err := s.codeRepo.Consume(ctx, hashToken(strings.TrimSpace(req.Code)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSSOInvalidLoginCode
		}
		return nil, err
	}
	sum := sha256.Sum256([]byte(req.CodeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(challenge), []byte(loginCode.ClientCodeChallenge)) != 1 {
		return nil, ErrSSOInvalidLoginCode
	}

	conn, err := s.connRepo.GetByID(ctx, loginCode.ConnectionID)
	if err != nil || !conn.IsActive() || !conn.IsDomainVerified() {
		return nil, ErrSSOConnectionInactive
	}

	app := strings.TrimSpace(req.App)
	if app == "" {
		app = "sso"
	}
	authResp, err := s.authService.IssueSSOSession(ctx, loginCode.UserID, app, req.DeviceID)
	if err != nil {
		return nil, err
	}
	org, err := s.orgRepo.GetByID(ctx, loginCode.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch organization: %w", err)
	}
	return &domain.SSOExchangeResponse{
		AuthResponse: authResp,
		Organization: &domain.SSOOrganizationDTO{ID: org.ID, PublicID: org.PublicID, Name: org.Name},
	}, nil
}

// jitProvisionMember creates a "provisioned" membership on first SSO sign-in.
// The member has no org key yet; an admin confirms them to share it.
func (s *ssoService) jitProvisionMember(ctx context.Context, conn *domain.SSOConnection, user *domain.User) (*domain.OrganizationUser, error) {
	if s.joinPolicies != nil {
		if err := s.joinPolicies.CheckJoinPolicies(ctx, conn.OrganizationID, user.ID); err != nil {
			return nil, err
		}
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, conn.OrganizationID, domain.CapabilityMemberInvite); err != nil {
			return nil, err
		}
	}
	now := s.clock()
	orgUser := &domain.OrganizationUser{
		UUID:            uuid.New(),
		OrganizationID:  conn.OrganizationID,
		UserID:          user.ID,
		Role:            domain.OrgRoleMember,
		EncryptedOrgKey: "pending_key_exchange",
		AccessAll:       false,
		Status:          domain.OrgUserStatusProvisioned,
		InvitedAt:       &now,
	}
	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		return nil, fmt.Errorf("failed to create JIT membership: %w", err)
	}
	orgUser.User = user
	return orgUser, nil
}

func (s *ssoService) GetSPMetadata(ctx context.Context, connID uint) (string, error) {
	conn, err := s.connRepo.GetByID(ctx, connID)
	if err != nil {
		s.logger.Error("SSO get SP metadata failed", "conn_id", connID, "err", err)
		return "", err
	}

	if conn.SPMetadata != "" {
		return conn.SPMetadata, nil
	}

	// Generate minimal SAML SP metadata
	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"
  entityID="%s">
  <md:SPSSODescriptor AuthnRequestsSigned="false"
    WantAssertionsSigned="true"
    protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:AssertionConsumerService
      Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
      Location="%s"
      index="0"
      isDefault="true"/>
  </md:SPSSODescriptor>
</md:EntityDescriptor>`, conn.SPEntityID, s.callbackURL())

	return metadata, nil
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

func defaultScopes(scopes []string) []string {
	if len(scopes) > 0 {
		return scopes
	}
	return []string{"openid", "email", "profile"}
}

func matchesDomain(email, domain string) bool {
	parts := strings.SplitN(email, "@", 2)
	return len(parts) == 2 && parts[1] == strings.ToLower(domain)
}

// samlEmailAttributeNames are the common SAML attribute names IdPs use to
// convey the user's email address.
var samlEmailAttributeNames = []string{
	"email", "mail", "emailaddress", "User.email",
	"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
	"urn:oid:0.9.2342.19200300.100.1.3",
}

// extractSAMLEmailFromAssertion pulls the user's email from a
// signature-validated gosaml2 AssertionInfo. It prefers well-known attribute
// names, then falls back to any attribute value containing an "@", and finally
// to the NameID. Operating on AssertionInfo (not raw XML) keeps extraction tied
// to the cryptographically verified assertion.
func extractSAMLEmailFromAssertion(info *saml2.AssertionInfo) string {
	if info == nil {
		return ""
	}
	for _, name := range samlEmailAttributeNames {
		if v := strings.TrimSpace(info.Values.Get(name)); strings.Contains(v, "@") {
			return v
		}
	}
	for key := range info.Values {
		for _, v := range info.Values.GetAll(key) {
			if val := strings.TrimSpace(v); strings.Contains(val, "@") {
				return val
			}
		}
	}
	if nameID := strings.TrimSpace(info.NameID); strings.Contains(nameID, "@") {
		return nameID
	}
	return ""
}

func (s *ssoService) callbackURL() string {
	return strings.TrimRight(s.baseURL, "/") + "/sso/callback"
}

// isEmailVerifiedFalse reports whether an OIDC email_verified claim explicitly
// indicates the email is NOT verified, supporting both boolean and string forms.
func isEmailVerifiedFalse(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return !val
	case string:
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "false", "0", "no":
			return true
		}
	}
	return false
}

// parseIdPCertificate parses an x509 certificate from PEM or raw base64.
func parseIdPCertificate(raw string) (*x509.Certificate, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty certificate")
	}

	block, _ := pem.Decode([]byte(raw))
	if block != nil {
		return x509.ParseCertificate(block.Bytes)
	}

	// Try raw base64 (certificate without PEM headers)
	cleaned := strings.ReplaceAll(raw, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	derBytes, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("certificate is neither valid PEM nor base64: %w", err)
	}
	return x509.ParseCertificate(derBytes)
}

// validateRedirectURL keeps a redirect only when its origin is one of the
// configured client origins (or localhost in development). The login code is
// short-lived and PKCE-bound, but it still must not travel to other hosts.
func (s *ssoService) validateRedirectURL(redirectURL string) string {
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		return ""
	}
	parsed, err := url.Parse(redirectURL)
	if err != nil || !parsed.IsAbs() || parsed.User != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if s.allowLocalhost && (host == "localhost" || host == "127.0.0.1") &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") {
		return redirectURL
	}
	if parsed.Scheme != "https" {
		return ""
	}
	origin := parsed.Scheme + "://" + strings.ToLower(parsed.Host)
	for _, allowed := range s.redirectOrigins {
		a, err := url.Parse(strings.TrimSpace(allowed))
		if err != nil || a.Scheme == "" || a.Host == "" {
			continue
		}
		if origin == a.Scheme+"://"+strings.ToLower(a.Host) {
			return redirectURL
		}
	}
	return ""
}

// normalizeSSODomain lowercases and validates a bare DNS domain.
func normalizeSSODomain(raw string) (string, error) {
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if len(d) < 3 || len(d) > 253 || !strings.Contains(d, ".") {
		return "", ErrSSOInvalidDomain
	}
	for _, label := range strings.Split(d, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", ErrSSOInvalidDomain
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return "", ErrSSOInvalidDomain
			}
		}
	}
	return d, nil
}

func generateDomainVerificationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isValidCodeChallenge accepts a base64url S256 challenge.
func isValidCodeChallenge(challenge string) bool {
	if len(challenge) < 43 || len(challenge) > 128 {
		return false
	}
	for _, r := range challenge {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
