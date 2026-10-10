package domain

import "sort"

// EffectivePolicy is a policy that applies to the current user, merged across
// every organization that enforces it. Clients enforce it as-is.
type EffectivePolicy struct {
	Type          PolicyType           `json:"type"`
	Data          PolicyData           `json:"data"`
	Organizations []EffectivePolicyOrg `json:"organizations"`
}

// EffectivePolicyOrg names an organization that enforces a policy.
type EffectivePolicyOrg struct {
	ID       uint   `json:"id"`
	PublicID string `json:"public_id"`
	Name     string `json:"name"`
}

// EffectivePoliciesResponse is returned by GET /api/policies/effective.
type EffectivePoliciesResponse struct {
	Policies []EffectivePolicy `json:"policies"`
}

// clientEnforcedPolicies are the policies clients enforce. The value tells
// whether owners and admins of the enforcing organization are exempt (the
// same rule the server applies to Send and personal vault items).
var clientEnforcedPolicies = map[PolicyType]bool{
	PolicyMasterPWRequirements:   false,
	PolicySessionTimeout:         false,
	PolicyRemovePINUnlock:        false,
	PolicyPasswordGenerator:      false,
	PolicyRemoveCardType:         false,
	PolicyActivateAutofill:       false,
	PolicyRequireAutofillConfirm: false,
	PolicyDefaultURIMatch:        false,
	PolicyDisablePersonalExport:  true,
	PolicyRemoveSend:             true,
	PolicyDisablePersonalVault:   true,
}

// PolicyAppliesToMember reports whether a client-enforced policy binds a
// member with the given role. Unknown or server-only policies return false.
func PolicyAppliesToMember(t PolicyType, role OrganizationRole) bool {
	adminExempt, ok := clientEnforcedPolicies[t]
	if !ok {
		return false
	}
	if adminExempt && (role == OrgRoleOwner || role == OrgRoleAdmin) {
		return false
	}
	return true
}

// MergePolicyData combines one policy's configuration from several
// organizations into the strictest version. datas must be in a stable order
// (organization ID); the first wins where values cannot be combined.
func MergePolicyData(t PolicyType, datas []PolicyData) PolicyData {
	merged := PolicyData{}
	switch t {
	case PolicyMasterPWRequirements:
		mergeMax(merged, datas, "min_length", "min_special_count", "min_complexity")
		mergeAny(merged, datas, "require_uppercase", "require_lowercase", "require_numbers", "require_special", "require_existing_change")
	case PolicyPasswordGenerator:
		mergeMax(merged, datas, "min_length", "min_special_count", "min_number_count", "passphrase_min_words")
		mergeAny(merged, datas, "require_uppercase", "require_lowercase", "require_numbers", "require_special", "passphrase_capitalize", "passphrase_include_number")
		mergeFirst(merged, datas, "type")
	case PolicySessionTimeout:
		mergeMin(merged, datas, "max_timeout_minutes")
		for _, d := range datas {
			if d["timeout_action"] == "logOut" {
				merged["timeout_action"] = "logOut"
			}
		}
		if _, ok := merged["timeout_action"]; !ok {
			mergeFirst(merged, datas, "timeout_action")
		}
	case PolicyDefaultURIMatch:
		mergeFirst(merged, datas, "match_type")
	}
	return merged
}

func numberValue(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

func mergeMax(dst PolicyData, datas []PolicyData, keys ...string) {
	for _, key := range keys {
		for _, d := range datas {
			if v, ok := numberValue(d[key]); ok {
				if cur, set := numberValue(dst[key]); !set || v > cur {
					dst[key] = v
				}
			}
		}
	}
}

func mergeMin(dst PolicyData, datas []PolicyData, keys ...string) {
	for _, key := range keys {
		for _, d := range datas {
			if v, ok := numberValue(d[key]); ok {
				if cur, set := numberValue(dst[key]); !set || v < cur {
					dst[key] = v
				}
			}
		}
	}
}

func mergeAny(dst PolicyData, datas []PolicyData, keys ...string) {
	for _, key := range keys {
		for _, d := range datas {
			if b, ok := d[key].(bool); ok {
				dst[key] = b || dst[key] == true
			}
		}
	}
}

func mergeFirst(dst PolicyData, datas []PolicyData, keys ...string) {
	for _, key := range keys {
		for _, d := range datas {
			if v, ok := d[key]; ok && v != nil && v != "" {
				dst[key] = v
				break
			}
		}
	}
}

// SortEffectivePolicies orders policies by type for stable responses.
func SortEffectivePolicies(policies []EffectivePolicy) {
	sort.Slice(policies, func(i, j int) bool { return policies[i].Type < policies[j].Type })
}
