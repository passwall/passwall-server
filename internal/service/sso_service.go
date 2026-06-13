package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	saml2 "github.com/russellhaering/gosaml2"
	dsig "github.com/russellhaering/goxmldsig"
	"golang.org/x/oauth2"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

var (
	ErrSSOConnectionNotFound  = errors.New("sso connection not found")
	ErrSSOConnectionInactive  = errors.New("sso connection is not active")
	ErrSSOInvalidState        = errors.New("invalid or expired SSO state")
	ErrSSODomainMismatch      = errors.New("email domain does not match SSO connection")
	ErrSSOProtocolMismatch    = errors.New("protocol config missing for connection type")
	ErrSSOProvisioningBlocked = errors.New("automatic provisioning requires org key exchange and is blocked")
	ErrSSOInvalidSAMLResponse = errors.New("invalid SAML response")
)

// SSOService handles SSO connection management and authentication flows
type SSOService interface {
	// Admin operations
	CreateConnection(ctx context.Context, orgID, userID uint, req *domain.CreateSSOConnectionRequest) (*domain.SSOConnection, error)
	GetConnection(ctx context.Context, id uint) (*domain.SSOConnection, error)
	ListConnections(ctx context.Context, orgID uint) ([]*domain.SSOConnection, error)
	UpdateConnection(ctx context.Context, id, userID uint, req *domain.UpdateSSOConnectionRequest) (*domain.SSOConnection, error)
	DeleteConnection(ctx context.Context, id, userID uint) error
	ActivateConnection(ctx context.Context, id, userID uint) (*domain.SSOConnection, error)

	// Authentication flows
	InitiateLogin(ctx context.Context, req *domain.SSOInitiateRequest) (redirectURL string, err error)
	HandleOIDCCallback(ctx context.Context, stateParam, code string) (*domain.SSOCallbackResult, error)
	HandleSAMLCallback(ctx context.Context, relayState, samlResponse string) (*domain.SSOCallbackResult, error)
	GetRedirectURLByState(ctx context.Context, state string) (string, error)

	// SP metadata
	GetSPMetadata(ctx context.Context, connID uint) (string, error)
}

type ssoService struct {
	connRepo      repository.SSOConnectionRepository
	stateRepo     repository.SSOStateRepository
	userRepo      repository.UserRepository
	orgUserRepo   repository.OrganizationUserRepository
	orgRepo       repository.OrganizationRepository
	authService   AuthService
	escrowService KeyEscrowService
	logger        Logger
	baseURL       string
}

// NewSSOService creates a new SSO service
func NewSSOService(
	connRepo repository.SSOConnectionRepository,
	stateRepo repository.SSOStateRepository,
	userRepo repository.UserRepository,
	orgUserRepo repository.OrganizationUserRepository,
	orgRepo repository.OrganizationRepository,
	authService AuthService,
	escrowService KeyEscrowService,
	logger Logger,
	baseURL string,
) SSOService {
	return &ssoService{
		connRepo:      connRepo,
		stateRepo:     stateRepo,
		userRepo:      userRepo,
		orgUserRepo:   orgUserRepo,
		orgRepo:       orgRepo,
		authService:   authService,
		escrowService: escrowService,
		logger:        logger,
		baseURL:       baseURL,
	}
}

