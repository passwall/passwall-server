package service

import (
	"context"
	"encoding/json"

	"github.com/passwall/passwall-server/internal/domain"
)

// Activity detail field constants
const (
	ActivityFieldOrganizationID     = "organization_id"
	ActivityFieldOrganizationName   = "organization_name"
	ActivityFieldUserID             = "user_id"
	ActivityFieldUserName           = "user_name"
	ActivityFieldUserEmail          = "user_email"
	ActivityFieldPlan               = "plan"
	ActivityFieldOldPlan            = "old_plan"
	ActivityFieldNewPlan            = "new_plan"
	ActivityFieldBillingCycle       = "billing_cycle"
	ActivityFieldSubscriptionID     = "subscription_id"
	ActivityFieldSessionID          = "session_id"
	ActivityFieldCustomerID         = "customer_id"
	ActivityFieldInvoiceID          = "invoice_id"
	ActivityFieldAmount             = "amount"
	ActivityFieldCurrency           = "currency"
	ActivityFieldStatus             = "status"
	ActivityFieldCancelAtPeriodEnd  = "cancel_at_period_end"
	ActivityFieldReason             = "reason"
	ActivityFieldError              = "error"
	ActivityFieldItemID             = "item_id"
	ActivityFieldItemType           = "item_type"
	ActivityFieldCollectionID       = "collection_id"
	ActivityFieldCollectionName     = "collection_name"
	ActivityFieldTeamID             = "team_id"
	ActivityFieldTeamName           = "team_name"
	ActivityFieldRole               = "role"
	ActivityFieldOldRole            = "old_role"
	ActivityFieldNewRole            = "new_role"
)

// ActivityLogger provides helper methods for logging user activities
type ActivityLogger struct {
	service UserActivityService
}

// NewActivityLogger creates a new activity logger
func NewActivityLogger(service UserActivityService) *ActivityLogger {
	return &ActivityLogger{
		service: service,
	}
}

// ActivityDetails is a flexible map for activity details
type ActivityDetails map[string]interface{}

// ToJSON converts activity details to JSON string
func (d ActivityDetails) ToJSON() string {
	bytes, err := json.Marshal(d)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

// LogActivity logs an activity with structured details
func (l *ActivityLogger) LogActivity(ctx context.Context, userID uint, activityType domain.ActivityType, ipAddress, userAgent string, details ActivityDetails) error {
	return l.service.LogActivity(ctx, &domain.CreateActivityRequest{
		UserID:       userID,
		ActivityType: activityType,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Details:      details.ToJSON(),
	})
}

// Payment Activity Builders

// LogCheckoutCreated logs checkout session creation
func (l *ActivityLogger) LogCheckoutCreated(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, plan, billingCycle, sessionID string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeCheckoutCreated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldPlan:             plan,
		ActivityFieldBillingCycle:     billingCycle,
		ActivityFieldSessionID:        sessionID,
	})
}

// LogSubscriptionCreated logs subscription creation
func (l *ActivityLogger) LogSubscriptionCreated(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, subscriptionID, plan, billingCycle, status string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeSubscriptionCreated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldSubscriptionID:   subscriptionID,
		ActivityFieldPlan:             plan,
		ActivityFieldBillingCycle:     billingCycle,
		ActivityFieldStatus:           status,
	})
}

// LogSubscriptionUpdated logs subscription update
func (l *ActivityLogger) LogSubscriptionUpdated(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, subscriptionID, plan, billingCycle, status string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeSubscriptionUpdated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldSubscriptionID:   subscriptionID,
		ActivityFieldPlan:             plan,
		ActivityFieldBillingCycle:     billingCycle,
		ActivityFieldStatus:           status,
	})
}

// LogOrganizationUpgraded logs organization plan upgrade
func (l *ActivityLogger) LogOrganizationUpgraded(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, subscriptionID, oldPlan, newPlan, billingCycle, status string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeOrganizationUpgraded, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldSubscriptionID:   subscriptionID,
		ActivityFieldOldPlan:          oldPlan,
		ActivityFieldNewPlan:          newPlan,
		ActivityFieldBillingCycle:     billingCycle,
		ActivityFieldStatus:           status,
	})
}

// LogOrganizationDowngraded logs organization plan downgrade
func (l *ActivityLogger) LogOrganizationDowngraded(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, subscriptionID, oldPlan, newPlan, reason string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeOrganizationDowngraded, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldSubscriptionID:   subscriptionID,
		ActivityFieldOldPlan:          oldPlan,
		ActivityFieldNewPlan:          newPlan,
		ActivityFieldReason:           reason,
	})
}

