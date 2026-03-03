package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	uuid "github.com/satori/go.uuid"
)

// PolicyType represents the type of organization policy
type PolicyType string

const (
	// Authentication & Access Policies
	PolicyRequireTwoFactor     PolicyType = "require_two_factor"
	PolicyRequireSSO           PolicyType = "require_sso"
	PolicyMasterPWRequirements PolicyType = "master_password_requirements"
	PolicySessionTimeout       PolicyType = "session_timeout"
	PolicyRemovePINUnlock      PolicyType = "remove_pin_unlock"
	PolicyFailedLoginLimit     PolicyType = "failed_login_limit"
	PolicyAccountRecovery      PolicyType = "account_recovery"

	// Vault & Data Policies
	PolicySingleOrganization     PolicyType = "single_organization"
	PolicyDisablePersonalExport  PolicyType = "disable_personal_export"
	PolicyEnforceDataOwnership   PolicyType = "enforce_data_ownership"
	PolicyRemoveCardType         PolicyType = "remove_card_type"
	PolicyDisableExternalSharing PolicyType = "disable_external_sharing"

	// Password Generation
	PolicyPasswordGenerator PolicyType = "password_generator"

	// Autofill & Browser
	PolicyActivateAutofill        PolicyType = "activate_autofill"
	PolicyDefaultURIMatch         PolicyType = "default_uri_match"
	PolicyRequireAutofillConfirm  PolicyType = "require_autofill_confirmation"
	PolicyRequireBrowserExtension PolicyType = "require_browser_extension"

	// Network & IP
	PolicyFirewallRules              PolicyType = "firewall_rules"
	PolicyBlockDomainAccountCreation PolicyType = "block_domain_account_creation"

	// Sharing & Send
	PolicySendOptions PolicyType = "send_options"
	PolicyRemoveSend  PolicyType = "remove_send"
)

// PolicyTier defines the minimum plan tier required for a policy
type PolicyTier string

const (
	PolicyTierTeam       PolicyTier = "team"
	PolicyTierBusiness   PolicyTier = "business"
	PolicyTierEnterprise PolicyTier = "enterprise"
)

// PolicyData stores policy-specific configuration as JSON
type PolicyData map[string]interface{}

func (d *PolicyData) Scan(value interface{}) error {
	if value == nil {
		*d = make(PolicyData)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan PolicyData: expected []byte, got %T", value)
	}

	return json.Unmarshal(bytes, d)
}

func (d PolicyData) Value() (driver.Value, error) {
	if d == nil {
		return json.Marshal(map[string]interface{}{})
	}
	return json.Marshal(d)
}

// OrganizationPolicy represents a policy enforced on organization members
type OrganizationPolicy struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID uint       `json:"organization_id" gorm:"not null;uniqueIndex:uq_org_policy_type,priority:1;constraint:OnDelete:CASCADE"`
	Type           PolicyType `json:"type" gorm:"type:varchar(50);not null;uniqueIndex:uq_org_policy_type,priority:2"`
	Enabled        bool       `json:"enabled" gorm:"not null;default:false"`
	Data           PolicyData `json:"data" gorm:"type:jsonb;not null;default:'{}'"`

	// Audit trail
	LastEnabledByUserID  *uint `json:"last_enabled_by_user_id,omitempty"`
	LastDisabledByUserID *uint `json:"last_disabled_by_user_id,omitempty"`

	Organization *Organization `json:"-" gorm:"foreignKey:OrganizationID"`
}

func (OrganizationPolicy) TableName() string {
	return "organization_policies"
}

