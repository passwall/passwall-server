package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergePolicyDataTakesStrictest(t *testing.T) {
	mp := MergePolicyData(PolicyMasterPWRequirements, []PolicyData{
		{"min_length": float64(12), "require_uppercase": true, "require_numbers": false},
		{"min_length": float64(16), "require_numbers": true, "min_complexity": float64(3)},
	})
	assert.Equal(t, float64(16), mp["min_length"])
	assert.Equal(t, true, mp["require_uppercase"])
	assert.Equal(t, true, mp["require_numbers"])
	assert.Equal(t, float64(3), mp["min_complexity"])

	st := MergePolicyData(PolicySessionTimeout, []PolicyData{
		{"max_timeout_minutes": float64(60), "timeout_action": "lock"},
		{"max_timeout_minutes": float64(15), "timeout_action": "logOut"},
	})
	assert.Equal(t, float64(15), st["max_timeout_minutes"])
	assert.Equal(t, "logOut", st["timeout_action"])

	uri := MergePolicyData(PolicyDefaultURIMatch, []PolicyData{{"match_type": "host"}, {"match_type": "exact"}})
	assert.Equal(t, "host", uri["match_type"], "first organization wins")
}

func TestPolicyAppliesToMember(t *testing.T) {
	assert.True(t, PolicyAppliesToMember(PolicyRemoveSend, OrgRoleMember))
	assert.False(t, PolicyAppliesToMember(PolicyRemoveSend, OrgRoleAdmin))
	assert.False(t, PolicyAppliesToMember(PolicyDisablePersonalExport, OrgRoleOwner))
	assert.True(t, PolicyAppliesToMember(PolicySessionTimeout, OrgRoleOwner), "session timeout binds everyone")
	assert.False(t, PolicyAppliesToMember(PolicyFirewallRules, OrgRoleMember), "server-only policies are not listed")
	assert.False(t, PolicyAppliesToMember(PolicyRequireSSO, OrgRoleMember))
}
