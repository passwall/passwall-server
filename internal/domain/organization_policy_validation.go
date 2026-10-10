package domain

import (
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
)

// ErrPolicyNotAvailable is returned when enabling a policy that no Passwall
// client or server enforces yet. Disabling it stays possible.
var ErrPolicyNotAvailable = errors.New("this policy is not available yet")

// unavailablePolicies are listed in the catalog but cannot be enabled
// because nothing enforces them yet. Showing them as working settings would
// give administrators a false sense of control.
var unavailablePolicies = map[PolicyType]bool{
	// Needs SSO sign-in in every client (extension, mobile, desktop) first,
	// otherwise members would be locked out of them.
	PolicyRequireSSO:                 true,
	PolicyAccountRecovery:            true,
	PolicyRequireDeviceApproval:      true,
	PolicyBlockDomainAccountCreation: true,
	PolicyRequireBrowserExtension:    true,
	PolicyPasswordExpiration:         true,
	PolicySendOptions:                true,
	PolicyEnforceDataOwnership:       true,
	PolicyDisableExternalSharing:     true,
}

// IsPolicyAvailable reports whether a known policy is enforced and may be
// enabled. Callers check IsValidPolicyType separately.
func IsPolicyAvailable(t PolicyType) bool {
	return !unavailablePolicies[t]
}

// PolicyValidationError describes an invalid policy configuration.
type PolicyValidationError struct {
	Field   string
	Message string
}

func (e *PolicyValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type fieldRule func(value interface{}) string

func intRange(min, max int) fieldRule {
	return func(value interface{}) string {
		f, ok := value.(float64)
		if !ok || f != math.Trunc(f) {
			return "must be a whole number"
		}
		if f < float64(min) || f > float64(max) {
			return fmt.Sprintf("must be between %d and %d", min, max)
		}
		return ""
	}
}

func boolean(value interface{}) string {
	if _, ok := value.(bool); !ok {
		return "must be true or false"
	}
	return ""
}

func oneOf(options ...string) fieldRule {
	return func(value interface{}) string {
		s, ok := value.(string)
		if ok {
			for _, option := range options {
				if s == option {
					return ""
				}
			}
		}
		return fmt.Sprintf("must be one of %v", options)
	}
}

// policyDataRules lists the accepted configuration fields per policy. A
// policy that is absent here takes no configuration.
var policyDataRules = map[PolicyType]map[string]fieldRule{
	PolicyRequireTwoFactor: {
		"grace_period_days": intRange(0, 90),
	},
	PolicyMasterPWRequirements: {
		"min_length":              intRange(8, 128),
		"require_uppercase":       boolean,
		"require_lowercase":       boolean,
		"require_numbers":         boolean,
		"require_special":         boolean,
		"min_special_count":       intRange(0, 10),
		"min_complexity":          intRange(0, 4),
		"require_existing_change": boolean,
	},
	PolicySessionTimeout: {
		"max_timeout_minutes": intRange(1, 43200),
		"timeout_action":      oneOf("lock", "logOut"),
	},
	PolicyFailedLoginLimit: {
		"max_attempts":           intRange(3, 20),
		"window_minutes":         intRange(5, 60),
		"block_duration_minutes": intRange(5, 1440),
	},
	PolicyPasswordGenerator: {
		"type":                      oneOf("password", "passphrase"),
		"min_length":                intRange(5, 128),
		"require_uppercase":         boolean,
		"require_lowercase":         boolean,
		"require_numbers":           boolean,
		"require_special":           boolean,
		"min_special_count":         intRange(0, 9),
		"min_number_count":          intRange(0, 9),
		"passphrase_min_words":      intRange(3, 20),
		"passphrase_capitalize":     boolean,
		"passphrase_include_number": boolean,
	},
	PolicyDefaultURIMatch: {
		"match_type": oneOf("base_domain", "host", "starts_with", "exact", "regular_expression", "never"),
	},
}

// ValidatePolicyData checks a policy configuration and returns a cleaned copy
// that only holds known, well-formed fields. Server-managed fields (such as
// the 2FA policy's enabled_at) are dropped; the service sets them.
func ValidatePolicyData(t PolicyType, data PolicyData) (PolicyData, error) {
	clean := make(PolicyData)
	if t == PolicyFirewallRules {
		return validateFirewallRules(data)
	}
	rules := policyDataRules[t]
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if t == PolicyRequireTwoFactor && key == "enabled_at" {
			continue
		}
		rule, known := rules[key]
		if !known {
			return nil, &PolicyValidationError{Field: key, Message: "is not a setting of this policy"}
		}
		if msg := rule(data[key]); msg != "" {
			return nil, &PolicyValidationError{Field: key, Message: msg}
		}
		clean[key] = data[key]
	}
	return clean, nil
}

func validateFirewallRules(data PolicyData) (PolicyData, error) {
	for key := range data {
		if key != "rules" {
			return nil, &PolicyValidationError{Field: key, Message: "is not a setting of this policy"}
		}
	}
	raw, ok := data["rules"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, &PolicyValidationError{Field: "rules", Message: "at least one rule is required"}
	}
	if len(raw) > 100 {
		return nil, &PolicyValidationError{Field: "rules", Message: "at most 100 rules are allowed"}
	}
	rules := make([]interface{}, 0, len(raw))
	for i, item := range raw {
		field := fmt.Sprintf("rules[%d]", i)
		rule, ok := item.(map[string]interface{})
		if !ok {
			return nil, &PolicyValidationError{Field: field, Message: "must be an object"}
		}
		ruleType, _ := rule["type"].(string)
		value, _ := rule["value"].(string)
		action, _ := rule["action"].(string)
		switch ruleType {
		case "ip":
			if net.ParseIP(value) == nil {
				return nil, &PolicyValidationError{Field: field + ".value", Message: "must be a valid IP address"}
			}
		case "cidr":
			if _, _, err := net.ParseCIDR(value); err != nil {
				return nil, &PolicyValidationError{Field: field + ".value", Message: "must be a valid CIDR range"}
			}
		default:
			return nil, &PolicyValidationError{Field: field + ".type", Message: "must be ip or cidr"}
		}
		if action != "allow" && action != "deny" {
			return nil, &PolicyValidationError{Field: field + ".action", Message: "must be allow or deny"}
		}
		rules = append(rules, map[string]interface{}{"type": ruleType, "value": value, "action": action})
	}
	return PolicyData{"rules": rules}, nil
}
