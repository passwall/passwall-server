package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── TierMeetsMinimum Tests ─────────────────────────────────────────────────────

func TestTierMeetsMinimum_PlanHierarchy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		plan     OrganizationPlan
		tier     PolicyTier
		expected bool
	}{
		// Free plan cannot access any policy tier
		{"free < team", PlanFree, PolicyTierTeam, false},
		{"free < business", PlanFree, PolicyTierBusiness, false},
		{"free < enterprise", PlanFree, PolicyTierEnterprise, false},

		// Pro plan (personal) cannot access any policy tier
		{"pro < team", PlanPro, PolicyTierTeam, false},
		{"pro < business", PlanPro, PolicyTierBusiness, false},
		{"pro < enterprise", PlanPro, PolicyTierEnterprise, false},

		// Family plan has team-level access
		{"family >= team", PlanFamily, PolicyTierTeam, true},
		{"family < business", PlanFamily, PolicyTierBusiness, false},
		{"family < enterprise", PlanFamily, PolicyTierEnterprise, false},

		// Team plan has team-level access
		{"team >= team", PlanTeam, PolicyTierTeam, true},
		{"team < business", PlanTeam, PolicyTierBusiness, false},
		{"team < enterprise", PlanTeam, PolicyTierEnterprise, false},

		// Business plan has business-level access
		{"business >= team", PlanBusiness, PolicyTierTeam, true},
		{"business >= business", PlanBusiness, PolicyTierBusiness, true},
		{"business < enterprise", PlanBusiness, PolicyTierEnterprise, false},

		// Enterprise plan has all access
		{"enterprise >= team", PlanEnterprise, PolicyTierTeam, true},
		{"enterprise >= business", PlanEnterprise, PolicyTierBusiness, true},
		{"enterprise >= enterprise", PlanEnterprise, PolicyTierEnterprise, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, TierMeetsMinimum(tt.plan, tt.tier))
		})
	}
}

func TestTierMeetsMinimum_UnknownPlanDefaultsToZero(t *testing.T) {
	t.Parallel()
	assert.False(t, TierMeetsMinimum(OrganizationPlan("unknown-plan"), PolicyTierTeam),
		"unknown plan should not meet any tier requirement")
}

// ─── IsValidPolicyType Tests ────────────────────────────────────────────────────

func TestIsValidPolicyType(t *testing.T) {
	t.Parallel()

	validTypes := []PolicyType{
		PolicyRequireTwoFactor, PolicyRequireSSO, PolicyMasterPWRequirements,
		PolicySessionTimeout, PolicyRemovePINUnlock, PolicyFailedLoginLimit,
		PolicyAccountRecovery, PolicySingleOrganization, PolicyDisablePersonalExport,
		PolicyEnforceDataOwnership, PolicyRemoveCardType, PolicyDisableExternalSharing,
		PolicyPasswordGenerator, PolicyActivateAutofill, PolicyDefaultURIMatch,
		PolicyRequireAutofillConfirm, PolicyRequireBrowserExtension, PolicyFirewallRules,
		PolicyBlockDomainAccountCreation, PolicySendOptions, PolicyRemoveSend,
	}

	for _, pt := range validTypes {
		t.Run(string(pt), func(t *testing.T) {
			t.Parallel()
			assert.True(t, IsValidPolicyType(pt), "expected %s to be valid", pt)
		})
	}

	invalidTypes := []PolicyType{
		"", "nonexistent", "REQUIRE_TWO_FACTOR", "require-two-factor",
		"admin_override", "god_mode",
	}

	for _, pt := range invalidTypes {
		name := string(pt)
		if name == "" {
			name = "empty"
		}
		t.Run("invalid_"+name, func(t *testing.T) {
			t.Parallel()
			assert.False(t, IsValidPolicyType(pt), "expected %s to be invalid", pt)
		})
	}
}

// ─── GetPolicyTier Tests ────────────────────────────────────────────────────────

func TestGetPolicyTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		policyType   PolicyType
		expectedTier PolicyTier
	}{
		{PolicyRequireTwoFactor, PolicyTierTeam},
		{PolicyRequireSSO, PolicyTierEnterprise},
		{PolicyMasterPWRequirements, PolicyTierTeam},
		{PolicySessionTimeout, PolicyTierBusiness},
		{PolicyFailedLoginLimit, PolicyTierEnterprise},
		{PolicySingleOrganization, PolicyTierBusiness},
		{PolicyFirewallRules, PolicyTierEnterprise},
		{PolicyDisableExternalSharing, PolicyTierTeam},
		{PolicyPasswordGenerator, PolicyTierTeam},
	}

	for _, tt := range tests {
		t.Run(string(tt.policyType), func(t *testing.T) {
			t.Parallel()
			tier, ok := GetPolicyTier(tt.policyType)
			require.True(t, ok)
			assert.Equal(t, tt.expectedTier, tier)
		})
	}

	t.Run("unknown_type_returns_false", func(t *testing.T) {
		t.Parallel()
		_, ok := GetPolicyTier("nonexistent_policy")
		assert.False(t, ok)
	})
}

// ─── GetPolicyDependencies Tests ────────────────────────────────────────────────

func TestGetPolicyDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		policyType PolicyType
		expected   []PolicyType
	}{
		{PolicyRequireSSO, []PolicyType{PolicySingleOrganization}},
		{PolicySessionTimeout, []PolicyType{PolicySingleOrganization}},
		{PolicyAccountRecovery, []PolicyType{PolicySingleOrganization}},
		{PolicyDefaultURIMatch, []PolicyType{PolicySingleOrganization}},

		// Policies with no dependencies
		{PolicyRequireTwoFactor, nil},
		{PolicySingleOrganization, nil},
		{PolicyPasswordGenerator, nil},
		{PolicyFirewallRules, nil},
		{PolicyDisableExternalSharing, nil},
	}

	for _, tt := range tests {
		t.Run(string(tt.policyType), func(t *testing.T) {
			t.Parallel()
			deps := GetPolicyDependencies(tt.policyType)
			if tt.expected == nil {
				assert.Nil(t, deps)
			} else {
				assert.Equal(t, tt.expected, deps)
			}
		})
	}
}

// ─── AllPolicyDefinitions Consistency Tests ─────────────────────────────────────

func TestAllPolicyDefinitions_NoDuplicateTypes(t *testing.T) {
	t.Parallel()
	defs := AllPolicyDefinitions()
	seen := make(map[PolicyType]bool)
	for _, def := range defs {
		assert.False(t, seen[def.Type], "duplicate policy type: %s", def.Type)
		seen[def.Type] = true
	}
}

func TestAllPolicyDefinitions_AllFieldsPopulated(t *testing.T) {
	t.Parallel()
	defs := AllPolicyDefinitions()
	for _, def := range defs {
		t.Run(string(def.Type), func(t *testing.T) {
			t.Parallel()
			assert.NotEmpty(t, def.Type, "type must not be empty")
			assert.NotEmpty(t, def.Name, "name must not be empty for %s", def.Type)
			assert.NotEmpty(t, def.Description, "description must not be empty for %s", def.Type)
			assert.NotEmpty(t, def.Category, "category must not be empty for %s", def.Type)
			assert.NotEmpty(t, def.Tier, "tier must not be empty for %s", def.Type)

			validTiers := map[PolicyTier]bool{
				PolicyTierTeam: true, PolicyTierBusiness: true, PolicyTierEnterprise: true,
			}
			assert.True(t, validTiers[def.Tier], "invalid tier %s for %s", def.Tier, def.Type)
		})
	}
}

func TestAllPolicyDefinitions_DependenciesReferenceValidPolicies(t *testing.T) {
	t.Parallel()
	defs := AllPolicyDefinitions()
	validTypes := make(map[PolicyType]bool)
	for _, def := range defs {
		validTypes[def.Type] = true
	}

	for _, def := range defs {
		for _, dep := range def.Dependencies {
			assert.True(t, validTypes[dep],
				"policy %s depends on unknown policy %s", def.Type, dep)
		}
	}
}

