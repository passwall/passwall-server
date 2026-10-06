package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
)

var ErrAccountFrozen = errors.New("account is frozen")

type EntitlementDeniedError struct {
	Capability domain.Capability
	Reason     domain.EntitlementReason
	Cause      error
}

func (e *EntitlementDeniedError) Error() string {
	return fmt.Sprintf("%s: %s", e.Cause.Error(), e.Capability)
}

func (e *EntitlementDeniedError) Unwrap() error {
	return e.Cause
}

type OrganizationEntitlementService interface {
	Resolve(ctx context.Context, orgID uint) (*domain.EntitlementSnapshot, error)
	Authorize(ctx context.Context, orgID uint, capability domain.Capability) error
}

type entitlementSubscriptionReader interface {
	GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
}

type entitlementPlanReader interface {
	GetByCode(ctx context.Context, code string) (*domain.Plan, error)
}

type entitlementUsageReader interface {
	GetByID(ctx context.Context, orgID uint) (*domain.Organization, error)
	GetMemberCount(ctx context.Context, orgID uint) (int, error)
	GetCollectionCount(ctx context.Context, orgID uint) (int, error)
}

type entitlementItemCounter interface {
	CountByOrganizationID(ctx context.Context, orgID uint) (int, error)
}

type organizationEntitlementService struct {
	subscriptions     entitlementSubscriptionReader
	plans             entitlementPlanReader
	usage             entitlementUsageReader
	items             entitlementItemCounter
	overrides         repository.OrganizationEntitlementOverrideRepository
	manageBillingBase string
	now               func() time.Time
	enforcementModes  map[domain.Capability]domain.EnforcementMode
	defaultMode       domain.EnforcementMode
	defaultModeSet    bool
	logger            Logger
}

// legacyEnforcedCapabilities were enforced before the central entitlement
// contract existed. They stay enforced unless an operator explicitly
// configures a different mode, so rolling out this service never relaxes
// limits that production already applies.
var legacyEnforcedCapabilities = map[domain.Capability]struct{}{
	domain.CapabilityItemCreate:               {},
	domain.CapabilityItemUpdate:               {},
	domain.CapabilityItemDelete:               {},
	domain.CapabilityBreachMonitoringRead:     {},
	domain.CapabilityPoliciesManage:           {},
	domain.CapabilityBusinessPoliciesManage:   {},
	domain.CapabilityEnterprisePoliciesManage: {},
}

// ErrEntitlementSubscriptionUnavailable means the organization has no
// effective subscription row, which violates the subscription invariant.
var ErrEntitlementSubscriptionUnavailable = errors.New("organization subscription unavailable")

type OrganizationEntitlementOption func(*organizationEntitlementService)

func WithEntitlementEnforcementModes(raw string, logger Logger) OrganizationEntitlementOption {
	return func(service *organizationEntitlementService) {
		service.logger = logger
		for _, entry := range strings.Split(raw, ",") {
			key, value, found := strings.Cut(strings.TrimSpace(entry), "=")
			if !found {
				continue
			}
			mode := domain.EnforcementMode(strings.TrimSpace(value))
			if mode != domain.EnforcementModeOff &&
				mode != domain.EnforcementModeLog &&
				mode != domain.EnforcementModeEnforce {
				continue
			}
			key = strings.TrimSpace(key)
			if key == "*" {
				service.defaultMode = mode
				service.defaultModeSet = true
				continue
			}
			capability := domain.Capability(key)
			if _, exists := domain.CapabilityRegistry()[capability]; exists {
				service.enforcementModes[capability] = mode
			}
		}
	}
}