func (s *ssoService) CreateConnection(ctx context.Context, orgID, userID uint, req *domain.CreateSSOConnectionRequest) (*domain.SSOConnection, error) {
	normalizedDomain := strings.ToLower(strings.TrimSpace(req.Domain))
	if normalizedDomain == "" {
		s.logger.Error("SSO create connection rejected: empty domain", "org_id", orgID, "user_id", userID)
		return nil, fmt.Errorf("domain is required")
	}

	// Validate protocol-specific config
	if req.Protocol == domain.SSOProtocolSAML && req.SAMLConfig == nil {
		s.logger.Error("SSO create connection rejected: missing SAML config", "org_id", orgID, "user_id", userID)
		return nil, ErrSSOProtocolMismatch
	}
	if req.Protocol == domain.SSOProtocolOIDC && req.OIDCConfig == nil {
		s.logger.Error("SSO create connection rejected: missing OIDC config", "org_id", orgID, "user_id", userID)
		return nil, ErrSSOProtocolMismatch
	}
	if req.Protocol == domain.SSOProtocolSAML {
		if req.SAMLConfig.EntityID == "" || req.SAMLConfig.SSOURL == "" || req.SAMLConfig.Certificate == "" {
			s.logger.Error("SSO create connection rejected: incomplete SAML config", "org_id", orgID, "user_id", userID)
			return nil, fmt.Errorf("SAML connection requires entity_id, sso_url and certificate")
		}
	}
	if existing, err := s.connRepo.GetAnyByDomain(ctx, normalizedDomain); err == nil && existing != nil {
		s.logger.Warn("SSO create connection domain already configured", "org_id", orgID, "user_id", userID, "domain", normalizedDomain, "existing_org_id", existing.OrganizationID)
		return nil, fmt.Errorf("domain is already configured for another organization")
	}

	connUUID := uuid.New()

	conn := &domain.SSOConnection{
		UUID:           connUUID,
		OrganizationID: orgID,
		Protocol:       req.Protocol,
		Name:           req.Name,
		Domain:         normalizedDomain,
		SAMLConfig:     req.SAMLConfig,
		OIDCConfig:     req.OIDCConfig,
		DefaultRole:    domain.OrgRoleMember,
		Status:         domain.SSOStatusDraft,
	}

	if req.DefaultRole != "" {
		conn.DefaultRole = req.DefaultRole
	}
	if req.AutoProvision != nil {
		conn.AutoProvision = *req.AutoProvision
	} else {
		conn.AutoProvision = false
	}
	if req.JITProvisioning != nil {
		conn.JITProvisioning = *req.JITProvisioning
	} else {
		conn.JITProvisioning = false
	}

	if err := s.connRepo.Create(ctx, conn); err != nil {
		s.logger.Error("SSO create connection repository create failed", "org_id", orgID, "user_id", userID, "domain", normalizedDomain, "err", err)
		return nil, fmt.Errorf("failed to create SSO connection: %w", err)
	}
	// Generate SP metadata URLs (stable callback path for simpler IdP setup).
	conn.SPEntityID = fmt.Sprintf("%s/sso/metadata/%d", s.baseURL, conn.ID)
	conn.SPAcsURL = s.callbackURL()
	if err := s.connRepo.Update(ctx, conn); err != nil {
		s.logger.Error("SSO create connection metadata update failed", "org_id", orgID, "user_id", userID, "conn_id", conn.ID, "err", err)
		return nil, fmt.Errorf("failed to persist generated SP metadata URLs: %w", err)
	}

	s.logger.Info("SSO connection created", "org_id", orgID, "protocol", req.Protocol, "domain", req.Domain)
	return conn, nil
}

func (s *ssoService) GetConnection(ctx context.Context, id uint) (*domain.SSOConnection, error) {
	conn, err := s.connRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("SSO get connection not found", "conn_id", id)
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
	conn, err := s.connRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("SSO update connection not found", "conn_id", id, "user_id", userID)
			return nil, ErrSSOConnectionNotFound
		}
		s.logger.Error("SSO update connection fetch failed", "conn_id", id, "user_id", userID, "err", err)
		return nil, err
	}

	if req.Name != nil {
		conn.Name = *req.Name
	}
	if req.Domain != nil {
		d := strings.ToLower(strings.TrimSpace(*req.Domain))
		if d == "" {
			s.logger.Error("SSO update connection rejected: empty domain", "conn_id", id, "user_id", userID)
			return nil, fmt.Errorf("domain cannot be empty")
		}
		if existing, err := s.connRepo.GetAnyByDomain(ctx, d); err == nil && existing != nil && existing.ID != conn.ID {
			s.logger.Warn("SSO update connection domain conflict", "conn_id", id, "user_id", userID, "domain", d, "existing_conn_id", existing.ID)
			return nil, fmt.Errorf("domain is already configured for another organization")
		}
		conn.Domain = d
	}
	if req.SAMLConfig != nil {
		conn.SAMLConfig = req.SAMLConfig
	}
	if req.OIDCConfig != nil {
		conn.OIDCConfig = req.OIDCConfig
	}
	if req.AutoProvision != nil {
		conn.AutoProvision = *req.AutoProvision
	}
	if req.DefaultRole != nil {
		conn.DefaultRole = *req.DefaultRole
	}
	if req.JITProvisioning != nil {
		conn.JITProvisioning = *req.JITProvisioning
	}
	if req.KeyEscrowEnabled != nil {
		if *req.KeyEscrowEnabled {
			if s.escrowService == nil || !s.escrowService.IsConfigured() {
				s.logger.Error("SSO update connection rejected: key escrow requested but server escrow_master_key not configured", "conn_id", id, "org_id", conn.OrganizationID)
				return nil, fmt.Errorf("cannot enable key escrow: server escrow master key is not configured")
			}
			if err := s.escrowService.EnableForOrg(ctx, conn.OrganizationID); err != nil {
				s.logger.Error("SSO update connection escrow enable failed", "conn_id", id, "org_id", conn.OrganizationID, "err", err)
				return nil, fmt.Errorf("failed to enable key escrow: %w", err)
			}
		}
		conn.KeyEscrowEnabled = *req.KeyEscrowEnabled
	}
	if req.Status != nil {
		conn.Status = *req.Status
	}

	if err := s.connRepo.Update(ctx, conn); err != nil {
		s.logger.Error("SSO update connection repository update failed", "conn_id", id, "user_id", userID, "err", err)
		return nil, fmt.Errorf("failed to update SSO connection: %w", err)
	}

	s.logger.Info("SSO connection updated", "conn_id", id, "user_id", userID, "org_id", conn.OrganizationID)
	return conn, nil
}

