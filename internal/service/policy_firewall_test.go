package service

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func firewallWith(rules ...map[string]interface{}) PolicyFirewallService {
	raw := make([]interface{}, len(rules))
	for i, r := range rules {
		raw[i] = r
	}
	return NewPolicyFirewallService(stubPolicyData{data: domain.PolicyData{"rules": raw}})
}

func TestFirewallRules(t *testing.T) {
	ctx := context.Background()
	fw := firewallWith(
		map[string]interface{}{"type": "ip", "value": "203.0.113.9", "action": "deny"},
		map[string]interface{}{"type": "cidr", "value": "203.0.113.0/24", "action": "allow"},
	)
	res, err := fw.CheckAccess(ctx, 1, "203.0.113.9")
	require.NoError(t, err)
	assert.False(t, res.Allowed, "first matching rule wins")
	assert.NotContains(t, res.Reason, "203.0.113.9", "the rule is not revealed to the client")

	res, _ = fw.CheckAccess(ctx, 1, "203.0.113.10")
	assert.True(t, res.Allowed)
	res, _ = fw.CheckAccess(ctx, 1, "198.51.100.1")
	assert.False(t, res.Allowed, "allow lists deny everything else")
}

func TestFirewallIgnoresUnsupportedRulesForImplicitDeny(t *testing.T) {
	fw := firewallWith(map[string]interface{}{"type": "country", "value": "TR", "action": "allow"})
	res, err := fw.CheckAccess(context.Background(), 1, "198.51.100.1")
	require.NoError(t, err)
	assert.True(t, res.Allowed, "a country rule cannot match, so it must not deny everyone")
}

func TestFirewallFailsClosed(t *testing.T) {
	fw := NewPolicyFirewallService(stubPolicyData{err: errors.New("db down")})
	_, err := fw.CheckAccess(context.Background(), 1, "198.51.100.1")
	assert.Error(t, err)
}
