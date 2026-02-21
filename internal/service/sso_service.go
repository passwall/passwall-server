package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	uuid "github.com/satori/go.uuid"
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
	InitiateLogin(ctx context.Context, req *domain.SSOInitiateRequest, baseURL string) (redirectURL string, err error)
	HandleOIDCCallback(ctx context.Context, stateParam, code string) (*domain.SSOCallbackResult, error)
	HandleSAMLCallback(ctx context.Context, relayState, samlResponse string) (*domain.SSOCallbackResult, error)

	// SP metadata
	GetSPMetadata(ctx context.Context, connID uint) (string, error)
}

type ssoService struct {
	connRepo    repository.SSOConnectionRepository
	stateRepo   repository.SSOStateRepository
	userRepo    repository.UserRepository
	orgUserRepo repository.OrganizationUserRepository
	orgRepo     repository.OrganizationRepository
	authService AuthService
	logger      Logger
	baseURL     string
}

// NewSSOService creates a new SSO service
func NewSSOService(
	connRepo repository.SSOConnectionRepository,
	stateRepo repository.SSOStateRepository,
	userRepo repository.UserRepository,
	orgUserRepo repository.OrganizationUserRepository,
	orgRepo repository.OrganizationRepository,
	authService AuthService,
	logger Logger,
	baseURL string,
) SSOService {
	return &ssoService{
		connRepo:    connRepo,
		stateRepo:   stateRepo,
		userRepo:    userRepo,
		orgUserRepo: orgUserRepo,
		orgRepo:     orgRepo,
		authService: authService,
		logger:      logger,
		baseURL:     baseURL,
	}
}

func (s *ssoService) CreateConnection(ctx context.Context, orgID, userID uint, req *domain.CreateSSOConnectionRequest) (*domain.SSOConnection, error) {
	normalizedDomain := strings.ToLower(strings.TrimSpace(req.Domain))
	if normalizedDomain == "" {
		return nil, fmt.Errorf("domain is required")
	}

	// Validate protocol-specific config
	if req.Protocol == domain.SSOProtocolSAML && req.SAMLConfig == nil {
		return nil, ErrSSOProtocolMismatch
	}
	if req.Protocol == domain.SSOProtocolOIDC && req.OIDCConfig == nil {
		return nil, ErrSSOProtocolMismatch
	}
	if req.Protocol == domain.SSOProtocolSAML {
		if req.SAMLConfig.EntityID == "" || req.SAMLConfig.SSOURL == "" || req.SAMLConfig.Certificate == "" {
			return nil, fmt.Errorf("SAML connection requires entity_id, sso_url and certificate")
		}
	}
	if existing, err := s.connRepo.GetAnyByDomain(ctx, normalizedDomain); err == nil && existing != nil {
		return nil, fmt.Errorf("domain is already configured for another organization")
	}

	connUUID := uuid.NewV4()

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
		return nil, fmt.Errorf("failed to create SSO connection: %w", err)
	}
	// Generate SP metadata URLs (stable callback path for simpler IdP setup).
	conn.SPEntityID = fmt.Sprintf("%s/sso/metadata/%d", s.baseURL, conn.ID)
	conn.SPAcsURL = s.callbackURL()
	if err := s.connRepo.Update(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to persist generated SP metadata URLs: %w", err)
	}

	s.logger.Info("SSO connection created", "org_id", orgID, "protocol", req.Protocol, "domain", req.Domain)
	return conn, nil
}

func (s *ssoService) GetConnection(ctx context.Context, id uint) (*domain.SSOConnection, error) {
	conn, err := s.connRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSSOConnectionNotFound
		}
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
			return nil, ErrSSOConnectionNotFound
		}
		return nil, err
	}

	if req.Name != nil {
		conn.Name = *req.Name
	}
	if req.Domain != nil {
		d := strings.ToLower(strings.TrimSpace(*req.Domain))
		if d == "" {
			return nil, fmt.Errorf("domain cannot be empty")
		}
		if existing, err := s.connRepo.GetAnyByDomain(ctx, d); err == nil && existing != nil && existing.ID != conn.ID {
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
	if req.Status != nil {
		conn.Status = *req.Status
	}

	if err := s.connRepo.Update(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to update SSO connection: %w", err)
	}

	return conn, nil
}

func (s *ssoService) DeleteConnection(ctx context.Context, id, userID uint) error {
	if _, err := s.connRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSSOConnectionNotFound
		}
		return err
	}
	return s.connRepo.Delete(ctx, id)
}