func (s *ssoService) DeleteConnection(ctx context.Context, id, userID uint) error {
	if _, err := s.connRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("SSO delete connection not found", "conn_id", id, "user_id", userID)
			return ErrSSOConnectionNotFound
		}
		s.logger.Error("SSO delete connection fetch failed", "conn_id", id, "user_id", userID, "err", err)
		return err
	}
	s.logger.Info("SSO connection delete requested", "conn_id", id, "user_id", userID)
	return s.connRepo.Delete(ctx, id)
}

func (s *ssoService) ActivateConnection(ctx context.Context, id, userID uint) (*domain.SSOConnection, error) {
	conn, err := s.connRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("SSO activate connection not found", "conn_id", id, "user_id", userID)
			return nil, ErrSSOConnectionNotFound
		}
		s.logger.Error("SSO activate connection fetch failed", "conn_id", id, "user_id", userID, "err", err)
		return nil, err
	}

	// Validate that required config is present before activation
	if conn.Protocol == domain.SSOProtocolOIDC {
		if conn.OIDCConfig == nil || conn.OIDCConfig.ClientID == "" || conn.OIDCConfig.Issuer == "" {
			s.logger.Error("SSO activate connection rejected: incomplete OIDC config", "conn_id", id, "user_id", userID)
			return nil, fmt.Errorf("OIDC connection requires issuer and client_id before activation")
		}
	}
	if conn.Protocol == domain.SSOProtocolSAML {
		if conn.SAMLConfig == nil || conn.SAMLConfig.EntityID == "" || conn.SAMLConfig.SSOURL == "" || conn.SAMLConfig.Certificate == "" {
			s.logger.Error("SSO activate connection rejected: incomplete SAML config", "conn_id", id, "user_id", userID)
			return nil, fmt.Errorf("SAML connection requires entity_id, sso_url and certificate before activation")
		}
	}

	conn.Status = domain.SSOStatusActive
	if err := s.connRepo.Update(ctx, conn); err != nil {
		s.logger.Error("SSO activate connection update failed", "conn_id", id, "user_id", userID, "err", err)
		return nil, fmt.Errorf("failed to activate SSO connection: %w", err)
	}

	s.logger.Info("SSO connection activated", "conn_id", id)
	return conn, nil
}