// OrganizationPolicyDTO for API responses
type OrganizationPolicyDTO struct {
	ID             uint       `json:"id"`
	UUID           uuid.UUID  `json:"uuid"`
	OrganizationID uint       `json:"organization_id"`
	Type           PolicyType `json:"type"`
	Enabled        bool       `json:"enabled"`
	Data           PolicyData `json:"data"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func ToOrganizationPolicyDTO(p *OrganizationPolicy) *OrganizationPolicyDTO {
	if p == nil {
		return nil
	}
	return &OrganizationPolicyDTO{
		ID:             p.ID,
		UUID:           p.UUID,
		OrganizationID: p.OrganizationID,
		Type:           p.Type,
		Enabled:        p.Enabled,
		Data:           p.Data,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

// UpdateOrganizationPolicyRequest for enabling/disabling and configuring a policy
type UpdateOrganizationPolicyRequest struct {
	Enabled *bool      `json:"enabled"`
	Data    PolicyData `json:"data,omitempty"`
}

// PolicyDefinition describes a policy type with its metadata (not stored in DB)
type PolicyDefinition struct {
	Type         PolicyType   `json:"type"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Category     string       `json:"category"`
	Tier         PolicyTier   `json:"tier"`
	Dependencies []PolicyType `json:"dependencies,omitempty"`
}

// AllPolicyDefinitions returns the complete catalog of available policies.
// This is used to present the full list to clients, regardless of which are
// currently persisted in the database.
func AllPolicyDefinitions() []PolicyDefinition {
	return []PolicyDefinition{
		// Authentication & Access
		{
			Type: PolicyRequireTwoFactor, Name: "Require Two-Factor Authentication",
			Description: "Require all members to use two-step login for vault access",
			Category:    "authentication", Tier: PolicyTierTeam,
		},
		{
			Type: PolicyRequireSSO, Name: "Require Single Sign-On",
			Description: "Require non-owner/non-admin members to authenticate via SSO",
			Category:    "authentication", Tier: PolicyTierEnterprise,
			Dependencies: []PolicyType{PolicySingleOrganization},
		},
		{
			Type: PolicyMasterPWRequirements, Name: "Master Password Requirements",
			Description: "Enforce minimum password complexity, length, and character requirements",
			Category:    "authentication", Tier: PolicyTierTeam,
		},
		{
			Type: PolicySessionTimeout, Name: "Session Timeout",
			Description: "Set maximum vault timeout duration and timeout action for members",
			Category:    "authentication", Tier: PolicyTierBusiness,
			Dependencies: []PolicyType{PolicySingleOrganization},
		},
		{
			Type: PolicyRemovePINUnlock, Name: "Remove Unlock with PIN",
			Description: "Prohibit members from using PIN unlock on web, browser, and desktop apps",
			Category:    "authentication", Tier: PolicyTierBusiness,
		},
		{
			Type: PolicyFailedLoginLimit, Name: "Failed Login Attempt Limit",
			Description: "Temporarily block IP after specified number of failed sign-in attempts",
			Category:    "authentication", Tier: PolicyTierEnterprise,
		},
		{
			Type: PolicyAccountRecovery, Name: "Account Recovery Administration",
			Description: "Allow admins to reset member master passwords and restore account access",
			Category:    "authentication", Tier: PolicyTierBusiness,
			Dependencies: []PolicyType{PolicySingleOrganization},
		},

		// Vault & Data
		{
			Type: PolicySingleOrganization, Name: "Single Organization",
			Description: "Restrict members from joining or creating other organizations",
			Category:    "vault", Tier: PolicyTierBusiness,
		},
		{
			Type: PolicyDisablePersonalExport, Name: "Disable Personal Vault Export",
			Description: "Prevent non-admin members from exporting their vault data",
			Category:    "vault", Tier: PolicyTierBusiness,
		},
		{
			Type: PolicyEnforceDataOwnership, Name: "Enforce Organization Data Ownership",
			Description: "All saved items belong to the organization, retained on member departure",
			Category:    "vault", Tier: PolicyTierEnterprise,
		},
		{
			Type: PolicyRemoveCardType, Name: "Remove Card Item Type",
			Description: "Prevent members from creating or importing credit card items",
			Category:    "vault", Tier: PolicyTierBusiness,
		},
		{
			Type: PolicyDisableExternalSharing, Name: "Disable External Sharing",
			Description: "Prevent sharing outside the organization; allow only via shared collections",
			Category:    "vault", Tier: PolicyTierTeam,
		},

		// Password Generation
		{
			Type: PolicyPasswordGenerator, Name: "Password Generator Requirements",
			Description: "Enforce minimum standards for generated passwords and passphrases",
			Category:    "generator", Tier: PolicyTierTeam,
		},

		// Autofill & Browser
		{
			Type: PolicyActivateAutofill, Name: "Activate Autofill",
			Description: "Automatically enable autofill on page load for all members",
			Category:    "autofill", Tier: PolicyTierBusiness,
		},
		{
			Type: PolicyDefaultURIMatch, Name: "Default URI Match Detection",
			Description: "Set the default URI match detection method for the organization",
			Category:    "autofill", Tier: PolicyTierBusiness,
			Dependencies: []PolicyType{PolicySingleOrganization},
		},
		{
			Type: PolicyRequireAutofillConfirm, Name: "Require Autofill Confirmation",
			Description: "Require confirmation before autofilling credit cards, addresses, or logins",
			Category:    "autofill", Tier: PolicyTierBusiness,
		},
		{
			Type: PolicyRequireBrowserExtension, Name: "Require Browser Extension",
			Description: "Require members to install the browser extension during signup",
			Category:    "autofill", Tier: PolicyTierTeam,
		},

		// Network & IP
		{
			Type: PolicyFirewallRules, Name: "Firewall Rules",
			Description: "Restrict vault access by IP address, geographic location, or anonymous IP type",
			Category:    "network", Tier: PolicyTierEnterprise,
		},
		{
			Type: PolicyBlockDomainAccountCreation, Name: "Block Domain Account Creation",
			Description: "Prevent account creation outside the organization for claimed domain emails",
			Category:    "network", Tier: PolicyTierEnterprise,
		},

		// Sharing & Send
		{
			Type: PolicySendOptions, Name: "Send Options",
			Description: "Configure Send creation options, including email visibility requirements",
			Category:    "sharing", Tier: PolicyTierBusiness,
		},
		{
			Type: PolicyRemoveSend, Name: "Remove Send",
			Description: "Prevent non-admin members from creating or editing Sends",
			Category:    "sharing", Tier: PolicyTierEnterprise,
		},
	}
}

