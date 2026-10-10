package service

import "context"

// CheckJoinPolicies reports whether organization policies allow userID to
// become a member of orgID.
func (s *organizationService) CheckJoinPolicies(ctx context.Context, orgID, userID uint) error {
	return s.checkSingleOrganizationPolicy(ctx, orgID, userID)
}