func NewOrganizationEntitlementService(
	subscriptions entitlementSubscriptionReader,
	plans entitlementPlanReader,
	usage entitlementUsageReader,
	items entitlementItemCounter,
	overrides repository.OrganizationEntitlementOverrideRepository,
	manageBillingBase string,
	options ...OrganizationEntitlementOption,
) OrganizationEntitlementService {
	service := &organizationEntitlementService{
		subscriptions:     subscriptions,
		plans:             plans,
		usage:             usage,
		items:             items,
		overrides:         overrides,
		manageBillingBase: strings.TrimRight(manageBillingBase, "/"),
		now:               time.Now,
		enforcementModes:  make(map[domain.Capability]domain.EnforcementMode),
		defaultMode:       domain.EnforcementModeOff,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

// Resolve returns the snapshot clients consume. Capability decisions reflect
// what the server will actually accept under the current enforcement modes.
func (s *organizationEntitlementService) Resolve(
	ctx context.Context,
	orgID uint,
) (*domain.EntitlementSnapshot, error) {
	snapshot, err := s.resolvePlanSnapshot(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for capability, decision := range snapshot.Capabilities {
		if decision.Allowed || s.modeFor(capability) == domain.EnforcementModeEnforce {
			continue
		}
		snapshot.Capabilities[capability] = domain.CapabilityDecision{
			Allowed:        true,
			Reason:         domain.EntitlementReasonAllowed,
			AdvisoryReason: decision.Reason,
		}
	}
	return snapshot, nil
}

func (s *organizationEntitlementService) modeFor(capability domain.Capability) domain.EnforcementMode {
	if configured, exists := s.enforcementModes[capability]; exists {
		return configured
	}
	if s.defaultModeSet {
		return s.defaultMode
	}
	if _, legacy := legacyEnforcedCapabilities[capability]; legacy {
		return domain.EnforcementModeEnforce
	}
	return s.defaultMode
}

// resolvePlanSnapshot computes the plan decision for every capability without
// applying enforcement modes.
func (s *organizationEntitlementService) resolvePlanSnapshot(
	ctx context.Context,
	orgID uint,
) (*domain.EntitlementSnapshot, error) {
	subscription, err := s.subscriptions.GetByOrganizationID(ctx, orgID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEntitlementSubscriptionUnavailable
		}
		return nil, fmt.Errorf("load subscription: %w", err)
	}
	if subscription == nil || subscription.Plan == nil {
		return nil, ErrEntitlementSubscriptionUnavailable
	}

	now := s.now()
	plan := subscription.Plan
	accessState := resolveAccessState(subscription, plan, now)
	if accessState == domain.AccessStateFree &&
		!plan.IsFree() &&
		plan.ExpiryBehavior == domain.ExpiryBehaviorDowngradeToFree {
		plan, err = s.plans.GetByCode(ctx, "free-monthly")
		if err != nil || plan == nil {
			return nil, fmt.Errorf("load free downgrade plan: %w", err)
		}
	}

	usage, err := s.resolveUsage(ctx, orgID)
	if err != nil {
		return nil, err
	}
	organization, err := s.usage.GetByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("load organization: %w", err)
	}
	limits := domain.EntitlementLimits{
		MaxUsers:       effectiveUserLimit(subscription, plan),
		MaxCollections: cloneInt(plan.MaxCollections),
		MaxItems:       cloneInt(plan.MaxItems),
		MaxDevices:     cloneInt(plan.MaxDevices),
	}
	features := plan.Features

	var activeOverrides []*domain.OrganizationEntitlementOverride
	if s.overrides != nil {
		activeOverrides, err = s.overrides.ListActiveByOrganization(ctx, orgID, now)
		if err != nil {
			return nil, fmt.Errorf("load entitlement overrides: %w", err)
		}
		applyEntitlementOverrides(activeOverrides, &features, &limits)
	}

	if accessState == domain.AccessStateFree &&
		(isOverLimit(usage.Items, limits.MaxItems) ||
			isOverLimit(usage.Collections, limits.MaxCollections) ||
			isOverLimit(usage.Users, limits.MaxUsers)) {
		accessState = domain.AccessStateFreeOverQuota
	}

	snapshot := &domain.EntitlementSnapshot{
		SchemaVersion:     1,
		OrganizationID:    orgID,
		EffectivePlan:     plan.Code,
		SubscriptionState: subscription.State,
		AccessState:       accessState,
		Features:          features,
		Limits:            limits,
		Usage:             usage,
		PeriodEnd:         subscription.RenewAt,
		GraceUntil:        subscription.GracePeriodEndsAt,
		ManageBillingURL:  s.manageBillingURL(organization.PublicID),
	}
	snapshot.Capabilities = resolveCapabilities(snapshot)
	applyCapabilityOverrides(activeOverrides, snapshot.Capabilities)
	return snapshot, nil
}

func (s *organizationEntitlementService) Authorize(
	ctx context.Context,
	orgID uint,
	capability domain.Capability,
) error {
	snapshot, err := s.resolvePlanSnapshot(ctx, orgID)
	if err != nil {
		return err
	}
	decision, ok := snapshot.Capabilities[capability]
	if !ok {
		return &EntitlementDeniedError{
			Capability: capability,
			Reason:     domain.EntitlementReasonFeatureUnavailable,
			Cause:      ErrFeatureNotAvailable,
		}
	}
	if decision.Allowed {
		return nil
	}
	mode := s.modeFor(capability)
	if mode == domain.EnforcementModeOff {
		return nil
	}
	if mode == domain.EnforcementModeLog {
		if s.logger != nil {
			s.logger.Warn(
				"entitlement denial observed",
				"organization_id", orgID,
				"capability", capability,
				"reason", decision.Reason,
			)
		}
		return nil
	}

	var cause error
	switch decision.Reason {
	case domain.EntitlementReasonAccountFrozen:
		cause = ErrAccountFrozen
	case domain.EntitlementReasonPlanLimitReached:
		cause = ErrPlanLimitReached
	case domain.EntitlementReasonSubscriptionExpired:
		cause = ErrSubscriptionExpired
	default:
		cause = ErrFeatureNotAvailable
	}
	return &EntitlementDeniedError{Capability: capability, Reason: decision.Reason, Cause: cause}
}

func (s *organizationEntitlementService) resolveUsage(
	ctx context.Context,
	orgID uint,
) (domain.EntitlementUsage, error) {
	users, err := s.usage.GetMemberCount(ctx, orgID)
	if err != nil {
		return domain.EntitlementUsage{}, fmt.Errorf("load member usage: %w", err)
	}
	collections, err := s.usage.GetCollectionCount(ctx, orgID)
	if err != nil {
		return domain.EntitlementUsage{}, fmt.Errorf("load collection usage: %w", err)
	}
	items, err := s.items.CountByOrganizationID(ctx, orgID)
	if err != nil {
		return domain.EntitlementUsage{}, fmt.Errorf("load item usage: %w", err)
	}
	return domain.EntitlementUsage{Users: users, Collections: collections, Items: items}, nil
}

func (s *organizationEntitlementService) manageBillingURL(orgRef string) string {
	if s.manageBillingBase == "" {
		return ""
	}
	if strings.TrimSpace(orgRef) == "" {
		return ""
	}
	return s.manageBillingBase + "/organizations/" + orgRef + "/billing"
}

func resolveAccessState(
	subscription *domain.Subscription,
	plan *domain.Plan,
	now time.Time,
) domain.AccessState {
	hasCurrentAccess := false
	switch subscription.State {
	case domain.SubStateActive:
		hasCurrentAccess = true
	case domain.SubStateTrialing:
		hasCurrentAccess = subscription.TrialEndsAt != nil &&
			subscription.TrialEndsAt.After(now)
	case domain.SubStatePastDue:
		hasCurrentAccess = subscription.GracePeriodEndsAt != nil &&
			subscription.GracePeriodEndsAt.After(now)
	case domain.SubStateCanceled:
		hasCurrentAccess = subscription.RenewAt != nil &&
			subscription.RenewAt.After(now)
	}
	if !hasCurrentAccess {
		if plan.ExpiryBehavior == domain.ExpiryBehaviorDowngradeToFree {
			return domain.AccessStateFree
		}
		return domain.AccessStateFrozen
	}
	if plan.IsFree() {
		return domain.AccessStateFree
	}
	return domain.AccessStatePaid
}

func effectiveUserLimit(subscription *domain.Subscription, plan *domain.Plan) *int {
	if subscription.SeatsPurchased != nil && *subscription.SeatsPurchased > 0 {
		return cloneInt(subscription.SeatsPurchased)
	}
	return cloneInt(plan.MaxUsers)
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func isAtLimit(current int, limit *int) bool {
	return limit != nil && current >= *limit
}

func isOverLimit(current int, limit *int) bool {
	return limit != nil && current > *limit
}

func resolveCapabilities(
	snapshot *domain.EntitlementSnapshot,
) map[domain.Capability]domain.CapabilityDecision {
	capabilities := make(map[domain.Capability]domain.CapabilityDecision)
	for capability, rule := range domain.CapabilityRegistry() {
		if !containsAccessState(rule.AllowedStates, snapshot.AccessState) {
			var reason domain.EntitlementReason
			switch {
			case snapshot.AccessState == domain.AccessStateFrozen:
				reason = domain.EntitlementReasonAccountFrozen
			case rule.FeatureKey != "" && !entitlementFeatureEnabled(snapshot.Features, rule.FeatureKey):
				reason = domain.EntitlementReasonFeatureUnavailable
			case snapshot.AccessState == domain.AccessStateFreeOverQuota:
				reason = domain.EntitlementReasonPlanLimitReached
			default:
				reason = domain.EntitlementReasonFeatureUnavailable
			}
			capabilities[capability] = domain.CapabilityDecision{Allowed: false, Reason: reason}
			continue
		}
		if rule.FeatureKey != "" && !entitlementFeatureEnabled(snapshot.Features, rule.FeatureKey) {
			capabilities[capability] = domain.CapabilityDecision{
				Allowed: false,
				Reason:  domain.EntitlementReasonFeatureUnavailable,
			}
			continue
		}
		if rule.LimitKey != "" {
			current, limit := entitlementLimit(snapshot, rule.LimitKey)
			capabilities[capability] = limitDecision(current, limit)
			continue
		}
		capabilities[capability] = domain.CapabilityDecision{
			Allowed: true,
			Reason:  domain.EntitlementReasonAllowed,
		}
	}
	return capabilities
}

func containsAccessState(states []domain.AccessState, state domain.AccessState) bool {
	for _, allowed := range states {
		if allowed == state {
			return true
		}
	}
	return false
}

func entitlementFeatureEnabled(features domain.PlanFeatures, key string) bool {
	switch key {
	case "passkeys":
		return features.Passkeys
	case "sharing":
		return features.Sharing || features.SharedItems
	case "secure_send":
		return features.SecureSend
	case "emergency_access":
		return features.EmergencyAccess
	case "teams":
		return features.Teams
	case "audit":
		return features.Audit
	case "sso":
		return features.SSO
	case "policies":
		return features.Policies
	case "business_policies":
		return features.BusinessPolicies
	case "enterprise_policies":
		return features.EnterprisePolicies
	case "security_insights":
		return features.SecurityInsights
	case "breach_monitoring":
		return features.BreachMonitoring
	default:
		return false
	}
}

func entitlementLimit(snapshot *domain.EntitlementSnapshot, key string) (int, *int) {
	switch key {
	case "max_items":
		return snapshot.Usage.Items, snapshot.Limits.MaxItems
	case "max_collections":
		return snapshot.Usage.Collections, snapshot.Limits.MaxCollections
	case "max_users":
		return snapshot.Usage.Users, snapshot.Limits.MaxUsers
	default:
		return 0, nil
	}
}

func limitDecision(current int, limit *int) domain.CapabilityDecision {
	if isAtLimit(current, limit) {
		return domain.CapabilityDecision{
			Allowed: false,
			Reason:  domain.EntitlementReasonPlanLimitReached,
		}
	}
	return domain.CapabilityDecision{Allowed: true, Reason: domain.EntitlementReasonAllowed}
}

func applyEntitlementOverrides(
	overrides []*domain.OrganizationEntitlementOverride,
	features *domain.PlanFeatures,
	limits *domain.EntitlementLimits,
) {
	for _, override := range overrides {
		if override == nil {
			continue
		}
		value, exists := override.Value["value"]
		if !exists {
			continue
		}
		switch override.Key {
		case "feature.passkeys":
			if parsed, ok := value.(bool); ok {
				features.Passkeys = parsed
			}
		case "feature.sharing":
			if parsed, ok := value.(bool); ok {
				features.Sharing = parsed
			}
		case "feature.shared_items":
			if parsed, ok := value.(bool); ok {
				features.SharedItems = parsed
			}
		case "feature.secure_send":
			if parsed, ok := value.(bool); ok {
				features.SecureSend = parsed
			}
		case "feature.emergency_access":
			if parsed, ok := value.(bool); ok {
				features.EmergencyAccess = parsed
			}
		case "feature.teams":
			if parsed, ok := value.(bool); ok {
				features.Teams = parsed
			}
		case "feature.audit":
			if parsed, ok := value.(bool); ok {
				features.Audit = parsed
			}
		case "feature.sso":
			if parsed, ok := value.(bool); ok {
				features.SSO = parsed
			}
		case "feature.policies":
			if parsed, ok := value.(bool); ok {
				features.Policies = parsed
			}
		case "feature.business_policies":
			if parsed, ok := value.(bool); ok {
				features.BusinessPolicies = parsed
			}
		case "feature.enterprise_policies":
			if parsed, ok := value.(bool); ok {
				features.EnterprisePolicies = parsed
			}
		case "feature.security_insights":
			if parsed, ok := value.(bool); ok {
				features.SecurityInsights = parsed
			}
		case "feature.breach_monitoring":
			if parsed, ok := value.(bool); ok {
				features.BreachMonitoring = parsed
			}
		case "limit.max_users":
			if parsed, ok := overrideInt(value); ok {
				limits.MaxUsers = parsed
			}
		case "limit.max_collections":
			if parsed, ok := overrideInt(value); ok {
				limits.MaxCollections = parsed
			}
		case "limit.max_items":
			if parsed, ok := overrideInt(value); ok {
				limits.MaxItems = parsed
			}
		case "limit.max_devices":
			if parsed, ok := overrideInt(value); ok {
				limits.MaxDevices = parsed
			}
		}
	}
}

func applyCapabilityOverrides(
	overrides []*domain.OrganizationEntitlementOverride,
	capabilities map[domain.Capability]domain.CapabilityDecision,
) {
	for _, override := range overrides {
		if override == nil || !strings.HasPrefix(override.Key, "capability.") {
			continue
		}
		value, ok := override.Value["value"].(bool)
		if !ok {
			continue
		}
		capability := domain.Capability(strings.TrimPrefix(override.Key, "capability."))
		if _, exists := capabilities[capability]; !exists {
			continue
		}
		reason := domain.EntitlementReasonFeatureUnavailable
		if value {
			reason = domain.EntitlementReasonAllowed
		}
		capabilities[capability] = domain.CapabilityDecision{Allowed: value, Reason: reason}
	}
}

func overrideInt(value any) (*int, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, true
	case float64:
		if typed < 0 || typed != float64(int(typed)) {
			return nil, false
		}
		converted := int(typed)
		return &converted, true
	case int:
		if typed < 0 {
			return nil, false
		}
		converted := typed
		return &converted, true
	default:
		return nil, false
	}
}