func TestAllPolicyDefinitions_NoCyclicDependencies(t *testing.T) {
	t.Parallel()
	defs := AllPolicyDefinitions()
	depMap := make(map[PolicyType][]PolicyType)
	for _, def := range defs {
		if len(def.Dependencies) > 0 {
			depMap[def.Type] = def.Dependencies
		}
	}

	for _, def := range defs {
		visited := make(map[PolicyType]bool)
		queue := []PolicyType{def.Type}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, dep := range depMap[current] {
				assert.NotEqual(t, def.Type, dep,
					"cyclic dependency detected: %s -> ... -> %s", def.Type, dep)
				if !visited[dep] {
					visited[dep] = true
					queue = append(queue, dep)
				}
			}
		}
	}
}

func TestAllPolicyDefinitions_DependencyTierNotHigherThanDependent(t *testing.T) {
	t.Parallel()
	defs := AllPolicyDefinitions()
	tierRank := map[PolicyTier]int{
		PolicyTierTeam: 1, PolicyTierBusiness: 2, PolicyTierEnterprise: 3,
	}
	defMap := make(map[PolicyType]PolicyDefinition)
	for _, def := range defs {
		defMap[def.Type] = def
	}

	for _, def := range defs {
		for _, dep := range def.Dependencies {
			depDef := defMap[dep]
			assert.LessOrEqual(t, tierRank[depDef.Tier], tierRank[def.Tier],
				"dependency %s (tier=%s) requires a higher plan than %s (tier=%s); "+
					"users can never satisfy the dependency",
				dep, depDef.Tier, def.Type, def.Tier)
		}
	}
}

// ─── PolicyData Tests ───────────────────────────────────────────────────────────

func TestPolicyData_ScanNil(t *testing.T) {
	t.Parallel()
	var d PolicyData
	err := d.Scan(nil)
	require.NoError(t, err)
	assert.NotNil(t, d)
	assert.Empty(t, d)
}

func TestPolicyData_ScanValidJSON(t *testing.T) {
	t.Parallel()
	var d PolicyData
	err := d.Scan([]byte(`{"min_length": 12, "require_uppercase": true}`))
	require.NoError(t, err)
	assert.Equal(t, float64(12), d["min_length"])
	assert.Equal(t, true, d["require_uppercase"])
}

func TestPolicyData_ScanInvalidType(t *testing.T) {
	t.Parallel()
	var d PolicyData
	err := d.Scan(42)
	assert.Error(t, err)
}

func TestPolicyData_ScanInvalidJSON(t *testing.T) {
	t.Parallel()
	var d PolicyData
	err := d.Scan([]byte(`{not valid json}`))
	assert.Error(t, err)
}

func TestPolicyData_ValueNil(t *testing.T) {
	t.Parallel()
	var d PolicyData
	v, err := d.Value()
	require.NoError(t, err)
	assert.Equal(t, []byte(`{}`), v)
}

func TestPolicyData_ValueWithData(t *testing.T) {
	t.Parallel()
	d := PolicyData{"key": "value"}
	v, err := d.Value()
	require.NoError(t, err)
	assert.Contains(t, string(v.([]byte)), `"key":"value"`)
}

// ─── ToOrganizationPolicyDTO Tests ──────────────────────────────────────────────

func TestToOrganizationPolicyDTO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ToOrganizationPolicyDTO(nil))
}

func TestToOrganizationPolicyDTO_FullConversion(t *testing.T) {
	t.Parallel()
	userID := uint(42)
	policy := &OrganizationPolicy{
		ID:                  1,
		OrganizationID:      10,
		Type:                PolicyRequireTwoFactor,
		Enabled:             true,
		Data:                PolicyData{"key": "val"},
		LastEnabledByUserID: &userID,
	}

	dto := ToOrganizationPolicyDTO(policy)
	require.NotNil(t, dto)
	assert.Equal(t, policy.ID, dto.ID)
	assert.Equal(t, policy.OrganizationID, dto.OrganizationID)
	assert.Equal(t, policy.Type, dto.Type)
	assert.Equal(t, policy.Enabled, dto.Enabled)
	assert.Equal(t, policy.Data, dto.Data)
}
