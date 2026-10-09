package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type AdminSubscriptionsHandler struct {
	orgRepo             repository.OrganizationRepository
	orgUserRepo         repository.OrganizationUserRepository
	subRepo             adminSubscriptionReader
	subscriptionService service.SubscriptionService
	paymentService      service.PaymentService
	activityLogger      *service.ActivityLogger
	logger              service.Logger
}

func NewAdminSubscriptionsHandler(
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	subRepo adminSubscriptionReader,
	subscriptionService service.SubscriptionService,
	paymentService service.PaymentService,
	userActivityService service.UserActivityService,
	logger service.Logger,
) *AdminSubscriptionsHandler {
	return &AdminSubscriptionsHandler{
		orgRepo:             orgRepo,
		orgUserRepo:         orgUserRepo,
		subRepo:             subRepo,
		subscriptionService: subscriptionService,
		paymentService:      paymentService,
		activityLogger:      service.NewActivityLogger(userActivityService),
		logger:              logger,
	}
}

type adminSubscriptionReader interface {
	GetEffectiveByOrganizationIDs(ctx context.Context, orgIDs []uint) (map[uint]*domain.Subscription, error)
}

type adminSubscriptionOwnerDTO struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type adminSubscriptionItemDTO struct {
	Organization   *domain.OrganizationDTO    `json:"organization"`
	Subscription   *domain.SubscriptionDTO    `json:"subscription,omitempty"`
	Owner          *adminSubscriptionOwnerDTO `json:"owner,omitempty"`
	CurrentUsers   int                        `json:"current_users"`
	IsStripe       bool                       `json:"is_stripe"`
	StripeCustID   *string                    `json:"stripe_customer_id,omitempty"`
	Provider       domain.PaymentProvider     `json:"provider"`
	AccessEndsAt   *time.Time                 `json:"access_ends_at,omitempty"`
	AutoRenews     bool                       `json:"auto_renews"`
	CanManualGrant bool                       `json:"can_manual_grant"`
	BlockedReason  *string                    `json:"blocked_reason,omitempty"`
	RiskExpiring   bool                       `json:"risk_expiring"`
	DaysToEnd      *int                       `json:"days_to_end,omitempty"`
}

type adminSubscriptionListResponse struct {
	Items    []*adminSubscriptionItemDTO `json:"items"`
	Total    int64                       `json:"total"`
	Filtered int64                       `json:"filtered"`
}

