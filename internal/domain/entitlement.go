package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type AccessState string

const (
	AccessStatePaid          AccessState = "paid"
	AccessStateFree          AccessState = "free"
	AccessStateFreeOverQuota AccessState = "free_over_quota"
	AccessStateFrozen        AccessState = "frozen"
)

type ExpiryBehavior string

const (
	ExpiryBehaviorDowngradeToFree ExpiryBehavior = "downgrade_to_free"
	ExpiryBehaviorFreeze          ExpiryBehavior = "freeze"
)

type EnforcementMode string

const (
	EnforcementModeOff     EnforcementMode = "off"
	EnforcementModeLog     EnforcementMode = "log"
	EnforcementModeEnforce EnforcementMode = "enforce"
)

type Capability string

const (
	CapabilityVaultRead                Capability = "vault.read"
	CapabilityVaultAutofill            Capability = "vault.autofill"
	CapabilityItemCreate               Capability = "item.create"
	CapabilityItemUpdate               Capability = "item.update"
	CapabilityItemDelete               Capability = "item.delete"
	CapabilityCollectionCreate         Capability = "collection.create"
	CapabilityCollectionUpdate         Capability = "collection.update"
	CapabilityCollectionDelete         Capability = "collection.delete"
	CapabilityFolderCreate             Capability = "folder.create"
	CapabilityFolderUpdate             Capability = "folder.update"
	CapabilityFolderDelete             Capability = "folder.delete"
	CapabilityOrganizationUpdate       Capability = "organization.update"
	CapabilityOrganizationDelete       Capability = "organization.delete"
	CapabilityOrganizationSettings     Capability = "organization.settings.update"
	CapabilityMemberInvite             Capability = "member.invite"
	CapabilityMemberUpdate             Capability = "member.update"
	CapabilityMemberRemove             Capability = "member.remove"
	CapabilityPasskeyCreate            Capability = "passkey.create"
	CapabilitySharingCreate            Capability = "sharing.create"
	CapabilitySecureSendCreate         Capability = "secure_send.create"
	CapabilityEmergencyAccessCreate    Capability = "emergency_access.create"
	CapabilityTeamsManage              Capability = "teams.manage"
	CapabilityAuditRead                Capability = "audit.read"
	CapabilitySSOManage                Capability = "sso.manage"
	CapabilitySCIMManage               Capability = "scim.manage"
	CapabilitySCIMDeprovision          Capability = "scim.deprovision"
	CapabilityAccessRevoke             Capability = "access.revoke"
	CapabilityPoliciesDisable          Capability = "policies.disable"
	CapabilityPoliciesManage           Capability = "policies.manage"
	CapabilityBusinessPoliciesManage   Capability = "policies.business.manage"
	CapabilityEnterprisePoliciesManage Capability = "policies.enterprise.manage"
	CapabilitySecurityInsightsRead     Capability = "security_insights.read"
	CapabilityBreachMonitoringRead     Capability = "breach_monitoring.read"
)

type EntitlementReason string

const (
	EntitlementReasonAllowed             EntitlementReason = "allowed"
	EntitlementReasonAccountFrozen       EntitlementReason = "account_frozen"
	EntitlementReasonFeatureUnavailable  EntitlementReason = "feature_not_available"
	EntitlementReasonPlanLimitReached    EntitlementReason = "plan_limit_reached"
	EntitlementReasonSubscriptionExpired EntitlementReason = "subscription_expired"
)

// CapabilityDecision is the effective server decision for a capability.
// AdvisoryReason is set when the plan would deny the capability but the
// server is not enforcing it yet (enforcement mode off/log); clients must
// treat the capability as allowed and may surface the reason as a warning.
type CapabilityDecision struct {
	Allowed        bool              `json:"allowed"`
	Reason         EntitlementReason `json:"reason"`
	AdvisoryReason EntitlementReason `json:"advisory_reason,omitempty"`
}

type CapabilityRule struct {
	FeatureKey    string
	LimitKey      string
	AllowedStates []AccessState
}

