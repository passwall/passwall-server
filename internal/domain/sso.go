package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SSOProtocol represents the SSO protocol type
type SSOProtocol string

const (
	SSOProtocolSAML SSOProtocol = "saml"
	SSOProtocolOIDC SSOProtocol = "oidc"
)

// SSOConnectionStatus represents the status of an SSO connection
type SSOConnectionStatus string

const (
	SSOStatusDraft    SSOConnectionStatus = "draft"
	SSOStatusActive   SSOConnectionStatus = "active"
	SSOStatusInactive SSOConnectionStatus = "inactive"
)

// SAMLConfig holds SAML-specific IdP configuration
type SAMLConfig struct {
	EntityID            string `json:"entity_id"`
	SSOURL              string `json:"sso_url"`
	SLOURL              string `json:"slo_url,omitempty"`
	Certificate         string `json:"certificate"`
	SignAuthnRequests   bool   `json:"sign_authn_requests"`
	WantAssertionSigned bool   `json:"want_assertion_signed"`
	NameIDFormat        string `json:"name_id_format,omitempty"`
}

// Scan implements sql.Scanner
func (c *SAMLConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan SAMLConfig: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, c)
}

// Value implements driver.Valuer
func (c SAMLConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// OIDCConfig holds OIDC-specific IdP configuration
type OIDCConfig struct {
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AuthURL      string   `json:"auth_url,omitempty"`
	TokenURL     string   `json:"token_url,omitempty"`
	UserInfoURL  string   `json:"user_info_url,omitempty"`
	JwksURI      string   `json:"jwks_uri,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	UseDiscovery bool     `json:"use_discovery"`
	PKCEEnabled  bool     `json:"pkce_enabled"`
	EmailClaim   string   `json:"email_claim,omitempty"`
	NameClaim    string   `json:"name_claim,omitempty"`
	GroupsClaim  string   `json:"groups_claim,omitempty"`
}

// Scan implements sql.Scanner
func (c *OIDCConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan OIDCConfig: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, c)
}

// Value implements driver.Valuer
func (c OIDCConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// SSOConnection represents an SSO provider configuration for an organization
type SSOConnection struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID uint        `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	Protocol       SSOProtocol `json:"protocol" gorm:"type:varchar(10);not null"`

	// Display
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	// Domain is unique only among verified connections (partial index in
	// migration 00009), so an unverified claim cannot block the real owner.
	Domain string `json:"domain" gorm:"type:varchar(255);not null;index"`

	// DNS TXT domain ownership proof. Login and provisioning require a
	// verified domain.
	DomainVerificationToken string     `json:"-" gorm:"type:varchar(64)"`
	DomainVerifiedAt        *time.Time `json:"domain_verified_at,omitempty"`

	// Protocol-specific configuration (stored as JSONB)
	SAMLConfig *SAMLConfig `json:"saml_config,omitempty" gorm:"type:jsonb"`
	OIDCConfig *OIDCConfig `json:"oidc_config,omitempty" gorm:"type:jsonb"`

	// SP (Passwall) metadata — generated at creation, read-only for admin
	SPEntityID string `json:"sp_entity_id" gorm:"type:varchar(512)"`
	SPAcsURL   string `json:"sp_acs_url" gorm:"type:varchar(512)"`
	SPMetadata string `json:"-" gorm:"type:text"`

	// Behaviour
	// No GORM defaults on the booleans: with default:true GORM omits a false
	// value on insert and the database default silently turns it back on.
	AutoProvision   bool                `json:"auto_provision" gorm:"not null"`
	DefaultRole     OrganizationRole    `json:"default_role" gorm:"type:varchar(20);default:'member'"`
	JITProvisioning bool                `json:"jit_provisioning" gorm:"not null"`
	Status          SSOConnectionStatus `json:"status" gorm:"type:varchar(20);not null;default:'draft'"`

	// Associations
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
}

// TableName specifies the table name
func (SSOConnection) TableName() string {
	return "sso_connections"
}

// IsSAML returns true if protocol is SAML
func (s *SSOConnection) IsSAML() bool {
	return s.Protocol == SSOProtocolSAML
}

// IsOIDC returns true if protocol is OIDC
func (s *SSOConnection) IsOIDC() bool {
	return s.Protocol == SSOProtocolOIDC
}

// IsActive returns true if connection is active
func (s *SSOConnection) IsActive() bool {
	return s.Status == SSOStatusActive
}

// IsDomainVerified reports whether the organization proved it owns Domain.
func (s *SSOConnection) IsDomainVerified() bool {
	return s.DomainVerifiedAt != nil
}

// ProvisionsMembers reports whether SSO sign-in may add new members.
func (s *SSOConnection) ProvisionsMembers() bool {
	return s.JITProvisioning || s.AutoProvision
}

// SSODomainVerificationPrefix starts the TXT record value.
const SSODomainVerificationPrefix = "passwall-verification="

// DomainVerificationRecordName is the DNS name that must hold the TXT record.
func (s *SSOConnection) DomainVerificationRecordName() string {
	return "_passwall-verification." + s.Domain
}

// DomainVerificationRecordValue is the TXT record value proving ownership.
func (s *SSOConnection) DomainVerificationRecordValue() string {
	if s.DomainVerificationToken == "" {
		return ""
	}
	return SSODomainVerificationPrefix + s.DomainVerificationToken
}

// SSODefaultRoleAllowed reports whether a role may be given automatically to
// members who join through SSO. Elevated roles must be granted by an admin.
func SSODefaultRoleAllowed(role OrganizationRole) bool {
	return NormalizeOrgRole(role) == OrgRoleMember
}

// SSOState stores transient SSO authentication state (CSRF protection)
type SSOState struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	State          string `json:"state" gorm:"type:varchar(512);not null;uniqueIndex"`
	ConnectionID   uint   `json:"connection_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	OrganizationID uint   `json:"organization_id" gorm:"not null;index"`
	RedirectURL    string `json:"redirect_url" gorm:"type:varchar(2048)"`
	CodeVerifier   string `json:"-" gorm:"type:varchar(512)"`
	Nonce          string `json:"-" gorm:"type:varchar(512)"`
	// ClientCodeChallenge binds the login code to the browser that started
	// the flow (S256 of a verifier the client keeps).
	ClientCodeChallenge string    `json:"-" gorm:"type:varchar(128)"`
	ExpiresAt           time.Time `json:"expires_at" gorm:"not null;index"`
}

// TableName specifies the table name
func (SSOState) TableName() string {
	return "sso_states"
}

// IsExpired checks if the state has expired
func (s *SSOState) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// SSOLoginCode is a single-use code handed to the client after a successful
// IdP callback. The client exchanges it (with its PKCE verifier) for a
// session, so no token travels in a URL.
type SSOLoginCode struct {
	ID                  uint      `gorm:"primary_key" json:"id"`
	CreatedAt           time.Time `json:"created_at"`
	CodeHash            string    `json:"-" gorm:"type:varchar(64);not null;uniqueIndex"`
	UserID              uint      `json:"user_id" gorm:"not null"`
	ConnectionID        uint      `json:"connection_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	OrganizationID      uint      `json:"organization_id" gorm:"not null"`
	ClientCodeChallenge string    `json:"-" gorm:"type:varchar(128);not null"`
	ExpiresAt           time.Time `json:"expires_at" gorm:"not null;index"`
}

