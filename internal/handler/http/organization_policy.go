package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type OrganizationPolicyHandler struct {
	service        service.OrganizationPolicyService
	activityLogger *service.ActivityLogger
}

func NewOrganizationPolicyHandler(service service.OrganizationPolicyService, activityLogger ...*service.ActivityLogger) *OrganizationPolicyHandler {
	handler := &OrganizationPolicyHandler{service: service}
	if len(activityLogger) > 0 {
		handler.activityLogger = activityLogger[0]
	}
	return handler
}

// ListPolicies godoc
// @Summary List organization policies
// @Description List all available policies for an organization (filtered by plan tier)
// @Tags organization-policies
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {array} domain.OrganizationPolicyDTO
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/policies [get]
func (h *OrganizationPolicyHandler) ListPolicies(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}

	policies, err := h.service.ListByOrganization(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list policies", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policies)
}

// GetPolicy godoc
// @Summary Get organization policy by type
// @Description Get a specific policy configuration for an organization
// @Tags organization-policies
// @Produce json
// @Param id path int true "Organization ID"
// @Param policyType path string true "Policy Type"
// @Success 200 {object} domain.OrganizationPolicyDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/policies/{policyType} [get]
func (h *OrganizationPolicyHandler) GetPolicy(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}

	policyType := domain.PolicyType(c.Param("policyType"))

	policy, err := h.service.GetByType(ctx, orgID, userID, policyType)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// UpdatePolicy godoc
// @Summary Update organization policy
// @Description Enable/disable a policy or update its configuration
// @Tags organization-policies
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param policyType path string true "Policy Type"
// @Param request body domain.UpdateOrganizationPolicyRequest true "Policy update"
// @Success 200 {object} domain.OrganizationPolicyDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/policies/{policyType} [put]
func (h *OrganizationPolicyHandler) UpdatePolicy(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}

	policyType := domain.PolicyType(c.Param("policyType"))

	var req domain.UpdateOrganizationPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	policy, err := h.service.UpdatePolicy(ctx, orgID, userID, policyType, &req)
	if err != nil {
		if respondEntitlementError(c, err) {
			return
		}
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, domain.ErrPolicyNotAvailable) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "POLICY_NOT_AVAILABLE"})
			return
		}
		var invalid *domain.PolicyValidationError
		if errors.As(err, &invalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": invalid.Error(), "code": "POLICY_INVALID_DATA", "field": invalid.Field})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)

	// Policy changes shape what every member may do; keep an audit trail.
	if h.activityLogger != nil {
		details := service.ActivityDetails{
			service.ActivityFieldOrganizationID: orgID,
			"policy_type":                       string(policyType),
			"enabled":                           policy.Enabled,
		}
		_ = h.activityLogger.LogActivity(ctx, userID, domain.ActivityTypeOrgPolicyChanged, GetIPAddress(c), GetUserAgent(c), details)
	}
}

// ListPolicyDefinitions godoc
// @Summary List all available policy definitions
// @Description Returns the catalog of all policy types with metadata (name, description, category, tier, dependencies)
// @Tags organization-policies
// @Produce json
// @Success 200 {array} domain.PolicyDefinition
// @Router /policies/definitions [get]
func (h *OrganizationPolicyHandler) ListPolicyDefinitions(c *gin.Context) {
	c.JSON(http.StatusOK, domain.AllPolicyDefinitions())
}

// GetActivePolicies godoc
// @Summary Get active policies for an organization
// @Description Returns only enabled policies with their data. Used by clients to adapt behavior.
// @Tags organization-policies
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /organizations/{id}/policies/active [get]
func (h *OrganizationPolicyHandler) GetActivePolicies(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}

	summary, err := h.service.GetActivePolicySummary(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get active policies"})
		return
	}

	c.JSON(http.StatusOK, summary)
}
