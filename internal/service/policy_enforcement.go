package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
)

// PolicyEnforcementService provides enforcement check methods that other
// services and handlers can call to gate features based on active org policies.
type PolicyEnforcementService interface {
	// CheckTwoFactorRequired returns an error if 2FA is required but the user has not set it up
	CheckTwoFactorRequired(ctx context.Context, orgID uint, userHas2FA bool) error

	// GetMasterPasswordRequirements returns the active password policy config or nil if not enabled
	GetMasterPasswordRequirements(ctx context.Context, orgID uint) (*MasterPasswordPolicy, error)

	// GetPasswordGeneratorRequirements returns the active generator policy config or nil if not enabled
	GetPasswordGeneratorRequirements(ctx context.Context, orgID uint) (*PasswordGeneratorPolicy, error)

	// CheckExternalSharingAllowed returns an error if external sharing is disabled
	CheckExternalSharingAllowed(ctx context.Context, orgID uint) error

	// CheckPersonalExportAllowed returns an error if personal export is disabled
	CheckPersonalExportAllowed(ctx context.Context, orgID uint, role domain.OrganizationRole) error

	// CheckSendAllowed returns an error if Send is disabled for non-admin members
	CheckSendAllowed(ctx context.Context, orgID uint, role domain.OrganizationRole) error

	// GetSessionTimeoutPolicy returns session timeout config or nil if not enabled
	GetSessionTimeoutPolicy(ctx context.Context, orgID uint) (*SessionTimeoutPolicy, error)
}

// MasterPasswordPolicy captures the parsed config for master password requirements
type MasterPasswordPolicy struct {
	MinLength         int  `json:"min_length"`
	RequireUppercase  bool `json:"require_uppercase"`
	RequireLowercase  bool `json:"require_lowercase"`
	RequireNumbers    bool `json:"require_numbers"`
	RequireSpecial    bool `json:"require_special"`
	MinSpecialCount   int  `json:"min_special_count"`
	MinComplexity     int  `json:"min_complexity"`
	RequireExistingChange bool `json:"require_existing_change"`
}

// PasswordGeneratorPolicy captures the parsed config for password generator requirements
type PasswordGeneratorPolicy struct {
	DefaultType       string `json:"type"`
	MinLength         int    `json:"min_length"`
	RequireUppercase  bool   `json:"require_uppercase"`
	RequireLowercase  bool   `json:"require_lowercase"`
	RequireNumbers    bool   `json:"require_numbers"`
	RequireSpecial    bool   `json:"require_special"`
	MinSpecialCount   int    `json:"min_special_count"`
	MinNumberCount    int    `json:"min_number_count"`
	PassphraseMinWords     int  `json:"passphrase_min_words"`
	PassphraseCapitalize   bool `json:"passphrase_capitalize"`
	PassphraseIncludeNumber bool `json:"passphrase_include_number"`
}

// SessionTimeoutPolicy captures the parsed config for session timeout requirements
type SessionTimeoutPolicy struct {
	MaxTimeoutMinutes int    `json:"max_timeout_minutes"`
	TimeoutAction     string `json:"timeout_action"`
}

type policyEnforcementService struct {
	policyService OrganizationPolicyService
}

// NewPolicyEnforcementService creates a new policy enforcement service
func NewPolicyEnforcementService(policyService OrganizationPolicyService) PolicyEnforcementService {
	return &policyEnforcementService{policyService: policyService}
}

func (s *policyEnforcementService) CheckTwoFactorRequired(ctx context.Context, orgID uint, userHas2FA bool) error {
	enabled, err := s.policyService.IsPolicyEnabled(ctx, orgID, domain.PolicyRequireTwoFactor)
	if err != nil {
		return err
	}
	if enabled && !userHas2FA {
		return fmt.Errorf("organization policy requires two-factor authentication")
	}
	return nil
}

func (s *policyEnforcementService) GetMasterPasswordRequirements(ctx context.Context, orgID uint) (*MasterPasswordPolicy, error) {
	data, err := s.policyService.GetPolicyData(ctx, orgID, domain.PolicyMasterPWRequirements)
	if err != nil || data == nil {
		return nil, err
	}
	return parseMasterPasswordPolicy(data), nil
}

func (s *policyEnforcementService) GetPasswordGeneratorRequirements(ctx context.Context, orgID uint) (*PasswordGeneratorPolicy, error) {
	data, err := s.policyService.GetPolicyData(ctx, orgID, domain.PolicyPasswordGenerator)
	if err != nil || data == nil {
		return nil, err
	}
	return parsePasswordGeneratorPolicy(data), nil
}