// TableName specifies the table name
func (SSOLoginCode) TableName() string {
	return "sso_login_codes"
}

// --- DTOs ---

// SSOConnectionDTO for API responses (sensitive fields stripped)
type SSOConnectionDTO struct {
	ID               uint                `json:"id"`
	UUID             uuid.UUID           `json:"uuid"`
	OrganizationID   uint                `json:"organization_id"`
	Protocol         SSOProtocol         `json:"protocol"`
	Name             string              `json:"name"`
	Domain           string              `json:"domain"`
	SPEntityID       string              `json:"sp_entity_id"`
	SPAcsURL         string              `json:"sp_acs_url"`
	AutoProvision    bool                `json:"auto_provision"`
	DefaultRole      OrganizationRole    `json:"default_role"`
	JITProvisioning  bool                `json:"jit_provisioning"`
	KeyEscrowEnabled bool                `json:"key_escrow_enabled"` // always false; key escrow was removed
	Status           SSOConnectionStatus `json:"status"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`

	DomainVerified     bool                      `json:"domain_verified"`
	DomainVerifiedAt   *time.Time                `json:"domain_verified_at,omitempty"`
	DomainVerification *SSODomainVerificationDTO `json:"domain_verification,omitempty"`

	// Protocol-specific (admin-visible only)
	SAMLConfig *SAMLConfigDTO `json:"saml_config,omitempty"`
	OIDCConfig *OIDCConfigDTO `json:"oidc_config,omitempty"`
}

