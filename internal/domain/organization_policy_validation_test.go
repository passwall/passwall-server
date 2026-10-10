package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePolicyData(t *testing.T) {
	clean, err := ValidatePolicyData(PolicyMasterPWRequirements, PolicyData{"min_length": float64(14), "require_numbers": true})
	require.NoError(t, err)
	assert.Equal(t, PolicyData{"min_length": float64(14), "require_numbers": true}, clean)

	for _, bad := range []PolicyData{
		{"min_length": float64(4)},
		{"min_length": 12.5},
		{"min_length": "12"},
		{"unknown": true},
	} {
		_, err := ValidatePolicyData(PolicyMasterPWRequirements, bad)
		assert.Error(t, err, "%v", bad)
	}

	_, err = ValidatePolicyData(PolicySessionTimeout, PolicyData{"timeout_action": "explode"})
	assert.Error(t, err)

	// Policies without settings take none.
	_, err = ValidatePolicyData(PolicyRemoveSend, PolicyData{"anything": 1.0})
	assert.Error(t, err)
	clean, err = ValidatePolicyData(PolicyRemoveSend, PolicyData{})
	require.NoError(t, err)
	assert.Empty(t, clean)

	// The 2FA enabled_at is server-managed and silently dropped.
	clean, err = ValidatePolicyData(PolicyRequireTwoFactor, PolicyData{"enabled_at": "2020-01-01T00:00:00Z", "grace_period_days": float64(3)})
	require.NoError(t, err)
	assert.Equal(t, PolicyData{"grace_period_days": float64(3)}, clean)
}

func TestValidateFirewallRules(t *testing.T) {
	ok := PolicyData{"rules": []interface{}{
		map[string]interface{}{"type": "ip", "value": "203.0.113.4", "action": "allow"},
		map[string]interface{}{"type": "cidr", "value": "10.0.0.0/8", "action": "deny", "note": "dropped"},
	}}
	clean, err := ValidatePolicyData(PolicyFirewallRules, ok)
	require.NoError(t, err)
	assert.Len(t, clean["rules"], 2)

	for _, bad := range []PolicyData{
		{},
		{"rules": []interface{}{}},
		{"rules": []interface{}{map[string]interface{}{"type": "ip", "value": "999.1.1.1", "action": "allow"}}},
		{"rules": []interface{}{map[string]interface{}{"type": "cidr", "value": "10.0.0.0", "action": "allow"}}},
		{"rules": []interface{}{map[string]interface{}{"type": "country", "value": "TR", "action": "allow"}}},
		{"rules": []interface{}{map[string]interface{}{"type": "ip", "value": "1.1.1.1", "action": "report"}}},
	} {
		_, err := ValidatePolicyData(PolicyFirewallRules, bad)
		assert.Error(t, err, "%v", bad)
	}
}

func TestUnavailablePoliciesAreMarked(t *testing.T) {
	for _, def := range AllPolicyDefinitions() {
		assert.Equal(t, IsPolicyAvailable(def.Type), def.Available, def.Type)
	}
	assert.False(t, IsPolicyAvailable(PolicyAccountRecovery))
	assert.True(t, IsPolicyAvailable(PolicyRequireTwoFactor))
}