func (s *policyEnforcementService) CheckExternalSharingAllowed(ctx context.Context, orgID uint) error {
	enabled, err := s.policyService.IsPolicyEnabled(ctx, orgID, domain.PolicyDisableExternalSharing)
	if err != nil {
		return err
	}
	if enabled {
		return fmt.Errorf("organization policy prohibits sharing outside the organization")
	}
	return nil
}

func (s *policyEnforcementService) CheckPersonalExportAllowed(ctx context.Context, orgID uint, role domain.OrganizationRole) error {
	if role == domain.OrgRoleOwner || role == domain.OrgRoleAdmin {
		return nil
	}
	enabled, err := s.policyService.IsPolicyEnabled(ctx, orgID, domain.PolicyDisablePersonalExport)
	if err != nil {
		return err
	}
	if enabled {
		return fmt.Errorf("organization policy prohibits personal vault export")
	}
	return nil
}

func (s *policyEnforcementService) CheckSendAllowed(ctx context.Context, orgID uint, role domain.OrganizationRole) error {
	if role == domain.OrgRoleOwner || role == domain.OrgRoleAdmin {
		return nil
	}
	enabled, err := s.policyService.IsPolicyEnabled(ctx, orgID, domain.PolicyRemoveSend)
	if err != nil {
		return err
	}
	if enabled {
		return fmt.Errorf("organization policy prohibits creating Sends")
	}
	return nil
}

func (s *policyEnforcementService) GetSessionTimeoutPolicy(ctx context.Context, orgID uint) (*SessionTimeoutPolicy, error) {
	data, err := s.policyService.GetPolicyData(ctx, orgID, domain.PolicySessionTimeout)
	if err != nil || data == nil {
		return nil, err
	}
	return parseSessionTimeoutPolicy(data), nil
}

// --- Data parsers ---

func parseMasterPasswordPolicy(data domain.PolicyData) *MasterPasswordPolicy {
	p := &MasterPasswordPolicy{}
	if v, ok := data["min_length"].(float64); ok {
		p.MinLength = int(v)
	}
	if v, ok := data["require_uppercase"].(bool); ok {
		p.RequireUppercase = v
	}
	if v, ok := data["require_lowercase"].(bool); ok {
		p.RequireLowercase = v
	}
	if v, ok := data["require_numbers"].(bool); ok {
		p.RequireNumbers = v
	}
	if v, ok := data["require_special"].(bool); ok {
		p.RequireSpecial = v
	}
	if v, ok := data["min_special_count"].(float64); ok {
		p.MinSpecialCount = int(v)
	}
	if v, ok := data["min_complexity"].(float64); ok {
		p.MinComplexity = int(v)
	}
	if v, ok := data["require_existing_change"].(bool); ok {
		p.RequireExistingChange = v
	}
	return p
}

func parsePasswordGeneratorPolicy(data domain.PolicyData) *PasswordGeneratorPolicy {
	p := &PasswordGeneratorPolicy{}
	if v, ok := data["type"].(string); ok {
		p.DefaultType = v
	}
	if v, ok := data["min_length"].(float64); ok {
		p.MinLength = int(v)
	}
	if v, ok := data["require_uppercase"].(bool); ok {
		p.RequireUppercase = v
	}
	if v, ok := data["require_lowercase"].(bool); ok {
		p.RequireLowercase = v
	}
	if v, ok := data["require_numbers"].(bool); ok {
		p.RequireNumbers = v
	}
	if v, ok := data["require_special"].(bool); ok {
		p.RequireSpecial = v
	}
	if v, ok := data["min_special_count"].(float64); ok {
		p.MinSpecialCount = int(v)
	}
	if v, ok := data["min_number_count"].(float64); ok {
		p.MinNumberCount = int(v)
	}
	if v, ok := data["passphrase_min_words"].(float64); ok {
		p.PassphraseMinWords = int(v)
	}
	if v, ok := data["passphrase_capitalize"].(bool); ok {
		p.PassphraseCapitalize = v
	}
	if v, ok := data["passphrase_include_number"].(bool); ok {
		p.PassphraseIncludeNumber = v
	}
	return p
}

func parseSessionTimeoutPolicy(data domain.PolicyData) *SessionTimeoutPolicy {
	p := &SessionTimeoutPolicy{}
	if v, ok := data["max_timeout_minutes"].(float64); ok {
		p.MaxTimeoutMinutes = int(v)
	}
	if v, ok := data["timeout_action"].(string); ok {
		p.TimeoutAction = v
	}
	return p
}