// InitiateLogin starts the SSO authentication flow by generating the IdP redirect URL
func (s *ssoService) InitiateLogin(ctx context.Context, req *domain.SSOInitiateRequest) (string, error) {
	s.logger.Info("SSO initiate login started", "domain", strings.ToLower(req.Domain), "has_redirect_url", strings.TrimSpace(req.RedirectURL) != "")
	conn, err := s.connRepo.GetByDomain(ctx, strings.ToLower(req.Domain))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("SSO initiate login connection not found", "domain", strings.ToLower(req.Domain))
			return "", ErrSSOConnectionNotFound
		}
		s.logger.Error("SSO initiate login domain lookup failed", "domain", strings.ToLower(req.Domain), "err", err)
		return "", err
	}
	if !conn.IsActive() {
		s.logger.Warn("SSO initiate login connection inactive", "conn_id", conn.ID, "domain", conn.Domain)
		return "", ErrSSOConnectionInactive
	}

	stateToken, err := generateRandomState()
	if err != nil {
		s.logger.Error("SSO initiate login state generation failed", "conn_id", conn.ID, "err", err)
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	validatedRedirect := validateRedirectURL(req.RedirectURL, s.baseURL, conn.Domain)
	if req.RedirectURL != "" && validatedRedirect == "" {
		s.logger.Warn("SSO initiate login rejected redirect URL", "conn_id", conn.ID, "redirect_url", req.RedirectURL)
	}

	ssoState := &domain.SSOState{
		State:          stateToken,
		ConnectionID:   conn.ID,
		OrganizationID: conn.OrganizationID,
		RedirectURL:    validatedRedirect,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
	}

	switch conn.Protocol {
	case domain.SSOProtocolOIDC:
		s.logger.Info("SSO initiate login using OIDC", "conn_id", conn.ID, "org_id", conn.OrganizationID)
		return s.initiateOIDC(ctx, conn, ssoState)
	case domain.SSOProtocolSAML:
		s.logger.Info("SSO initiate login using SAML", "conn_id", conn.ID, "org_id", conn.OrganizationID)
		return s.initiateSAML(ctx, conn, ssoState)
	default:
		s.logger.Error("SSO initiate login unsupported protocol", "conn_id", conn.ID, "protocol", conn.Protocol)
		return "", fmt.Errorf("unsupported SSO protocol: %s", conn.Protocol)
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
	s.logger.Info("SSO OIDC callback started", "state", stateParam)
	ssoState, err := s.stateRepo.GetByState(ctx, stateParam)
	if err != nil {
		s.logger.Warn("SSO OIDC callback state not found", "state", stateParam)
		return nil, ErrSSOInvalidState
	}
	if ssoState.IsExpired() {
		_ = s.stateRepo.Delete(ctx, ssoState.ID)
		s.logger.Warn("SSO OIDC callback state expired", "state_id", ssoState.ID, "conn_id", ssoState.ConnectionID)
		return nil, ErrSSOInvalidState
	}

	// Clean up state (single use)
	defer func() { _ = s.stateRepo.Delete(ctx, ssoState.ID) }()

	conn, err := s.connRepo.GetByID(ctx, ssoState.ConnectionID)
	if err != nil {
		s.logger.Error("SSO OIDC callback connection lookup failed", "state_id", ssoState.ID, "conn_id", ssoState.ConnectionID, "err", err)
		return nil, fmt.Errorf("SSO connection not found for state: %w", err)
	}
	if !conn.IsActive() {
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
	result, err := s.completeSSOLogin(ctx, conn, email)
	if err != nil {
		return nil, err
	}
	result.RedirectURL = strings.TrimSpace(ssoState.RedirectURL)
	return result, nil
}

func (s *ssoService) HandleSAMLCallback(ctx context.Context, relayState, samlResponse string) (*domain.SSOCallbackResult, error) {
	if relayState == "" || samlResponse == "" {
		s.logger.Warn("SSO SAML callback missing parameters", "has_relay_state", relayState != "", "has_saml_response", samlResponse != "")
		return nil, ErrSSOInvalidSAMLResponse
	}
	ssoState, err := s.stateRepo.GetByState(ctx, relayState)
	if err != nil || ssoState == nil || ssoState.IsExpired() {
		s.logger.Warn("SSO SAML callback invalid state", "relay_state", relayState, "err", err)
		return nil, ErrSSOInvalidState
	}
	defer func() { _ = s.stateRepo.Delete(ctx, ssoState.ID) }()

	conn, err := s.connRepo.GetByID(ctx, ssoState.ConnectionID)
	if err != nil {
		s.logger.Error("SSO SAML callback connection lookup failed", "state_id", ssoState.ID, "conn_id", ssoState.ConnectionID, "err", err)
		return nil, ErrSSOConnectionNotFound
	}
	if !conn.IsActive() {
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
	result, err := s.completeSSOLogin(ctx, conn, email)
	if err != nil {
		return nil, err
	}
	result.RedirectURL = strings.TrimSpace(ssoState.RedirectURL)
	return result, nil
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

func (s *ssoService) completeSSOLogin(ctx context.Context, conn *domain.SSOConnection, email string) (*domain.SSOCallbackResult, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("SSO complete login user not found", "conn_id", conn.ID, "email", email)
			return nil, fmt.Errorf("user does not exist in Passwall; create account first")
		}
		s.logger.Error("SSO complete login user lookup failed", "conn_id", conn.ID, "email", email, "err", err)
		return nil, err
	}
	orgMembership, err := s.orgUserRepo.GetByOrgAndUser(ctx, conn.OrganizationID, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			if conn.JITProvisioning || conn.AutoProvision {
				orgMembership, err = s.jitProvisionMember(ctx, conn, user)
				if err != nil {
					s.logger.Error("SSO JIT provisioning failed", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID, "err", err)
					return nil, fmt.Errorf("SSO provisioning failed: %w", err)
				}
			} else {
				s.logger.Warn("SSO complete login user not member of organization", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID)
				return nil, fmt.Errorf("user is not a member of this organization")
			}
		} else {
			s.logger.Error("SSO complete login membership lookup failed", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID, "err", err)
			return nil, err
		}
	}

	allowedStatuses := map[domain.OrganizationUserStatus]bool{
		domain.OrgUserStatusAccepted:    true,
		domain.OrgUserStatusConfirmed:   true,
		domain.OrgUserStatusProvisioned: true,
	}
	if !allowedStatuses[orgMembership.Status] {
		s.logger.Warn("SSO complete login membership inactive", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID, "status", orgMembership.Status)
		return nil, fmt.Errorf("organization membership is not active")
	}
	authResp, err := s.authService.IssueTokenForUser(ctx, user.ID, "sso", "")
	if err != nil {
		s.logger.Error("SSO complete login token issue failed", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID, "err", err)
		return nil, fmt.Errorf("failed to create Passwall session from SSO login: %w", err)
	}
	org, err := s.orgRepo.GetByID(ctx, conn.OrganizationID)
	if err != nil {
		s.logger.Error("SSO complete login organization lookup failed", "conn_id", conn.ID, "org_id", conn.OrganizationID, "err", err)
		return nil, fmt.Errorf("failed to fetch organization: %w", err)
	}
	result := &domain.SSOCallbackResult{
		User:             user,
		AuthUser:         authResp.User,
		Organization:     org,
		IsNewUser:        false,
		AccessToken:      authResp.AccessToken,
		RefreshToken:     authResp.RefreshToken,
		ProtectedUserKey: authResp.ProtectedUserKey,
		KdfConfig:        authResp.KdfConfig,
	}

	// If key escrow is enabled and user is enrolled, include the raw User Key
	// so the client can unlock org vault items without master password.
	// Only the org key is returned — personal vault remains locked.
	if conn.KeyEscrowEnabled && s.escrowService != nil && s.escrowService.IsConfigured() {
		orgKey, err := s.escrowService.GetOrgKey(ctx, user.ID, conn.OrganizationID)
		if err != nil {
			s.logger.Warn("SSO key escrow retrieval failed (fallback to master password)", "conn_id", conn.ID, "user_id", user.ID, "err", err)
		} else {
			result.OrgKey = orgKey
			result.OrgID = conn.OrganizationID
			result.KeyEscrowUsed = true
			s.logger.Info("SSO org key escrow used for login", "conn_id", conn.ID, "user_id", user.ID, "org_id", conn.OrganizationID)
		}
	}

	s.logger.Info("SSO complete login success", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID, "key_escrow_used", result.KeyEscrowUsed)
	return result, nil
}

// jitProvisionMember creates an org membership for a user during their first SSO login.
// The member is created with "provisioned" status and a placeholder org key.
// An org admin must later confirm the member to complete the key exchange.
func (s *ssoService) jitProvisionMember(ctx context.Context, conn *domain.SSOConnection, user *domain.User) (*domain.OrganizationUser, error) {
	now := time.Now()
	orgUser := &domain.OrganizationUser{
		UUID:            uuid.New(),
		OrganizationID:  conn.OrganizationID,
		UserID:          user.ID,
		Role:            conn.DefaultRole,
		EncryptedOrgKey: "pending_key_exchange",
		AccessAll:       false,
		Status:          domain.OrgUserStatusProvisioned,
		InvitedAt:       &now,
	}

	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		return nil, fmt.Errorf("failed to create JIT membership: %w", err)
	}

	s.logger.Info("SSO JIT provisioned user into org", "conn_id", conn.ID, "org_id", conn.OrganizationID, "user_id", user.ID, "role", conn.DefaultRole)
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

// validateRedirectURL ensures the redirect URL is safe and belongs to allowed origins.
// It checks against the server's own base URL origin and the SSO connection's domain.
func validateRedirectURL(redirectURL, serverBaseURL, connectionDomain string) string {
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL == "" {
		return ""
	}

	parsed, err := url.Parse(redirectURL)
	if err != nil || !parsed.IsAbs() {
		return ""
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())

	serverParsed, err := url.Parse(serverBaseURL)
	if err == nil && strings.ToLower(serverParsed.Hostname()) == host {
		return redirectURL
	}

	// Allow origins that share the SSO connection's verified domain
	connDomain := strings.ToLower(strings.TrimSpace(connectionDomain))
	if connDomain != "" && (host == connDomain || strings.HasSuffix(host, "."+connDomain)) {
		return redirectURL
	}

	// Allow common Passwall domains
	allowedSuffixes := []string{".passwall.io", ".passwall.com"}
	for _, suffix := range allowedSuffixes {
		if strings.HasSuffix(host, suffix) || host == strings.TrimPrefix(suffix, ".") {
			return redirectURL
		}
	}

	// Allow localhost for development
	if host == "localhost" || host == "127.0.0.1" {
		return redirectURL
	}

	return ""
}
