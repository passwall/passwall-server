package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	uuid "github.com/satori/go.uuid"
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
	SignAuthnRequests    bool   `json:"sign_authn_requests"`
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
	Issuer        string   `json:"issuer"`
	ClientID      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	AuthURL       string   `json:"auth_url,omitempty"`
	TokenURL      string   `json:"token_url,omitempty"`
	UserInfoURL   string   `json:"user_info_url,omitempty"`
	JwksURI       string   `json:"jwks_uri,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
	UseDiscovery  bool     `json:"use_discovery"`
	PKCEEnabled   bool     `json:"pkce_enabled"`
	EmailClaim    string   `json:"email_claim,omitempty"`
	NameClaim     string   `json:"name_claim,omitempty"`
	GroupsClaim   string   `json:"groups_claim,omitempty"`
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
	Name   string `json:"name" gorm:"type:varchar(255);not null"`
	Domain string `json:"domain" gorm:"type:varchar(255);not null;uniqueIndex"`

	// Protocol-specific configuration (stored as JSONB)
	SAMLConfig *SAMLConfig `json:"saml_config,omitempty" gorm:"type:jsonb"`
	OIDCConfig *OIDCConfig `json:"oidc_config,omitempty" gorm:"type:jsonb"`

	// SP (Passwall) metadata — generated at creation, read-only for admin
	SPEntityID  string `json:"sp_entity_id" gorm:"type:varchar(512)"`
	SPAcsURL    string `json:"sp_acs_url" gorm:"type:varchar(512)"`
	SPMetadata  string `json:"-" gorm:"type:text"`

	// Behaviour
	AutoProvision    bool               `json:"auto_provision" gorm:"default:true"`
	DefaultRole      OrganizationRole   `json:"default_role" gorm:"type:varchar(20);default:'member'"`
	JITProvisioning  bool               `json:"jit_provisioning" gorm:"default:true"`
	Status           SSOConnectionStatus `json:"status" gorm:"type:varchar(20);not null;default:'draft'"`

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

// SSOState stores transient SSO authentication state (CSRF protection)
type SSOState struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	State          string    `json:"state" gorm:"type:varchar(512);not null;uniqueIndex"`
	ConnectionID   uint      `json:"connection_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	OrganizationID uint      `json:"organization_id" gorm:"not null;index"`
	RedirectURL    string    `json:"redirect_url" gorm:"type:varchar(2048)"`
	CodeVerifier   string    `json:"-" gorm:"type:varchar(512)"`
	Nonce          string    `json:"-" gorm:"type:varchar(512)"`
	ExpiresAt      time.Time `json:"expires_at" gorm:"not null;index"`
}

// TableName specifies the table name
func (SSOState) TableName() string {
	return "sso_states"
}

// IsExpired checks if the state has expired
func (s *SSOState) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
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
	Status           SSOConnectionStatus `json:"status"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`

	// Protocol-specific (admin-visible only)
	SAMLConfig *SAMLConfigDTO `json:"saml_config,omitempty"`
	OIDCConfig *OIDCConfigDTO `json:"oidc_config,omitempty"`
}

// SAMLConfigDTO strips the certificate body for list views
type SAMLConfigDTO struct {
	EntityID            string `json:"entity_id"`
	SSOURL              string `json:"sso_url"`
	SLOURL              string `json:"slo_url,omitempty"`
	HasCertificate      bool   `json:"has_certificate"`
	SignAuthnRequests    bool   `json:"sign_authn_requests"`
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
		ID:              conn.ID,
		UUID:            conn.UUID,
		OrganizationID:  conn.OrganizationID,
		Protocol:        conn.Protocol,
		Name:            conn.Name,
		Domain:          conn.Domain,
		SPEntityID:      conn.SPEntityID,
		SPAcsURL:        conn.SPAcsURL,
		AutoProvision:   conn.AutoProvision,
		DefaultRole:     conn.DefaultRole,
		JITProvisioning: conn.JITProvisioning,
		Status:          conn.Status,
		CreatedAt:       conn.CreatedAt,
		UpdatedAt:       conn.UpdatedAt,
	}

	if conn.SAMLConfig != nil {
		dto.SAMLConfig = &SAMLConfigDTO{
			EntityID:            conn.SAMLConfig.EntityID,
			SSOURL:              conn.SAMLConfig.SSOURL,
			SLOURL:              conn.SAMLConfig.SLOURL,
			HasCertificate:      conn.SAMLConfig.Certificate != "",
			SignAuthnRequests:    conn.SAMLConfig.SignAuthnRequests,
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
	Name            *string          `json:"name,omitempty" binding:"omitempty,max=255"`
	Domain          *string          `json:"domain,omitempty" binding:"omitempty,max=255"`
	SAMLConfig      *SAMLConfig      `json:"saml_config,omitempty"`
	OIDCConfig      *OIDCConfig      `json:"oidc_config,omitempty"`
	AutoProvision   *bool            `json:"auto_provision,omitempty"`
	DefaultRole     *OrganizationRole `json:"default_role,omitempty"`
	JITProvisioning *bool            `json:"jit_provisioning,omitempty"`
	Status          *SSOConnectionStatus `json:"status,omitempty" binding:"omitempty,oneof=draft active inactive"`
}

// SSOInitiateRequest for starting SSO login
type SSOInitiateRequest struct {
	Domain      string `json:"domain" binding:"required"`
	RedirectURL string `json:"redirect_url,omitempty"`
}

// SSOCallbackResult returned after successful SSO authentication
type SSOCallbackResult struct {
	User         *User         `json:"user"`
	Organization *Organization `json:"organization"`
	IsNewUser    bool          `json:"is_new_user"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
}