// LogSubscriptionCanceled logs subscription cancellation
func (l *ActivityLogger) LogSubscriptionCanceled(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, subscriptionID, plan string, cancelAtPeriodEnd bool) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeSubscriptionCanceled, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:    orgID,
		ActivityFieldOrganizationName:  orgName,
		ActivityFieldSubscriptionID:    subscriptionID,
		ActivityFieldPlan:              plan,
		ActivityFieldCancelAtPeriodEnd: cancelAtPeriodEnd,
	})
}

// LogSubscriptionReactivated logs subscription reactivation
func (l *ActivityLogger) LogSubscriptionReactivated(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, subscriptionID, plan string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeSubscriptionReactivated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldSubscriptionID:   subscriptionID,
		ActivityFieldPlan:             plan,
	})
}

// LogInvoicePaid logs successful invoice payment
func (l *ActivityLogger) LogInvoicePaid(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, invoiceID string, amount int64, currency string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeInvoicePaid, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldInvoiceID:        invoiceID,
		ActivityFieldAmount:           amount,
		ActivityFieldCurrency:         currency,
	})
}

// LogInvoicePaymentFailed logs failed invoice payment
func (l *ActivityLogger) LogInvoicePaymentFailed(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, invoiceID string, amount int64, currency, reason string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeInvoicePaymentFailed, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldInvoiceID:        invoiceID,
		ActivityFieldAmount:           amount,
		ActivityFieldCurrency:         currency,
		ActivityFieldReason:           reason,
	})
}

// Organization Activity Builders

// LogOrganizationCreated logs organization creation
func (l *ActivityLogger) LogOrganizationCreated(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName, plan string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeItemCreated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldPlan:             plan,
	})
}

// LogOrganizationDeleted logs organization deletion
func (l *ActivityLogger) LogOrganizationDeleted(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeItemDeleted, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
	})
}

// Team Activity Builders

// LogTeamCreated logs team creation
func (l *ActivityLogger) LogTeamCreated(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName string, teamID uint, teamName string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeItemCreated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldTeamID:           teamID,
		ActivityFieldTeamName:         teamName,
	})
}

// LogMemberRoleChanged logs member role change
func (l *ActivityLogger) LogMemberRoleChanged(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName string, targetUserID uint, targetUserName, oldRole, newRole string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeItemUpdated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldUserID:           targetUserID,
		ActivityFieldUserName:         targetUserName,
		ActivityFieldOldRole:          oldRole,
		ActivityFieldNewRole:          newRole,
	})
}

// Collection Activity Builders

// LogCollectionCreated logs collection creation
func (l *ActivityLogger) LogCollectionCreated(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName string, collectionID uint, collectionName string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeItemCreated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldCollectionID:     collectionID,
		ActivityFieldCollectionName:   collectionName,
	})
}

// LogCollectionShared logs collection sharing
func (l *ActivityLogger) LogCollectionShared(ctx context.Context, userID uint, ipAddress, userAgent string, orgID uint, orgName string, collectionID uint, collectionName string, sharedWithUserID uint, sharedWithUserName string) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeItemUpdated, ipAddress, userAgent, ActivityDetails{
		ActivityFieldOrganizationID:   orgID,
		ActivityFieldOrganizationName: orgName,
		ActivityFieldCollectionID:     collectionID,
		ActivityFieldCollectionName:   collectionName,
		ActivityFieldUserID:           sharedWithUserID,
		ActivityFieldUserName:         sharedWithUserName,
	})
}

// Generic helpers for custom activities

// LogCustomActivity logs a custom activity with flexible details
func (l *ActivityLogger) LogCustomActivity(ctx context.Context, userID uint, activityType domain.ActivityType, ipAddress, userAgent string, details ActivityDetails) {
	_ = l.LogActivity(ctx, userID, activityType, ipAddress, userAgent, details)
}

// LogError logs an error activity
func (l *ActivityLogger) LogError(ctx context.Context, userID uint, ipAddress, userAgent string, operation string, err error) {
	_ = l.LogActivity(ctx, userID, domain.ActivityTypeFailedSignIn, ipAddress, userAgent, ActivityDetails{
		"operation":              operation,
		ActivityFieldError:       err.Error(),
	})
}

