package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

// InvitationHandler serves organization invitations (admin and invitee side)
// and referral invitations.
type InvitationHandler struct {
	invitationService   service.InvitationService
	userService         service.UserService
	organizationService service.OrganizationService
	activityLogger      *service.ActivityLogger
}

// NewInvitationHandler creates a new invitation handler
func NewInvitationHandler(
	invitationService service.InvitationService,
	userService service.UserService,
	organizationService service.OrganizationService,
	activityService service.UserActivityService,
) *InvitationHandler {
	return &InvitationHandler{
		invitationService:   invitationService,
		userService:         userService,
		organizationService: organizationService,
		activityLogger:      service.NewActivityLogger(activityService),
	}
}

// writeInvitationError maps service errors to stable JSON responses.
func writeInvitationError(c *gin.Context, err error, fallback string) {
	if respondEntitlementError(c, err) {
		return
	}
	var typed *service.InvitationError
	if errors.As(err, &typed) {
		c.JSON(typed.Status, gin.H{"error": typed.Message, "code": typed.Code})
		return
	}
	switch {
	case errors.Is(err, repository.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied", "code": "FORBIDDEN"})
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "code": service.InvitationCodeNotFound})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
	}
}

func (h *InvitationHandler) logOrgActivity(c *gin.Context, actorID uint, activityType domain.ActivityType, orgID uint, details service.ActivityDetails) {
	if h.activityLogger == nil {
		return
	}
	details[service.ActivityFieldOrganizationID] = orgID
	_ = h.activityLogger.LogActivity(c.Request.Context(), actorID, activityType, GetIPAddress(c), GetUserAgent(c), details)
}

// ---------------------------------------------------------------------------
// Organization admin side: /api/organizations/:id/invitations
// ---------------------------------------------------------------------------

// CreateOrgInvitation invites someone to the organization.
// POST /api/organizations/:id/invitations
func (h *InvitationHandler) CreateOrgInvitation(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	var req domain.CreateOrgInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a valid email and role are required", "code": "INVALID_REQUEST"})
		return
	}
	result, err := h.organizationService.InviteMember(c.Request.Context(), orgID, userID, &req)
	if err != nil {
		writeInvitationError(c, err, "failed to invite member")
		return
	}
	h.logOrgActivity(c, userID, domain.ActivityTypeMemberInvited, orgID, service.ActivityDetails{
		service.ActivityFieldUserEmail: result.Invitation.Email,
		service.ActivityFieldRole:      result.Invitation.Role,
		"invitation_id":                result.Invitation.ID,
	})
	c.JSON(http.StatusCreated, gin.H{
		"invitation": domain.ToOrgInvitationDTO(result.Invitation, time.Now()),
		"email_sent": result.EmailSent,
	})
}

// ListOrgInvitations lists the organization's invitations.
// GET /api/organizations/:id/invitations?status=pending|accepted|declined|revoked|expired|all
func (h *InvitationHandler) ListOrgInvitations(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	status := c.DefaultQuery("status", "all")
	switch domain.OrganizationInvitationStatus(status) {
	case domain.OrgInvitationPending, domain.OrgInvitationAccepted, domain.OrgInvitationDeclined,
		domain.OrgInvitationRevoked, domain.OrgInvitationExpired, "all":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status filter", "code": "INVALID_REQUEST"})
		return
	}
	// Pending rows past their expiry report "expired"; fetch pending for both.
	query := status
	if status == string(domain.OrgInvitationExpired) {
		query = "all"
	}
	invitations, err := h.organizationService.ListInvitations(c.Request.Context(), orgID, GetCurrentUserID(c), query)
	if err != nil {
		writeInvitationError(c, err, "failed to list invitations")
		return
	}
	now := time.Now()
	dtos := make([]*domain.OrgInvitationDTO, 0, len(invitations))
	for _, inv := range invitations {
		dto := domain.ToOrgInvitationDTO(inv, now)
		if status != "all" && string(dto.Status) != status {
			continue
		}
		dtos = append(dtos, dto)
	}
	c.JSON(http.StatusOK, dtos)
}

// ResendOrgInvitation re-sends an invitation with a fresh expiry.
// POST /api/organizations/:id/invitations/:invitationId/resend
func (h *InvitationHandler) ResendOrgInvitation(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	invitationID, ok := GetUintParam(c, "invitationId")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	result, err := h.organizationService.ResendInvitation(c.Request.Context(), orgID, invitationID, userID)
	if err != nil {
		writeInvitationError(c, err, "failed to resend invitation")
		return
	}
	h.logOrgActivity(c, userID, domain.ActivityTypeInvitationResent, orgID, service.ActivityDetails{
		service.ActivityFieldUserEmail: result.Invitation.Email,
		"invitation_id":                invitationID,
	})
	c.JSON(http.StatusOK, gin.H{
		"invitation": domain.ToOrgInvitationDTO(result.Invitation, time.Now()),
		"email_sent": result.EmailSent,
	})
}

