package service

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/passwall/passwall-server/internal/domain"
)

// FirewallAction determines what happens when a rule matches
type FirewallAction string

const (
	FirewallActionAllow  FirewallAction = "allow"
	FirewallActionDeny   FirewallAction = "deny"
	FirewallActionReport FirewallAction = "report"
)

// FirewallRule represents a single firewall rule
type FirewallRule struct {
	Type   string         `json:"type"`   // "ip", "cidr", "country"
	Value  string         `json:"value"`  // IP address, CIDR range, or country code
	Action FirewallAction `json:"action"` // "allow", "deny", "report"
}

// FirewallCheckResult contains the result of a firewall check
type FirewallCheckResult struct {
	Allowed     bool          `json:"allowed"`
	MatchedRule *FirewallRule `json:"matched_rule,omitempty"`
	Reason      string        `json:"reason,omitempty"`
}

// PolicyFirewallService handles firewall rules enforcement
type PolicyFirewallService interface {
	CheckAccess(ctx context.Context, orgID uint, clientIP string) (*FirewallCheckResult, error)
}

type policyFirewallService struct {
	policyService OrganizationPolicyService
}

// NewPolicyFirewallService creates a new firewall enforcement service
func NewPolicyFirewallService(policyService OrganizationPolicyService) PolicyFirewallService {
	return &policyFirewallService{policyService: policyService}
}

func (s *policyFirewallService) CheckAccess(ctx context.Context, orgID uint, clientIP string) (*FirewallCheckResult, error) {
	data, err := s.policyService.GetPolicyData(ctx, orgID, domain.PolicyFirewallRules)
	if err != nil {
		return &FirewallCheckResult{Allowed: true}, nil
	}
	if data == nil {
		return &FirewallCheckResult{Allowed: true}, nil
	}

	rules := parseFirewallRules(data)
	if len(rules) == 0 {
		return &FirewallCheckResult{Allowed: true}, nil
	}

	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return &FirewallCheckResult{
			Allowed: false,
			Reason:  "could not parse client IP",
		}, nil
	}

	// Evaluate rules in order (first match wins, like a traditional firewall)
	for _, rule := range rules {
		matched := false
		switch rule.Type {
		case "ip":
			ruleIP := net.ParseIP(strings.TrimSpace(rule.Value))
			if ruleIP != nil && ruleIP.Equal(ip) {
				matched = true
			}
		case "cidr":
			_, cidr, err := net.ParseCIDR(strings.TrimSpace(rule.Value))
			if err == nil && cidr.Contains(ip) {
				matched = true
			}
		case "country":
			// Country-based filtering requires a GeoIP database.
			// This is a placeholder for future GeoIP integration.
			// For now, country rules are skipped.
			continue
		}

		if matched {
			switch rule.Action {
			case FirewallActionDeny:
				return &FirewallCheckResult{
					Allowed:     false,
					MatchedRule: &rule,
					Reason:      fmt.Sprintf("access denied by firewall rule: %s %s", rule.Type, rule.Value),
				}, nil
			case FirewallActionAllow:
				return &FirewallCheckResult{
					Allowed:     true,
					MatchedRule: &rule,
				}, nil
			case FirewallActionReport:
				// Log but allow
				return &FirewallCheckResult{
					Allowed:     true,
					MatchedRule: &rule,
					Reason:      "access reported by firewall rule",
				}, nil
			}
		}
	}

	// Default: if rules exist but none matched, check for default deny
	// If there are any "allow" rules, treat unmatched as implicit deny
	hasAllowRules := false
	for _, rule := range rules {
		if rule.Action == FirewallActionAllow {
			hasAllowRules = true
			break
		}
	}

	if hasAllowRules {
		return &FirewallCheckResult{
			Allowed: false,
			Reason:  "IP not in any allow list (implicit deny)",
		}, nil
	}

	// If only deny rules exist, unmatched traffic is allowed
	return &FirewallCheckResult{Allowed: true}, nil
}

func parseFirewallRules(data domain.PolicyData) []FirewallRule {
	rulesRaw, ok := data["rules"]
	if !ok {
		return nil
	}

	rulesSlice, ok := rulesRaw.([]interface{})
	if !ok {
		return nil
	}

	rules := make([]FirewallRule, 0, len(rulesSlice))
	for _, r := range rulesSlice {
		rMap, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		rule := FirewallRule{}
		if t, ok := rMap["type"].(string); ok {
			rule.Type = t
		}
		if v, ok := rMap["value"].(string); ok {
			rule.Value = v
		}
		if a, ok := rMap["action"].(string); ok {
			rule.Action = FirewallAction(a)
		}

		if rule.Type != "" && rule.Action != "" {
			rules = append(rules, rule)
		}
	}

	return rules
}