// policyTierMap provides O(1) lookup for tier requirements
var policyTierMap map[PolicyType]PolicyTier

func init() {
	policyTierMap = make(map[PolicyType]PolicyTier)
	for _, def := range AllPolicyDefinitions() {
		policyTierMap[def.Type] = def.Tier
	}
}

// GetPolicyTier returns the minimum plan tier required for a policy type
func GetPolicyTier(policyType PolicyType) (PolicyTier, bool) {
	tier, ok := policyTierMap[policyType]
	return tier, ok
}

// policyDependencyMap provides O(1) lookup for policy dependencies
var policyDependencyMap map[PolicyType][]PolicyType

func init() {
	policyDependencyMap = make(map[PolicyType][]PolicyType)
	for _, def := range AllPolicyDefinitions() {
		if len(def.Dependencies) > 0 {
			policyDependencyMap[def.Type] = def.Dependencies
		}
	}
}

// GetPolicyDependencies returns the prerequisite policies
func GetPolicyDependencies(policyType PolicyType) []PolicyType {
	return policyDependencyMap[policyType]
}

// IsValidPolicyType checks if a policy type is known
func IsValidPolicyType(policyType PolicyType) bool {
	_, ok := policyTierMap[policyType]
	return ok
}

// TierMeetsMinimum checks if actualTier meets or exceeds the required tier.
// Ordering: team < business < enterprise.
func TierMeetsMinimum(actualPlan OrganizationPlan, requiredTier PolicyTier) bool {
	planRank := map[OrganizationPlan]int{
		PlanFree:       0,
		PlanPro:        0,
		PlanFamily:     1,
		PlanTeam:       1,
		PlanBusiness:   2,
		PlanEnterprise: 3,
	}
	tierRank := map[PolicyTier]int{
		PolicyTierTeam:       1,
		PolicyTierBusiness:   2,
		PolicyTierEnterprise: 3,
	}
	return planRank[actualPlan] >= tierRank[requiredTier]
}
