package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	uuid "github.com/satori/go.uuid"
)

type AdminSubscriptionsHandler struct {
	orgRepo     repository.OrganizationRepository
	orgUserRepo repository.OrganizationUserRepository
	subRepo     interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
		Create(ctx context.Context, sub *domain.Subscription) error
		Update(ctx context.Context, sub *domain.Subscription) error
	}
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	}
	paymentService service.PaymentService
	activityLogger *service.ActivityLogger
	logger         service.Logger
}

func NewAdminSubscriptionsHandler(
	orgRepo repository.OrganizationRepository,
	orgUserRepo repository.OrganizationUserRepository,
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
		Create(ctx context.Context, sub *domain.Subscription) error
		Update(ctx context.Context, sub *domain.Subscription) error
	},
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	},
	paymentService service.PaymentService,
	userActivityService service.UserActivityService,
	logger service.Logger,
) *AdminSubscriptionsHandler {
	return &AdminSubscriptionsHandler{
		orgRepo:        orgRepo,
		orgUserRepo:    orgUserRepo,
		subRepo:        subRepo,
		planRepo:       planRepo,
		paymentService: paymentService,
		activityLogger: service.NewActivityLogger(userActivityService),
		logger:         logger,
	}
}

type adminSubscriptionOwnerDTO struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type adminSubscriptionItemDTO struct {
	Organization *domain.OrganizationDTO    `json:"organization"`
	Subscription *domain.SubscriptionDTO    `json:"subscription,omitempty"`
	Owner        *adminSubscriptionOwnerDTO `json:"owner,omitempty"`
	CurrentUsers int                        `json:"current_users"`
	IsStripe     bool                       `json:"is_stripe"`
	StripeCustID *string                    `json:"stripe_customer_id,omitempty"`
	RiskExpiring bool                       `json:"risk_expiring"`
	DaysToEnd    *int                       `json:"days_to_end,omitempty"`
}

type adminSubscriptionListResponse struct {
	Items    []*adminSubscriptionItemDTO `json:"items"`
	Total    int64                       `json:"total"`
	Filtered int64                       `json:"filtered"`
}