func (s *ssoService) ActivateConnection(ctx context.Context, id, userID uint) (*domain.SSOConnection, error) {
	conn, err := s.connRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSSOConnectionNotFound
		}
		return nil, err
	}

	// Validate that required config is present before activation
	if conn.Protocol == domain.SSOProtocolOIDC {
		if conn.OIDCConfig == nil || conn.OIDCConfig.ClientID == "" || conn.OIDCConfig.Issuer == "" {
			return nil, fmt.Errorf("OIDC connection requires issuer and client_id before activation")
		}
	}
	if conn.Protocol == domain.SSOProtocolSAML {
		if conn.SAMLConfig == nil || conn.SAMLConfig.EntityID == "" || conn.SAMLConfig.SSOURL == "" || conn.SAMLConfig.Certificate == "" {
			return nil, fmt.Errorf("SAML connection requires entity_id, sso_url and certificate before activation")
		}
	}

	conn.Status = domain.SSOStatusActive
	if err := s.connRepo.Update(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to activate SSO connection: %w", err)
	}

	s.logger.Info("SSO connection activated", "conn_id", id)
	return conn, nil
}

// InitiateLogin starts the SSO authentication flow by generating the IdP redirect URL
func (s *ssoService) InitiateLogin(ctx context.Context, req *domain.SSOInitiateRequest, baseURL string) (string, error) {
	conn, err := s.connRepo.GetByDomain(ctx, strings.ToLower(req.Domain))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrSSOConnectionNotFound
		}
		return "", err
	}
	if !conn.IsActive() {
		return "", ErrSSOConnectionInactive
	}

	stateToken, err := generateRandomState()
	if err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	ssoState := &domain.SSOState{
		State:          stateToken,
		ConnectionID:   conn.ID,
		OrganizationID: conn.OrganizationID,
		RedirectURL:    req.RedirectURL,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
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

func (s *ssoService) initiateOIDC(ctx context.Context, conn *domain.SSOConnection, ssoState *domain.SSOState) (string, error) {
	cfg := conn.OIDCConfig
	if cfg == nil {
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
			return "", fmt.Errorf("failed to generate PKCE: %w", err)
		}
		codeVerifier = v
		codeChallenge = ch
		ssoState.CodeVerifier = codeVerifier
	}
	nonce, err := generateRandomState()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	ssoState.Nonce = nonce

	// Persist state
	if err := s.stateRepo.Create(ctx, ssoState); err != nil {
		return "", fmt.Errorf("failed to persist SSO state: %w", err)
	}

	var endpoint oauth2.Endpoint
	if cfg.UseDiscovery || (cfg.AuthURL == "" || cfg.TokenURL == "") {
		provider, err := oidc.NewProvider(ctx, cfg.Issuer)
		if err != nil {
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
	return oauthCfg.AuthCodeURL(ssoState.State, opts...), nil
}

func (s *ssoService) initiateSAML(ctx context.Context, conn *domain.SSOConnection, ssoState *domain.SSOState) (string, error) {
	cfg := conn.SAMLConfig
	if cfg == nil {
		return "", ErrSSOProtocolMismatch
	}
	if cfg.SSOURL == "" {
		return "", fmt.Errorf("SAML SSO URL is not configured")
	}
	if err := s.stateRepo.Create(ctx, ssoState); err != nil {
		return "", fmt.Errorf("failed to persist SSO state: %w", err)
	}

	idpURL, err := url.Parse(cfg.SSOURL)
	if err != nil {
		return "", fmt.Errorf("invalid SAML SSO URL: %w", err)
	}
	q := idpURL.Query()
	q.Set("RelayState", ssoState.State)
	idpURL.RawQuery = q.Encode()
	return idpURL.String(), nil
}

// HandleOIDCCallback processes the IdP's authorization code callback
func (s *ssoService) HandleOIDCCallback(ctx context.Context, stateParam, code string) (*domain.SSOCallbackResult, error) {
	ssoState, err := s.stateRepo.GetByState(ctx, stateParam)
	if err != nil {
		return nil, ErrSSOInvalidState
	}
	if ssoState.IsExpired() {
		_ = s.stateRepo.Delete(ctx, ssoState.ID)
		return nil, ErrSSOInvalidState
	}

	// Clean up state (single use)
	defer func() { _ = s.stateRepo.Delete(ctx, ssoState.ID) }()

	conn, err := s.connRepo.GetByID(ctx, ssoState.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("SSO connection not found for state: %w", err)
	}
	if !conn.IsActive() {
		return nil, ErrSSOConnectionInactive
	}

	cfg := conn.OIDCConfig
	if cfg == nil {
		return nil, ErrSSOProtocolMismatch
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
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
		return nil, fmt.Errorf("OIDC code exchange failed: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("OIDC response did not include id_token")
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("OIDC id_token verification failed: %w", err)
	}
	claims := map[string]interface{}{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse id_token claims: %w", err)
	}
	if ssoState.Nonce != "" {
		nonce, _ := claims["nonce"].(string)
		if nonce != ssoState.Nonce {
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
		return nil, fmt.Errorf("email claim is missing in id_token")
	}
	if verified, exists := claims["email_verified"]; exists {
		if vb, ok := verified.(bool); ok && !vb {
			return nil, fmt.Errorf("email is not verified by identity provider")
		}
	}
	if !matchesDomain(email, conn.Domain) {
		return nil, ErrSSODomainMismatch
	}
	return s.completeSSOLogin(ctx, conn, email)
}

func (s *ssoService) HandleSAMLCallback(ctx context.Context, relayState, samlResponse string) (*domain.SSOCallbackResult, error) {
	if relayState == "" || samlResponse == "" {
		return nil, ErrSSOInvalidSAMLResponse
	}
	ssoState, err := s.stateRepo.GetByState(ctx, relayState)
	if err != nil || ssoState == nil || ssoState.IsExpired() {
		return nil, ErrSSOInvalidState
	}
	defer func() { _ = s.stateRepo.Delete(ctx, ssoState.ID) }()

	conn, err := s.connRepo.GetByID(ctx, ssoState.ConnectionID)
	if err != nil {
		return nil, ErrSSOConnectionNotFound
	}
	if !conn.IsActive() {
		return nil, ErrSSOConnectionInactive
	}
	if conn.Protocol != domain.SSOProtocolSAML || conn.SAMLConfig == nil {
		return nil, ErrSSOProtocolMismatch
	}

	decoded, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(samlResponse)
		if err != nil {
			return nil, ErrSSOInvalidSAMLResponse
		}
	}

	var resp samlResponseEnvelope
	if err := xml.Unmarshal(decoded, &resp); err != nil {
		return nil, ErrSSOInvalidSAMLResponse
	}
	if resp.Assertion == nil {
		return nil, ErrSSOInvalidSAMLResponse
	}
	if conn.SAMLConfig.WantAssertionSigned {
		if !resp.hasSignature() && !resp.Assertion.hasSignature() {
			return nil, fmt.Errorf("SAML response/assertion is not signed")
		}
	}
	if conn.SAMLConfig.EntityID != "" {
		issuer := strings.TrimSpace(resp.Assertion.Issuer.Value)
		if issuer == "" {
			issuer = strings.TrimSpace(resp.Issuer.Value)
		}
		if issuer != "" && issuer != strings.TrimSpace(conn.SAMLConfig.EntityID) {
			return nil, fmt.Errorf("SAML issuer mismatch")
		}
	}
	now := time.Now().UTC()
	if resp.Assertion.Conditions.NotBefore != "" {
		notBefore, err := parseSAMLTime(resp.Assertion.Conditions.NotBefore)
		if err == nil && now.Before(notBefore.Add(-2*time.Minute)) {
			return nil, fmt.Errorf("SAML assertion not yet valid")
		}
	}
	if resp.Assertion.Conditions.NotOnOrAfter != "" {
		notOnOrAfter, err := parseSAMLTime(resp.Assertion.Conditions.NotOnOrAfter)
		if err == nil && !now.Before(notOnOrAfter.Add(2*time.Minute)) {
			return nil, fmt.Errorf("SAML assertion expired")
		}
	}
	if recipient := strings.TrimSpace(resp.Assertion.Subject.SubjectConfirmation.SubjectConfirmationData.Recipient); recipient != "" {
		if !urlsEqualWithoutTrailingSlash(recipient, s.callbackURL()) {
			return nil, fmt.Errorf("SAML recipient mismatch")
		}
	}
	if audience := strings.TrimSpace(resp.Assertion.Conditions.AudienceRestriction.Audience); audience != "" {
		if !urlsEqualWithoutTrailingSlash(audience, conn.SPEntityID) {
			return nil, fmt.Errorf("SAML audience mismatch")
		}
	}

	email := extractSAMLEmail(resp.Assertion)
	if email == "" {
		return nil, fmt.Errorf("email is missing in SAML assertion")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !matchesDomain(email, conn.Domain) {
		return nil, ErrSSODomainMismatch
	}
	return s.completeSSOLogin(ctx, conn, email)
}

func (s *ssoService) completeSSOLogin(ctx context.Context, conn *domain.SSOConnection, email string) (*domain.SSOCallbackResult, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("user does not exist in Passwall; create account first")
		}
		return nil, err
	}
	orgMembership, err := s.orgUserRepo.GetByOrgAndUser(ctx, conn.OrganizationID, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			if conn.AutoProvision {
				return nil, ErrSSOProvisioningBlocked
			}
			return nil, fmt.Errorf("user is not a member of this organization")
		}
		return nil, err
	}
	if orgMembership.Status != domain.OrgUserStatusAccepted && orgMembership.Status != domain.OrgUserStatusConfirmed {
		return nil, fmt.Errorf("organization membership is not active")
	}
	authResp, err := s.authService.IssueTokenForUser(ctx, user.ID, "sso", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create Passwall session from SSO login: %w", err)
	}
	org, err := s.orgRepo.GetByID(ctx, conn.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch organization: %w", err)
	}
	return &domain.SSOCallbackResult{
		User:         user,
		Organization: org,
		IsNewUser:    false,
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
	}, nil
}

func (s *ssoService) GetSPMetadata(ctx context.Context, connID uint) (string, error) {
	conn, err := s.connRepo.GetByID(ctx, connID)
	if err != nil {
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

type samlResponseEnvelope struct {
	XMLName   xml.Name       `xml:"Response"`
	Issuer    samlTextNode   `xml:"Issuer"`
	Signature *xmlSignature  `xml:"Signature"`
	Assertion *samlAssertion `xml:"Assertion"`
}

func (r *samlResponseEnvelope) hasSignature() bool {
	return r != nil && r.Signature != nil
}

type samlAssertion struct {
	Issuer     samlTextNode   `xml:"Issuer"`
	Signature  *xmlSignature  `xml:"Signature"`
	Subject    samlSubject    `xml:"Subject"`
	Conditions samlConditions `xml:"Conditions"`
	Attributes samlAttrStmt   `xml:"AttributeStatement"`
}

func (a *samlAssertion) hasSignature() bool {
	return a != nil && a.Signature != nil
}

type xmlSignature struct {
	XMLName xml.Name `xml:"Signature"`
}

type samlTextNode struct {
	Value string `xml:",chardata"`
}

type samlSubject struct {
	NameID              samlTextNode            `xml:"NameID"`
	SubjectConfirmation samlSubjectConfirmation `xml:"SubjectConfirmation"`
}

type samlSubjectConfirmation struct {
	SubjectConfirmationData samlSubjectConfirmationData `xml:"SubjectConfirmationData"`
}

type samlSubjectConfirmationData struct {
	Recipient string `xml:"Recipient,attr"`
}

type samlConditions struct {
	NotBefore           string                  `xml:"NotBefore,attr"`
	NotOnOrAfter        string                  `xml:"NotOnOrAfter,attr"`
	AudienceRestriction samlAudienceRestriction `xml:"AudienceRestriction"`
}

type samlAudienceRestriction struct {
	Audience string `xml:"Audience"`
}

type samlAttrStmt struct {
	Attributes []samlAttribute `xml:"Attribute"`
}

type samlAttribute struct {
	Name   string             `xml:"Name,attr"`
	Values []samlAttributeVal `xml:"AttributeValue"`
}

type samlAttributeVal struct {
	Value string `xml:",chardata"`
}

func extractSAMLEmail(assertion *samlAssertion) string {
	if assertion == nil {
		return ""
	}
	for _, attr := range assertion.Attributes.Attributes {
		name := strings.ToLower(strings.TrimSpace(attr.Name))
		switch name {
		case "email", "mail", "emailaddress",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
			"urn:oid:0.9.2342.19200300.100.1.3":
			for _, v := range attr.Values {
				value := strings.TrimSpace(v.Value)
				if strings.Contains(value, "@") {
					return value
				}
			}
		}
	}
	nameID := strings.TrimSpace(assertion.Subject.NameID.Value)
	if strings.Contains(nameID, "@") {
		return nameID
	}
	return ""
}

func parseSAMLTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func urlsEqualWithoutTrailingSlash(a, b string) bool {
	a = strings.TrimRight(strings.TrimSpace(a), "/")
	b = strings.TrimRight(strings.TrimSpace(b), "/")
	return a != "" && b != "" && strings.EqualFold(a, b)
}

func (s *ssoService) callbackURL() string {
	return strings.TrimRight(s.baseURL, "/") + "/sso/callback"
}
