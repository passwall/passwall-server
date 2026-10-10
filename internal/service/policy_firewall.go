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
	// ResolveOrganization returns the organization that owns a resource
	// addressed by ID (routes like /org-items/:id), so its firewall applies.
	ResolveOrganization(ctx context.Context, kind FirewallResource, id uint) (uint, error)
}

// FirewallResource names resources reached by ID outside /organizations/:id.
type FirewallResource string

const (
	FirewallResourceItem       FirewallResource = "item"
	FirewallResourceCollection FirewallResource = "collection"
	FirewallResourceTeam       FirewallResource = "team"
)

// FirewallOrgLookup returns the organization of a resource.
type FirewallOrgLookup func(ctx context.Context, id uint) (uint, error)

type policyFirewallService struct {
	policyService OrganizationPolicyService
	lookups       map[FirewallResource]FirewallOrgLookup
}

// NewPolicyFirewallService creates a new firewall enforcement service.
// lookups resolve the organization of resources addressed by ID.
func NewPolicyFirewallService(policyService OrganizationPolicyService, lookups ...map[FirewallResource]FirewallOrgLookup) PolicyFirewallService {
	svc := &policyFirewallService{policyService: policyService, lookups: map[FirewallResource]FirewallOrgLookup{}}
	if len(lookups) > 0 {
		svc.lookups = lookups[0]
	}
	return svc
}

func (s *policyFirewallService) ResolveOrganization(ctx context.Context, kind FirewallResource, id uint) (uint, error) {
	lookup, ok := s.lookups[kind]
	if !ok {
		return 0, fmt.Errorf("no organization lookup for %s", kind)
	}
	return lookup(ctx, id)
}

func (s *policyFirewallService) CheckAccess(ctx context.Context, orgID uint, clientIP string) (*FirewallCheckResult, error) {
	data, err := s.policyService.GetPolicyData(ctx, orgID, domain.PolicyFirewallRules)
	if err != nil {
		// Fail closed: the caller answers with an error instead of letting the
		// request through unchecked.
		return nil, fmt.Errorf("failed to load firewall policy: %w", err)
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
					Reason:      "access denied by organization firewall rule",
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
	// Only rules that can match count: unsupported types (e.g. country, which
	// needs a GeoIP database) must not turn into an implicit deny-all.
	hasAllowRules := false
	for _, rule := range rules {
		if rule.Action == FirewallActionAllow && (rule.Type == "ip" || rule.Type == "cidr") {
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