// List subscriptions across all organizations (admin-only).
// GET /api/admin/subscriptions?search=&limit=&offset=
func (h *AdminSubscriptionsHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	search := c.Query("search")
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

	orgs, res, err := h.orgRepo.List(ctx, repository.ListFilter{
		Search: search,
		Limit:  limit,
		Offset: offset,
		Sort:   "created_at",
		Order:  "desc",
	})
	if err != nil {
		h.logger.Error("admin subscriptions: failed to list organizations", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list organizations"})
		return
	}

	now := time.Now()
	expiringCutoff := now.Add(7 * 24 * time.Hour)

	items := make([]*adminSubscriptionItemDTO, 0, len(orgs))
	for _, org := range orgs {
		// Owner info (best-effort)
		var owner *adminSubscriptionOwnerDTO
		if members, err := h.orgUserRepo.ListByOrganization(ctx, org.ID); err == nil {
			for _, m := range members {
				if m.Role == domain.OrgRoleOwner && m.User != nil {
					owner = &adminSubscriptionOwnerDTO{
						UserID: m.UserID,
						Email:  m.User.Email,
						Name:   m.User.Name,
					}
					break
				}
			}
		}

		// Subscription (best-effort)
		var sub *domain.Subscription
		var subDTO *domain.SubscriptionDTO
		isStripe := false
		riskExpiring := false
		var daysToEnd *int

		if s, err := h.subRepo.GetByOrganizationID(ctx, org.ID); err == nil && s != nil {
			sub = s
			subDTO = domain.ToSubscriptionDTO(s)
			if s.StripeSubscriptionID != nil && *s.StripeSubscriptionID != "" {
				isStripe = true
			}

			// Business-risk highlighting:
			// - Stripe subscriptions: "renew_at" is next renewal (not end), we don't flag as expiring.
			// - Manual grants (stripe_subscription_id null): we treat "renew_at" as end date.
			if !isStripe && s.RenewAt != nil {
				if s.RenewAt.Before(expiringCutoff) {
					riskExpiring = true
				}
				d := int(s.RenewAt.Sub(now).Hours() / 24)
				daysToEnd = &d
			}
		}

		orgDTO := domain.ToOrganizationDTOWithSubscription(org, sub)

		// Current users (best-effort)
		currentUsers := 0
		if cnt, err := h.orgRepo.GetMemberCount(ctx, org.ID); err == nil {
			currentUsers = cnt
		}

		items = append(items, &adminSubscriptionItemDTO{
			Organization: orgDTO,
			Subscription: subDTO,
			Owner:        owner,
			CurrentUsers: currentUsers,
			IsStripe:     isStripe,
			StripeCustID: org.StripeCustomerID,
			RiskExpiring: riskExpiring,
			DaysToEnd:    daysToEnd,
		})
	}

	c.JSON(http.StatusOK, &adminSubscriptionListResponse{
		Items:    items,
		Total:    res.Total,
		Filtered: res.Filtered,
	})
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

	search := c.Query("search")
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

	orgs, res, err := h.orgRepo.List(ctx, repository.ListFilter{
		Search: search,
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

	items := make([]*adminOrganizationsItemDTO, 0, len(orgs))
	for _, org := range orgs {
		// Owner info (best-effort)
		var owner *adminSubscriptionOwnerDTO
		if members, err := h.orgUserRepo.ListByOrganization(ctx, org.ID); err == nil {
			for _, m := range members {
				if m.Role == domain.OrgRoleOwner && m.User != nil {
					owner = &adminSubscriptionOwnerDTO{
						UserID: m.UserID,
						Email:  m.User.Email,
						Name:   m.User.Name,
					}
					break
				}
			}
		}
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

		// Subscription (best-effort) - used to derive plan/limits
		var sub *domain.Subscription
		if s, err := h.subRepo.GetByOrganizationID(ctx, org.ID); err == nil && s != nil {
			sub = s
		}

		// Stats (best-effort)
		memberCount := 0
		teamCount := 0
		collectionCount := 0
		itemCount := 0

		if cnt, err := h.orgRepo.GetMemberCount(ctx, org.ID); err == nil {
			memberCount = cnt
			org.MemberCount = &memberCount
		}
		if cnt, err := h.orgRepo.GetTeamCount(ctx, org.ID); err == nil {
			teamCount = cnt
			org.TeamCount = &teamCount
		}
		if cnt, err := h.orgRepo.GetCollectionCount(ctx, org.ID); err == nil {
			collectionCount = cnt
			org.CollectionCount = &collectionCount
		}
		if cnt, err := h.orgRepo.GetItemCount(ctx, org.ID); err == nil {
			itemCount = cnt
			org.ItemCount = &itemCount
		}

		orgDTO := domain.ToOrganizationDTOWithSubscription(org, sub)

		items = append(items, &adminOrganizationsItemDTO{
			Organization:    orgDTO,
			Owner:           owner,
			MemberCount:     memberCount,
			TeamCount:       teamCount,
			CollectionCount: collectionCount,
			ItemCount:       itemCount,
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
	Note     string `json:"note,omitempty"`
}

// GrantManual grants a plan without payment (admin-only).
// We implement this as: state=canceled + renew_at=end_date, so features remain until end_date,
// and the subscription worker will auto-expire it once renew_at passes.
//
// POST /api/admin/organizations/:id/subscription/grant
func (h *AdminSubscriptionsHandler) GrantManual(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req grantManualSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ends_at (must be RFC3339)"})
		return
	}
	now := time.Now()
	if !endsAt.After(now) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ends_at must be in the future"})
		return
	}

	org, err := h.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	plan, err := h.planRepo.GetByCode(ctx, req.PlanCode)
	if err != nil || plan == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan not found"})
		return
	}
	if !plan.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan is not active"})
		return
	}

	// Load existing subscription (if any)
	sub, subErr := h.subRepo.GetByOrganizationID(ctx, orgID)
	if subErr == nil && sub != nil && sub.StripeSubscriptionID != nil && *sub.StripeSubscriptionID != "" {
		// Safety: do not override Stripe-managed subscriptions via manual grant.
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot manually grant plan for Stripe-managed subscription"})
		return
	}

	oldPlanCode := ""
	if sub != nil && sub.Plan != nil {
		oldPlanCode = sub.Plan.Code
	}

	if sub == nil || subErr != nil {
		sub = &domain.Subscription{
			UUID:           uuid.NewV4(),
			OrganizationID: orgID,
		}
	}

	sub.PlanID = plan.ID
	// IMPORTANT:
	// We may have loaded an existing subscription with Preload("Plan"), meaning sub.Plan can point to the old plan.
	// GORM can sync foreign keys from loaded associations on Save(), which would overwrite PlanID.
	// Clear the association to ensure PlanID is persisted.
	sub.Plan = nil
	// Manual grants should appear as ACTIVE to the business/user.
	// We use renew_at as the manual end date and expire it via the worker when time passes.
	sub.State = domain.SubStateActive
	sub.StartedAt = &now
	sub.CancelAt = nil
	sub.RenewAt = &endsAt
	sub.EndedAt = nil
	sub.GracePeriodEndsAt = nil
	sub.TrialEndsAt = nil
	sub.StripeSubscriptionID = nil

	if sub.ID == 0 {
		if err := h.subRepo.Create(ctx, sub); err != nil {
			h.logger.Error("admin subscriptions: failed to create manual subscription", "org_id", orgID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to grant subscription"})
			return
		}
	} else {
		if err := h.subRepo.Update(ctx, sub); err != nil {
			h.logger.Error("admin subscriptions: failed to update manual subscription", "org_id", orgID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to grant subscription"})
			return
		}
	}

	// Audit log (admin action)
	if actorID, actorErr := GetUserID(c); actorErr == nil && h.activityLogger != nil {
		ipAddress := GetIPAddress(c)
		userAgent := GetUserAgent(c)
		h.activityLogger.LogCustomActivity(ctx, actorID, domain.ActivityTypeSubscriptionUpdated, ipAddress, userAgent, service.ActivityDetails{
			service.ActivityFieldOrganizationID:   orgID,
			service.ActivityFieldOrganizationName: org.Name,
			service.ActivityFieldOldPlan:          oldPlanCode,
			service.ActivityFieldNewPlan:          plan.Code,
			service.ActivityFieldReason:           req.Note,
			"manual_grant":                        true,
			"ends_at":                             endsAt.Format(time.RFC3339),
		})
	}

	// Return fresh billing info for convenience
	if billingInfo, err := h.paymentService.GetBillingInfo(ctx, orgID); err == nil {
		c.JSON(http.StatusOK, billingInfo)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription granted"})
}

// RevokeManual expires a manual grant immediately (admin-only).
// POST /api/admin/organizations/:id/subscription/revoke
func (h *AdminSubscriptionsHandler) RevokeManual(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	org, err := h.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return
	}

	sub, err := h.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil || sub == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}

	if sub.StripeSubscriptionID != nil && *sub.StripeSubscriptionID != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot revoke Stripe-managed subscription via this endpoint"})
		return
	}

	now := time.Now()
	sub.State = domain.SubStateExpired
	sub.EndedAt = &now

	if err := h.subRepo.Update(ctx, sub); err != nil {
		h.logger.Error("admin subscriptions: failed to revoke manual subscription", "org_id", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke subscription"})
		return
	}

	// Audit log
	if actorID, actorErr := GetUserID(c); actorErr == nil && h.activityLogger != nil {
		ipAddress := GetIPAddress(c)
		userAgent := GetUserAgent(c)
		h.activityLogger.LogCustomActivity(ctx, actorID, domain.ActivityTypeSubscriptionUpdated, ipAddress, userAgent, service.ActivityDetails{
			service.ActivityFieldOrganizationID:   orgID,
			service.ActivityFieldOrganizationName: org.Name,
			"manual_revoke":                       true,
		})
	}

	if billingInfo, err := h.paymentService.GetBillingInfo(ctx, orgID); err == nil {
		c.JSON(http.StatusOK, billingInfo)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription revoked"})
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