// List subscriptions across all organizations (admin-only).
// GET /api/admin/subscriptions?search=&owner_user_id=&ending_within_days=&sort=access_end&limit=&offset=
// search matches organization name, billing email and owner email/name.
func (h *AdminSubscriptionsHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	ownerUserID := parseUintWithDefault(firstNonEmpty(c.Query("owner_user_id"), c.Query("ownerUserId")), 0)
	limit, offset := parseAdminPagination(c)
	now := time.Now()

	filter := repository.ListFilter{
		Search:             c.Query("search"),
		SearchOwners:       true,
		OwnerUserID:        ownerUserID,
		Limit:              limit,
		Offset:             offset,
		Sort:               "created_at",
		Order:              "desc",
		SortByAccessEndAsc: c.Query("sort") == "access_end",
	}
	if days := parseIntWithDefault(c.Query("ending_within_days"), 0); days > 0 {
		if days > 365 {
			days = 365
		}
		cutoff := now.Add(time.Duration(days) * 24 * time.Hour)
		filter.ManualGrantEndsBefore = &cutoff
	}

	orgs, res, err := h.orgRepo.List(ctx, filter)
	if err != nil {
		h.logger.Error("admin subscriptions: failed to list organizations", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list organizations"})
		return
	}

	orgIDs := organizationIDs(orgs)
	owners := h.loadOwners(ctx, orgIDs)
	subs := h.loadSubscriptions(ctx, orgIDs)
	counts := h.loadCounts(ctx, orgIDs)
	expiringCutoff := now.Add(7 * 24 * time.Hour)

	items := make([]*adminSubscriptionItemDTO, 0, len(orgs))
	for _, org := range orgs {
		sub := subs[org.ID]
		item := &adminSubscriptionItemDTO{
			Organization:   domain.ToOrganizationDTOWithSubscription(org, sub),
			Owner:          owners[org.ID],
			CurrentUsers:   counts[org.ID].Members,
			StripeCustID:   org.StripeCustomerID,
			Provider:       domain.PaymentProviderNone,
			CanManualGrant: true,
		}
		if sub != nil {
			item.Subscription = domain.ToSubscriptionDTO(sub)
			item.Provider = domain.AdminListProvider(sub)
			item.IsStripe = item.Provider == domain.PaymentProviderStripe
			item.AccessEndsAt = sub.RenewAt
			item.AutoRenews = (item.Provider == domain.PaymentProviderStripe || item.Provider == domain.PaymentProviderRevenueCat) &&
				sub.State != domain.SubStateCanceled &&
				sub.State != domain.SubStateExpired
			if reason := adminManualGrantBlockedReason(sub, now); reason != nil {
				item.CanManualGrant = false
				item.BlockedReason = reason
			}
			// For external providers renew_at is the next renewal, not an end;
			// only manual grants are flagged as expiring.
			if item.Provider == domain.PaymentProviderManual && sub.RenewAt != nil {
				item.RiskExpiring = sub.RenewAt.Before(expiringCutoff)
				d := int(sub.RenewAt.Sub(now).Hours() / 24)
				item.DaysToEnd = &d
			}
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, &adminSubscriptionListResponse{
		Items:    items,
		Total:    res.Total,
		Filtered: res.Filtered,
	})
}

func parseAdminPagination(c *gin.Context) (int, int) {
	limit := parseIntWithDefault(c.Query("limit"), 20)
	offset := parseIntWithDefault(c.Query("offset"), 0)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func organizationIDs(orgs []*domain.Organization) []uint {
	ids := make([]uint, 0, len(orgs))
	for _, org := range orgs {
		ids = append(ids, org.ID)
	}
	return ids
}

// loadOwners, loadSubscriptions and loadCounts are best-effort: a failure
// degrades the list to missing columns instead of failing the page.
func (h *AdminSubscriptionsHandler) loadOwners(ctx context.Context, orgIDs []uint) map[uint]*adminSubscriptionOwnerDTO {
	out := make(map[uint]*adminSubscriptionOwnerDTO, len(orgIDs))
	owners, err := h.orgUserRepo.ListOwnersByOrganizationIDs(ctx, orgIDs)
	if err != nil {
		h.logger.Warn("admin subscriptions: failed to load owners", "error", err)
		return out
	}
	for orgID, owner := range owners {
		out[orgID] = &adminSubscriptionOwnerDTO{UserID: owner.UserID, Email: owner.User.Email, Name: owner.User.Name}
	}
	return out
}

func (h *AdminSubscriptionsHandler) loadSubscriptions(ctx context.Context, orgIDs []uint) map[uint]*domain.Subscription {
	subs, err := h.subRepo.GetEffectiveByOrganizationIDs(ctx, orgIDs)
	if err != nil {
		h.logger.Warn("admin subscriptions: failed to load subscriptions", "error", err)
		return map[uint]*domain.Subscription{}
	}
	return subs
}

func (h *AdminSubscriptionsHandler) loadCounts(ctx context.Context, orgIDs []uint) map[uint]repository.OrganizationCounts {
	counts, err := h.orgRepo.GetCountsByIDs(ctx, orgIDs)
	if err != nil {
		h.logger.Warn("admin subscriptions: failed to load usage counts", "error", err)
		return map[uint]repository.OrganizationCounts{}
	}
	return counts
}

type adminOrganizationsItemDTO struct {
	Organization    *domain.OrganizationDTO    `json:"organization"`
	Owner           *adminSubscriptionOwnerDTO `json:"owner,omitempty"`
	MemberCount     int                        `json:"member_count"`
	TeamCount       int                        `json:"team_count"`
	CollectionCount int                        `json:"collection_count"`
	ItemCount       int                        `json:"item_count"`
}

type adminOrganizationsListResponse struct {
	Items    []*adminOrganizationsItemDTO `json:"items"`
	Total    int64                        `json:"total"`
	Filtered int64                        `json:"filtered"`
}

// List organizations across the system (admin-only).
// GET /api/admin/organizations?search=&limit=&offset=
func (h *AdminSubscriptionsHandler) ListOrganizations(c *gin.Context) {
	ctx := c.Request.Context()
	limit, offset := parseAdminPagination(c)

	orgs, res, err := h.orgRepo.List(ctx, repository.ListFilter{
		Search: c.Query("search"),
		Limit:  limit,
		Offset: offset,
		Sort:   "created_at",
		Order:  "desc",
	})
	if err != nil {
		h.logger.Error("admin organizations: failed to list organizations", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list organizations"})
		return
	}

	orgIDs := organizationIDs(orgs)
	owners := h.loadOwners(ctx, orgIDs)
	subs := h.loadSubscriptions(ctx, orgIDs)
	counts := h.loadCounts(ctx, orgIDs)

	items := make([]*adminOrganizationsItemDTO, 0, len(orgs))
	for _, org := range orgs {
		owner := owners[org.ID]
		if owner != nil &&
			org.CreatedByUserID == nil &&
			org.CreatedByUserEmail == nil &&
			org.CreatedByUserName == nil {
			creatorID := owner.UserID
			creatorEmail := owner.Email
			creatorName := owner.Name
			org.CreatedByUserID = &creatorID
			org.CreatedByUserEmail = &creatorEmail
			org.CreatedByUserName = &creatorName
			if err := h.orgRepo.Update(ctx, org); err != nil {
				h.logger.Debug(
					"admin organizations: failed to backfill creator snapshot",
					"org_id",
					org.ID,
					"error",
					err,
				)
			}
		}

		count := counts[org.ID]
		org.MemberCount = &count.Members
		org.TeamCount = &count.Teams
		org.CollectionCount = &count.Collections
		org.ItemCount = &count.Items

		items = append(items, &adminOrganizationsItemDTO{
			Organization:    domain.ToOrganizationDTOWithSubscription(org, subs[org.ID]),
			Owner:           owner,
			MemberCount:     count.Members,
			TeamCount:       count.Teams,
			CollectionCount: count.Collections,
			ItemCount:       count.Items,
		})
	}

	c.JSON(http.StatusOK, &adminOrganizationsListResponse{
		Items:    items,
		Total:    res.Total,
		Filtered: res.Filtered,
	})
}

type grantManualSubscriptionRequest struct {
	PlanCode string `json:"plan_code" binding:"required"`
	EndsAt   string `json:"ends_at" binding:"required"` // RFC3339 timestamp
	Users    *int   `json:"users,omitempty"`            // Optional effective user limit
	Seats    *int   `json:"seats,omitempty"`            // Legacy alias (backward compatibility)
	Note     string `json:"note" binding:"required"`
}

func (h *AdminSubscriptionsHandler) GrantManual(c *gin.Context) {
	ctx := c.Request.Context()
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	var req grantManualSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		writeManualSubscriptionError(c, &service.ManualSubscriptionError{
			Code: service.ManualSubscriptionCodeInvalidEndDate, Message: "ends_at must be RFC3339",
		})
		return
	}
	seats := req.Users
	if seats == nil {
		seats = req.Seats
	}
	change, err := h.subscriptionService.GrantManual(ctx, orgID, service.ManualSubscriptionInput{
		PlanCode: req.PlanCode,
		EndsAt:   endsAt,
		Seats:    seats,
		Note:     req.Note,
	})
	if err != nil {
		writeManualSubscriptionError(c, err)
		return
	}
	h.logManualChange(c, domain.ActivityTypeAdminSubscriptionGranted, change, req.Note)
	h.writeBillingInfo(c, orgID, "subscription granted")
}

type extendManualSubscriptionRequest struct {
	EndsAt string `json:"ends_at" binding:"required"`
	Note   string `json:"note" binding:"required"`
}

func (h *AdminSubscriptionsHandler) ExtendManual(c *gin.Context) {
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	var req extendManualSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		writeManualSubscriptionError(c, &service.ManualSubscriptionError{
			Code: service.ManualSubscriptionCodeInvalidEndDate, Message: "ends_at must be RFC3339",
		})
		return
	}
	change, err := h.subscriptionService.ExtendManual(c.Request.Context(), orgID, endsAt, req.Note)
	if err != nil {
		writeManualSubscriptionError(c, err)
		return
	}
	h.logManualChange(c, domain.ActivityTypeAdminSubscriptionExtended, change, req.Note)
	h.writeBillingInfo(c, orgID, "subscription extended")
}

type endManualSubscriptionRequest struct {
	Note string `json:"note" binding:"required"`
}

func (h *AdminSubscriptionsHandler) RevokeManual(c *gin.Context) {
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	var req endManualSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	change, err := h.subscriptionService.EndManual(c.Request.Context(), orgID, req.Note)
	if err != nil {
		writeManualSubscriptionError(c, err)
		return
	}
	h.logManualChange(c, domain.ActivityTypeAdminSubscriptionEnded, change, req.Note)
	h.writeBillingInfo(c, orgID, "subscription ended")
}

func (h *AdminSubscriptionsHandler) writeBillingInfo(c *gin.Context, orgID uint, message string) {
	if billingInfo, err := h.paymentService.GetBillingInfo(c.Request.Context(), orgID); err == nil {
		c.JSON(http.StatusOK, billingInfo)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": message})
}

func (h *AdminSubscriptionsHandler) logManualChange(c *gin.Context, activityType domain.ActivityType, change *service.ManualSubscriptionChange, note string) {
	actorID, err := GetUserID(c)
	if err != nil || h.activityLogger == nil || change == nil || change.Organization == nil {
		return
	}
	details := service.ActivityDetails{
		service.ActivityFieldOrganizationID:   change.Organization.ID,
		service.ActivityFieldOrganizationName: change.Organization.Name,
		service.ActivityFieldOldPlan:          change.OldPlan,
		service.ActivityFieldNewPlan:          change.NewPlan,
		service.ActivityFieldReason:           strings.TrimSpace(note),
		"provider":                            change.Provider,
		"ends_at":                             change.EndsAt,
		"seats":                               change.Seats,
	}
	if owner := h.findOwner(c.Request.Context(), change.Organization.ID); owner != nil {
		details["owner_user_id"] = owner.UserID
		details["owner_email"] = owner.Email
	}
	_ = h.activityLogger.LogActivity(
		c.Request.Context(), actorID, activityType,
		GetIPAddress(c), GetUserAgent(c), details,
	)
}

func (h *AdminSubscriptionsHandler) findOwner(ctx context.Context, orgID uint) *adminSubscriptionOwnerDTO {
	members, err := h.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil
	}
	for _, member := range members {
		if member.Role == domain.OrgRoleOwner && member.User != nil {
			return &adminSubscriptionOwnerDTO{UserID: member.UserID, Email: member.User.Email, Name: member.User.Name}
		}
	}
	return nil
}

func writeManualSubscriptionError(c *gin.Context, err error) {
	var typed *service.ManualSubscriptionError
	if errors.As(err, &typed) {
		status := http.StatusBadRequest
		if typed.Code == service.ManualSubscriptionCodeExternalActive {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": typed.Message, "code": typed.Code})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "manual subscription operation failed"})
}

func adminManualGrantBlockedReason(sub *domain.Subscription, now time.Time) *string {
	provider := domain.AdminListProvider(sub)
	if (provider == domain.PaymentProviderStripe || provider == domain.PaymentProviderRevenueCat) &&
		adminExternalSubscriptionBlocksGrant(sub, now) {
		reason := service.ManualSubscriptionCodeExternalActive
		return &reason
	}
	return nil
}

func adminExternalSubscriptionBlocksGrant(sub *domain.Subscription, now time.Time) bool {
	switch sub.State {
	case domain.SubStateActive, domain.SubStateTrialing, domain.SubStatePastDue:
		return true
	case domain.SubStateCanceled:
		return sub.RenewAt != nil && sub.RenewAt.After(now)
	default:
		return false
	}
}

func parseIntWithDefault(v string, def int) int {
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func parseUintWithDefault(v string, def uint) uint {
	if v == "" {
		return def
	}
	i, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return def
	}
	return uint(i)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
