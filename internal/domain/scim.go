package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	uuid "github.com/satori/go.uuid"
)

// SCIMToken represents an API bearer token used by IdP directory sync (SCIM 2.0)
type SCIMToken struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID uint   `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	Label          string `json:"label" gorm:"type:varchar(255);not null"`

	// Token value (hashed for storage, plain text returned only on creation)
	TokenHash string     `json:"-" gorm:"type:varchar(512);not null;uniqueIndex"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	IsActive   bool       `json:"is_active" gorm:"not null;default:true"`

	// Associations
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
}

// TableName specifies the table name
func (SCIMToken) TableName() string {
	return "scim_tokens"
}

// IsExpired checks if the token has expired
func (t *SCIMToken) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

// IsValid returns true if the token is active and not expired
func (t *SCIMToken) IsValid() bool {
	return t.IsActive && !t.IsExpired()
}

// GenerateSCIMToken creates a cryptographically secure random token string
func GenerateSCIMToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pwscim_" + hex.EncodeToString(b), nil
}

// SCIMTokenDTO for API responses
type SCIMTokenDTO struct {
	ID             uint       `json:"id"`
	UUID           uuid.UUID  `json:"uuid"`
	OrganizationID uint       `json:"organization_id"`
	Label          string     `json:"label"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
}

// SCIMTokenCreatedDTO includes the plain-text token (returned only on creation)
type SCIMTokenCreatedDTO struct {
	SCIMTokenDTO
	Token string `json:"token"`
}

// ToSCIMTokenDTO converts SCIMToken to DTO
func ToSCIMTokenDTO(t *SCIMToken) *SCIMTokenDTO {
	if t == nil {
		return nil
	}
	return &SCIMTokenDTO{
		ID:             t.ID,
		UUID:           t.UUID,
		OrganizationID: t.OrganizationID,
		Label:          t.Label,
		ExpiresAt:      t.ExpiresAt,
		LastUsedAt:     t.LastUsedAt,
		IsActive:       t.IsActive,
		CreatedAt:      t.CreatedAt,
	}
}

// CreateSCIMTokenRequest for generating a new SCIM token
type CreateSCIMTokenRequest struct {
	Label     string     `json:"label" binding:"required,max=255"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// --- SCIM 2.0 Resource Schemas (RFC 7643 / 7644) ---

// SCIMUser represents a SCIM 2.0 User resource
type SCIMUser struct {
	Schemas    []string        `json:"schemas"`
	ID         string          `json:"id"`
	ExternalID string          `json:"externalId,omitempty"`
	UserName   string          `json:"userName"`
	Name       *SCIMName       `json:"name,omitempty"`
	Emails     []SCIMEmail     `json:"emails,omitempty"`
	Active     bool            `json:"active"`
	Groups     []SCIMGroupRef  `json:"groups,omitempty"`
	Meta       *SCIMMeta       `json:"meta,omitempty"`
}

// SCIMName represents the name component of a SCIM user
type SCIMName struct {
	Formatted  string `json:"formatted,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
}

// SCIMEmail represents an email in the SCIM schema
type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// SCIMGroupRef is a reference to a group the user belongs to
type SCIMGroupRef struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// SCIMGroup represents a SCIM 2.0 Group resource
type SCIMGroup struct {
	Schemas    []string         `json:"schemas"`
	ID         string           `json:"id"`
	ExternalID string           `json:"externalId,omitempty"`
	DisplayName string          `json:"displayName"`
	Members    []SCIMMemberRef  `json:"members,omitempty"`
	Meta       *SCIMMeta        `json:"meta,omitempty"`
}

// SCIMMemberRef is a reference to a group member
type SCIMMemberRef struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}

// SCIMMeta holds resource metadata per SCIM spec
type SCIMMeta struct {
	ResourceType string `json:"resourceType"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Location     string `json:"location,omitempty"`
}

// SCIMListResponse wraps a SCIM list with pagination envelope
type SCIMListResponse struct {
	Schemas      []string    `json:"schemas"`
	TotalResults int         `json:"totalResults"`
	StartIndex   int         `json:"startIndex"`
	ItemsPerPage int         `json:"itemsPerPage"`
	Resources    interface{} `json:"Resources"`
}

// SCIMError represents a SCIM 2.0 error response
type SCIMError struct {
	Schemas  []string `json:"schemas"`
	Detail   string   `json:"detail"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
}

// SCIMPatchOp represents a SCIM PATCH operation (RFC 7644 §3.5.2)
type SCIMPatchOp struct {
	Schemas    []string          `json:"schemas"`
	Operations []SCIMPatchOpItem `json:"Operations"`
}

// SCIMPatchOpItem is a single patch operation
type SCIMPatchOpItem struct {
	Op    string      `json:"op"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// SCIM 2.0 schema URNs
const (
	SCIMSchemaUser           = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMSchemaGroup          = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SCIMSchemaListResponse   = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SCIMSchemaError          = "urn:ietf:params:scim:api:messages:2.0:Error"
	SCIMSchemaPatchOp        = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	SCIMSchemaServiceConfig  = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	SCIMSchemaResourceType   = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	SCIMSchemaSchema         = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)