func CapabilityRegistry() map[Capability]CapabilityRule {
	nonFrozen := []AccessState{AccessStatePaid, AccessStateFree, AccessStateFreeOverQuota}
	allStates := append(append([]AccessState{}, nonFrozen...), AccessStateFrozen)
	return map[Capability]CapabilityRule{
		CapabilityVaultRead:                {AllowedStates: allStates},
		CapabilityVaultAutofill:            {AllowedStates: nonFrozen},
		CapabilityItemCreate:               {LimitKey: "max_items", AllowedStates: []AccessState{AccessStatePaid, AccessStateFree}},
		CapabilityItemUpdate:               {AllowedStates: nonFrozen},
		CapabilityItemDelete:               {AllowedStates: nonFrozen},
		CapabilityCollectionCreate:         {LimitKey: "max_collections", AllowedStates: []AccessState{AccessStatePaid, AccessStateFree}},
		CapabilityCollectionUpdate:         {AllowedStates: nonFrozen},
		CapabilityCollectionDelete:         {AllowedStates: nonFrozen},
		CapabilityFolderCreate:             {AllowedStates: nonFrozen},
		CapabilityFolderUpdate:             {AllowedStates: nonFrozen},
		CapabilityFolderDelete:             {AllowedStates: nonFrozen},
		CapabilityOrganizationUpdate:       {AllowedStates: nonFrozen},
		CapabilityOrganizationDelete:       {AllowedStates: allStates},
		CapabilityOrganizationSettings:     {AllowedStates: nonFrozen},
		CapabilityMemberInvite:             {LimitKey: "max_users", AllowedStates: nonFrozen},
		CapabilityMemberUpdate:             {AllowedStates: nonFrozen},
		CapabilityMemberRemove:             {AllowedStates: allStates},
		CapabilityAccessRevoke:             {AllowedStates: allStates},
		CapabilityPoliciesDisable:          {AllowedStates: allStates},
		CapabilityPasskeyCreate:            {FeatureKey: "passkeys", AllowedStates: nonFrozen},
		CapabilitySharingCreate:            {FeatureKey: "sharing", AllowedStates: nonFrozen},
		CapabilitySecureSendCreate:         {FeatureKey: "secure_send", AllowedStates: nonFrozen},
		CapabilityEmergencyAccessCreate:    {FeatureKey: "emergency_access", AllowedStates: nonFrozen},
		CapabilityTeamsManage:              {FeatureKey: "teams", AllowedStates: nonFrozen},
		CapabilityAuditRead:                {FeatureKey: "audit", AllowedStates: allStates},
		CapabilitySSOManage:                {FeatureKey: "sso", AllowedStates: nonFrozen},
		CapabilitySCIMManage:               {FeatureKey: "sso", AllowedStates: nonFrozen},
		CapabilitySCIMDeprovision:          {FeatureKey: "sso", AllowedStates: allStates},
		CapabilityPoliciesManage:           {FeatureKey: "policies", AllowedStates: nonFrozen},
		CapabilityBusinessPoliciesManage:   {FeatureKey: "business_policies", AllowedStates: nonFrozen},
		CapabilityEnterprisePoliciesManage: {FeatureKey: "enterprise_policies", AllowedStates: nonFrozen},
		CapabilitySecurityInsightsRead:     {FeatureKey: "security_insights", AllowedStates: allStates},
		CapabilityBreachMonitoringRead:     {FeatureKey: "breach_monitoring", AllowedStates: allStates},
	}
}

type EntitlementLimits struct {
	MaxUsers       *int `json:"max_users"`
	MaxCollections *int `json:"max_collections"`
	MaxItems       *int `json:"max_items"`
	MaxDevices     *int `json:"max_devices"`
}

type EntitlementUsage struct {
	Users       int `json:"users"`
	Collections int `json:"collections"`
	Items       int `json:"items"`
}

type EntitlementSnapshot struct {
	SchemaVersion     int                               `json:"schema_version"`
	OrganizationID    uint                              `json:"organization_id"`
	EffectivePlan     string                            `json:"effective_plan"`
	SubscriptionState SubscriptionState                 `json:"subscription_state"`
	AccessState       AccessState                       `json:"access_state"`
	Capabilities      map[Capability]CapabilityDecision `json:"capabilities"`
	Features          PlanFeatures                      `json:"features"`
	Limits            EntitlementLimits                 `json:"limits"`
	Usage             EntitlementUsage                  `json:"usage"`
	PeriodEnd         *time.Time                        `json:"period_end,omitempty"`
	GraceUntil        *time.Time                        `json:"grace_until,omitempty"`
	ManageBillingURL  string                            `json:"manage_billing_url"`
}

type EntitlementOverrideValue map[string]any

func (v *EntitlementOverrideValue) Scan(value any) error {
	if value == nil {
		*v = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan entitlement override: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, v)
}

func (v EntitlementOverrideValue) Value() (driver.Value, error) {
	return json.Marshal(v)
}

type OrganizationEntitlementOverride struct {
	ID             uint                     `gorm:"primary_key" json:"id"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
	OrganizationID uint                     `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	Key            string                   `json:"key" gorm:"type:varchar(100);not null;index"`
	Value          EntitlementOverrideValue `json:"value" gorm:"type:jsonb;not null"`
	Reason         string                   `json:"reason" gorm:"type:varchar(255);not null"`
	ExpiresAt      *time.Time               `json:"expires_at,omitempty" gorm:"index"`
	CreatedBy      uint                     `json:"created_by" gorm:"not null"`
}

func (OrganizationEntitlementOverride) TableName() string {
	return "organization_entitlement_overrides"
}