// RevokeOrgInvitation cancels a pending invitation.
// DELETE /api/organizations/:id/invitations/:invitationId
func (h *InvitationHandler) RevokeOrgInvitation(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	invitationID, ok := GetUintParam(c, "invitationId")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	inv, err := h.organizationService.RevokeInvitation(c.Request.Context(), orgID, invitationID, userID)
	if err != nil {
		writeInvitationError(c, err, "failed to revoke invitation")
		return
	}
	h.logOrgActivity(c, userID, domain.ActivityTypeInvitationRevoked, orgID, service.ActivityDetails{
		service.ActivityFieldUserEmail: inv.Email,
		"invitation_id":                invitationID,
	})
	c.JSON(http.StatusOK, domain.ToOrgInvitationDTO(inv, time.Now()))
}

// ---------------------------------------------------------------------------
// Invitee side: /api/invitations
// ---------------------------------------------------------------------------

// ListReceived returns pending invitations addressed to the current user.
// GET /api/invitations
func (h *InvitationHandler) ListReceived(c *gin.Context) {
	invitations, err := h.organizationService.ListReceivedInvitations(c.Request.Context(), GetCurrentUserID(c))
	if err != nil {
		writeInvitationError(c, err, "failed to list invitations")
		return
	}
	dtos := make([]*domain.ReceivedInvitationDTO, 0, len(invitations))
	for _, inv := range invitations {
		dtos = append(dtos, domain.ToReceivedInvitationDTO(inv))
	}
	c.JSON(http.StatusOK, dtos)
}

// Accept accepts an invitation and creates the membership.
// POST /api/invitations/:id/accept
func (h *InvitationHandler) Accept(c *gin.Context) {
	invitationID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	var req domain.AcceptOrgInvitationRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "code": "INVALID_REQUEST"})
			return
		}
	}
	userID := GetCurrentUserID(c)
	member, err := h.organizationService.AcceptReceivedInvitation(c.Request.Context(), invitationID, userID, req.EncryptedOrgKey)
	if err != nil {
		writeInvitationError(c, err, "failed to accept invitation")
		return
	}
	h.logOrgActivity(c, userID, domain.ActivityTypeMemberJoined, member.OrganizationID, service.ActivityDetails{
		"invitation_id":             invitationID,
		service.ActivityFieldStatus: member.Status,
	})
	c.JSON(http.StatusOK, gin.H{
		"organization_id":             member.OrganizationID,
		"status":                      member.Status,
		"requires_admin_confirmation": member.Status == domain.OrgUserStatusProvisioned,
	})
}

// Decline declines an invitation.
// POST /api/invitations/:id/decline
func (h *InvitationHandler) Decline(c *gin.Context) {
	invitationID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	if err := h.organizationService.DeclineReceivedInvitation(c.Request.Context(), invitationID, userID); err != nil {
		writeInvitationError(c, err, "failed to decline invitation")
		return
	}
	if h.activityLogger != nil {
		_ = h.activityLogger.LogActivity(c.Request.Context(), userID, domain.ActivityTypeInvitationDeclined, GetIPAddress(c), GetUserAgent(c), service.ActivityDetails{
			"invitation_id": invitationID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": domain.OrgInvitationDeclined})
}

// ---------------------------------------------------------------------------
// Referrals: /api/referrals
// ---------------------------------------------------------------------------

type createReferralRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type referralDTO struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Status    string    `json:"status"` // pending | joined | expired
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func toReferralDTO(inv *domain.Invitation, now time.Time) referralDTO {
	status := "pending"
	switch {
	case inv.IsUsed():
		status = "joined"
	case !now.Before(inv.ExpiresAt):
		status = "expired"
	}
	return referralDTO{ID: inv.ID, Email: inv.Email, Status: status, CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt}
}

// CreateReferral invites a friend to Passwall.
// POST /api/referrals
func (h *InvitationHandler) CreateReferral(c *gin.Context) {
	var req createReferralRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a valid email is required", "code": service.ReferralCodeInvalidEmail})
		return
	}
	userID := GetCurrentUserID(c)
	inviterName := ""
	if user, err := h.userService.GetByID(c.Request.Context(), userID); err == nil {
		inviterName = user.Name
	}
	inv, err := h.invitationService.CreateReferral(c.Request.Context(), req.Email, userID, inviterName)
	if err != nil {
		writeInvitationError(c, err, "failed to send invitation")
		return
	}
	if h.activityLogger != nil {
		h.activityLogger.LogInvitationSent(c.Request.Context(), userID, GetIPAddress(c), GetUserAgent(c), inv.Email)
	}
	c.JSON(http.StatusCreated, toReferralDTO(inv, time.Now()))
}

// ListReferrals lists referrals sent by the current user.
// GET /api/referrals
func (h *InvitationHandler) ListReferrals(c *gin.Context) {
	invitations, err := h.invitationService.ListSentReferrals(c.Request.Context(), GetCurrentUserID(c))
	if err != nil {
		writeInvitationError(c, err, "failed to list invitations")
		return
	}
	now := time.Now()
	dtos := make([]referralDTO, 0, len(invitations))
	for _, inv := range invitations {
		dtos = append(dtos, toReferralDTO(inv, now))
	}
	c.JSON(http.StatusOK, dtos)
}