// SSODomainVerificationDTO tells the admin which DNS record to publish.
type SSODomainVerificationDTO struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SAMLConfigDTO strips the certificate body for list views
type SAMLConfigDTO struct {
	EntityID            string `json:"entity_id"`
	SSOURL              string `json:"sso_url"`
	SLOURL              string `json:"slo_url,omitempty"`
	HasCertificate      bool   `json:"has_certificate"`
	SignAuthnRequests   bool   `json:"sign_authn_requests"`
	WantAssertionSigned bool   `json:"want_assertion_signed"`
	NameIDFormat        string `json:"name_id_format,omitempty"`
}

// OIDCConfigDTO strips the client secret
type OIDCConfigDTO struct {
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	AuthURL      string   `json:"auth_url,omitempty"`
	TokenURL     string   `json:"token_url,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	UseDiscovery bool     `json:"use_discovery"`
	PKCEEnabled  bool     `json:"pkce_enabled"`
}

// ToSSOConnectionDTO converts SSOConnection to DTO
func ToSSOConnectionDTO(conn *SSOConnection) *SSOConnectionDTO {
	if conn == nil {
		return nil
	}

	dto := &SSOConnectionDTO{
		ID:               conn.ID,
		UUID:             conn.UUID,
		OrganizationID:   conn.OrganizationID,
		Protocol:         conn.Protocol,
		Name:             conn.Name,
		Domain:           conn.Domain,
		SPEntityID:       conn.SPEntityID,
		SPAcsURL:         conn.SPAcsURL,
		AutoProvision:    conn.AutoProvision,
		DefaultRole:      conn.DefaultRole,
		JITProvisioning:  conn.JITProvisioning,
		Status:           conn.Status,
		CreatedAt:        conn.CreatedAt,
		UpdatedAt:        conn.UpdatedAt,
		DomainVerified:   conn.IsDomainVerified(),
		DomainVerifiedAt: conn.DomainVerifiedAt,
	}
	if value := conn.DomainVerificationRecordValue(); value != "" {
		dto.DomainVerification = &SSODomainVerificationDTO{
			Type:  "TXT",
			Name:  conn.DomainVerificationRecordName(),
			Value: value,
		}
	}

	if conn.SAMLConfig != nil {
		dto.SAMLConfig = &SAMLConfigDTO{
			EntityID:            conn.SAMLConfig.EntityID,
			SSOURL:              conn.SAMLConfig.SSOURL,
			SLOURL:              conn.SAMLConfig.SLOURL,
			HasCertificate:      conn.SAMLConfig.Certificate != "",
			SignAuthnRequests:   conn.SAMLConfig.SignAuthnRequests,
			WantAssertionSigned: conn.SAMLConfig.WantAssertionSigned,
			NameIDFormat:        conn.SAMLConfig.NameIDFormat,
		}
	}

	if conn.OIDCConfig != nil {
		dto.OIDCConfig = &OIDCConfigDTO{
			Issuer:       conn.OIDCConfig.Issuer,
			ClientID:     conn.OIDCConfig.ClientID,
			AuthURL:      conn.OIDCConfig.AuthURL,
			TokenURL:     conn.OIDCConfig.TokenURL,
			Scopes:       conn.OIDCConfig.Scopes,
			UseDiscovery: conn.OIDCConfig.UseDiscovery,
			PKCEEnabled:  conn.OIDCConfig.PKCEEnabled,
		}
	}

	return dto
}

// --- Request DTOs ---

// CreateSSOConnectionRequest for creating a new SSO connection
type CreateSSOConnectionRequest struct {
	Protocol        SSOProtocol      `json:"protocol" binding:"required,oneof=saml oidc"`
	Name            string           `json:"name" binding:"required,max=255"`
	Domain          string           `json:"domain" binding:"required,max=255"`
	SAMLConfig      *SAMLConfig      `json:"saml_config,omitempty"`
	OIDCConfig      *OIDCConfig      `json:"oidc_config,omitempty"`
	AutoProvision   *bool            `json:"auto_provision,omitempty"`
	DefaultRole     OrganizationRole `json:"default_role,omitempty"`
	JITProvisioning *bool            `json:"jit_provisioning,omitempty"`
}

// UpdateSSOConnectionRequest for updating an SSO connection
type UpdateSSOConnectionRequest struct {
	Name            *string           `json:"name,omitempty" binding:"omitempty,max=255"`
	Domain          *string           `json:"domain,omitempty" binding:"omitempty,max=255"`
	SAMLConfig      *SAMLConfig       `json:"saml_config,omitempty"`
	OIDCConfig      *OIDCConfig       `json:"oidc_config,omitempty"`
	AutoProvision   *bool             `json:"auto_provision,omitempty"`
	DefaultRole     *OrganizationRole `json:"default_role,omitempty"`
	JITProvisioning *bool             `json:"jit_provisioning,omitempty"`
	// Activation goes through the activate endpoint, which validates config
	// and domain ownership.
	Status *SSOConnectionStatus `json:"status,omitempty" binding:"omitempty,oneof=draft inactive"`
}

// SSOInitiateRequest for starting SSO login
type SSOInitiateRequest struct {
	Domain      string `json:"domain" binding:"required"`
	RedirectURL string `json:"redirect_url,omitempty"`
	// CodeChallenge is base64url(SHA-256(code_verifier)); the verifier is
	// required to exchange the login code.
	CodeChallenge string `json:"code_challenge" binding:"required,min=43,max=128"`
}

// SSOExchangeRequest trades a single-use login code for a session.
type SSOExchangeRequest struct {
	Code         string `json:"code" binding:"required,max=128"`
	CodeVerifier string `json:"code_verifier" binding:"required,min=43,max=128"`
	App          string `json:"app,omitempty" binding:"omitempty,max=32"`
	DeviceID     string `json:"device_id,omitempty" binding:"omitempty,max=64"`
}

// SSOCallbackResult is the outcome of a validated IdP callback.
type SSOCallbackResult struct {
	Code        string `json:"-"`
	RedirectURL string `json:"-"`
	UserID      uint   `json:"-"`
	OrgID       uint   `json:"-"`
	// Provisioned is true when this sign-in created the membership (JIT).
	Provisioned bool `json:"-"`
}

// SSOOrganizationDTO identifies the organization the user signed in through.
type SSOOrganizationDTO struct {
	ID       uint   `json:"id"`
	PublicID string `json:"public_id"`
	Name     string `json:"name"`
}

// SSOExchangeResponse is a regular sign-in response plus the organization.
// When the user has 2FA, only two_factor_required/two_factor_token are set.
type SSOExchangeResponse struct {
	*AuthResponse
	Organization *SSOOrganizationDTO `json:"organization,omitempty"`
}

// SSO activity types (organization audit log).
const (
	ActivityTypeSSOSignIn            ActivityType = "sso_signin"
	ActivityTypeSSOMemberProvisioned ActivityType = "sso_member_provisioned"
	ActivityTypeSSOConnectionChanged ActivityType = "sso_connection_changed"
	ActivityTypeSSODomainVerified    ActivityType = "sso_domain_verified"
	ActivityTypeSCIMMemberChanged    ActivityType = "scim_member_changed"
)
