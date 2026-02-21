package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
)

// SCIMHandler handles SCIM 2.0 provisioning endpoints
type SCIMHandler struct {
	scimService service.SCIMService
	orgService  interface {
		GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error)
	}
}

// NewSCIMHandler creates a new SCIM handler
func NewSCIMHandler(
	scimService service.SCIMService,
	orgService interface {
		GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error)
	},
) *SCIMHandler {
	return &SCIMHandler{scimService: scimService, orgService: orgService}
}

// --- Token Management (authenticated org admin endpoints) ---

// CreateToken generates a new SCIM bearer token
func (h *SCIMHandler) CreateToken(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	var req domain.CreateSCIMTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	result, err := h.scimService.CreateToken(ctx, orgID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create SCIM token"})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// ListTokens lists SCIM tokens for an organization
func (h *SCIMHandler) ListTokens(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	tokens, err := h.scimService.ListTokens(ctx, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list SCIM tokens"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// RevokeToken revokes a SCIM token
func (h *SCIMHandler) RevokeToken(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	tokenID, ok := GetUintParam(c, "tokenId")
	if !ok {
		return
	}

	if err := h.scimService.RevokeToken(ctx, orgID, tokenID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke SCIM token"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// --- SCIM 2.0 Endpoints (authenticated by SCIM bearer token) ---

// SCIMAuthMiddleware validates SCIM bearer tokens and sets org context
func SCIMAuthMiddleware(scimService service.SCIMService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			scimError(c, http.StatusUnauthorized, "authorization header required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			scimError(c, http.StatusUnauthorized, "invalid authorization header format")
			c.Abort()
			return
		}

		orgID, err := scimService.ValidateToken(c.Request.Context(), parts[1])
		if err != nil {
			scimError(c, http.StatusUnauthorized, "invalid or expired SCIM token")
			c.Abort()
			return
		}

		c.Set("scim_org_id", orgID)
		c.Next()
	}
}

func getSCIMOrgID(c *gin.Context) uint {
	orgID, _ := c.Get("scim_org_id")
	if id, ok := orgID.(uint); ok {
		return id
	}
	return 0
}

// ServiceProviderConfig returns SCIM 2.0 ServiceProviderConfig (RFC 7643 §5)
func (h *SCIMHandler) ServiceProviderConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"schemas": []string{domain.SCIMSchemaServiceConfig},
		"documentationUri": "https://passwall.io/docs/scim",
		"patch": gin.H{
			"supported": true,
		},
		"bulk": gin.H{
			"supported":  false,
			"maxOperations": 0,
			"maxPayloadSize": 0,
		},
		"filter": gin.H{
			"supported":  true,
			"maxResults": 100,
		},
		"changePassword": gin.H{
			"supported": false,
		},
		"sort": gin.H{
			"supported": false,
		},
		"etag": gin.H{
			"supported": false,
		},
		"authenticationSchemes": []gin.H{
			{
				"name":             "OAuth Bearer Token",
				"description":      "Authentication scheme using the OAuth Bearer Token Standard",
				"specUri":          "http://www.rfc-editor.org/info/rfc6750",
				"type":             "oauthbearertoken",
				"primary":          true,
			},
		},
		"meta": gin.H{
			"resourceType": "ServiceProviderConfig",
			"location":     "/scim/v2/ServiceProviderConfig",
		},
	})
}

// ResourceTypes returns SCIM 2.0 ResourceTypes
func (h *SCIMHandler) ResourceTypes(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{
		{
			"schemas":     []string{domain.SCIMSchemaResourceType},
			"id":          "User",
			"name":        "User",
			"endpoint":    "/Users",
			"schema":      domain.SCIMSchemaUser,
			"meta": gin.H{
				"resourceType": "ResourceType",
				"location":     "/scim/v2/ResourceTypes/User",
			},
		},
		{
			"schemas":     []string{domain.SCIMSchemaResourceType},
			"id":          "Group",
			"name":        "Group",
			"endpoint":    "/Groups",
			"schema":      domain.SCIMSchemaGroup,
			"meta": gin.H{
				"resourceType": "ResourceType",
				"location":     "/scim/v2/ResourceTypes/Group",
			},
		},
	})
}

// --- SCIM User Endpoints ---

// ListUsers handles GET /scim/v2/Users
func (h *SCIMHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)

	filter := c.Query("filter")
	startIndex, _ := strconv.Atoi(c.DefaultQuery("startIndex", "1"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "100"))

	result, err := h.scimService.ListUsers(ctx, orgID, filter, startIndex, count)
	if err != nil {
		scimError(c, http.StatusInternalServerError, "failed to list users")
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetUser handles GET /scim/v2/Users/:id
func (h *SCIMHandler) GetUser(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)
	userID := c.Param("id")

	user, err := h.scimService.GetUser(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, service.ErrSCIMUserNotFound) {
			scimError(c, http.StatusNotFound, "User not found")
			return
		}
		scimError(c, http.StatusInternalServerError, "failed to get user")
		return
	}

	c.JSON(http.StatusOK, user)
}

// CreateUser handles POST /scim/v2/Users
func (h *SCIMHandler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)

	var scimUser domain.SCIMUser
	if err := c.ShouldBindJSON(&scimUser); err != nil {
		scimError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.scimService.CreateUser(ctx, orgID, &scimUser)
	if err != nil {
		if errors.Is(err, service.ErrSCIMUserExists) {
			scimError(c, http.StatusConflict, "User already exists in organization")
			return
		}
		if errors.Is(err, service.ErrSCIMProvisioningBlocked) {
			scimError(c, http.StatusNotImplemented, err.Error())
			return
		}
		scimError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, user)
}

// UpdateUser handles PUT /scim/v2/Users/:id
func (h *SCIMHandler) UpdateUser(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)
	userID := c.Param("id")

	var scimUser domain.SCIMUser
	if err := c.ShouldBindJSON(&scimUser); err != nil {
		scimError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.scimService.UpdateUser(ctx, orgID, userID, &scimUser)
	if err != nil {
		if errors.Is(err, service.ErrSCIMUserNotFound) {
			scimError(c, http.StatusNotFound, "User not found")
			return
		}
		scimError(c, http.StatusInternalServerError, "failed to update user")
		return
	}

	c.JSON(http.StatusOK, user)
}

// PatchUser handles PATCH /scim/v2/Users/:id
func (h *SCIMHandler) PatchUser(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)
	userID := c.Param("id")

	var patch domain.SCIMPatchOp
	if err := c.ShouldBindJSON(&patch); err != nil {
		scimError(c, http.StatusBadRequest, "invalid PATCH body")
		return
	}

	user, err := h.scimService.PatchUser(ctx, orgID, userID, &patch)
	if err != nil {
		if errors.Is(err, service.ErrSCIMUserNotFound) {
			scimError(c, http.StatusNotFound, "User not found")
			return
		}
		scimError(c, http.StatusInternalServerError, "failed to patch user")
		return
	}

	c.JSON(http.StatusOK, user)
}

// DeleteUser handles DELETE /scim/v2/Users/:id
func (h *SCIMHandler) DeleteUser(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)
	userID := c.Param("id")

	if err := h.scimService.DeleteUser(ctx, orgID, userID); err != nil {
		if errors.Is(err, service.ErrSCIMUserNotFound) {
			scimError(c, http.StatusNotFound, "User not found")
			return
		}
		scimError(c, http.StatusInternalServerError, "failed to delete user")
		return
	}

	c.Status(http.StatusNoContent)
}

// --- SCIM Group Endpoints ---

// ListGroups handles GET /scim/v2/Groups
func (h *SCIMHandler) ListGroups(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)

	filter := c.Query("filter")
	startIndex, _ := strconv.Atoi(c.DefaultQuery("startIndex", "1"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "100"))

	result, err := h.scimService.ListGroups(ctx, orgID, filter, startIndex, count)
	if err != nil {
		scimError(c, http.StatusInternalServerError, "failed to list groups")
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetGroup handles GET /scim/v2/Groups/:id
func (h *SCIMHandler) GetGroup(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)
	groupID := c.Param("id")

	group, err := h.scimService.GetGroup(ctx, orgID, groupID)
	if err != nil {
		if errors.Is(err, service.ErrSCIMGroupNotFound) {
			scimError(c, http.StatusNotFound, "Group not found")
			return
		}
		scimError(c, http.StatusInternalServerError, "failed to get group")
		return
	}

	c.JSON(http.StatusOK, group)
}

// CreateGroup handles POST /scim/v2/Groups
func (h *SCIMHandler) CreateGroup(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)

	var scimGroup domain.SCIMGroup
	if err := c.ShouldBindJSON(&scimGroup); err != nil {
		scimError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	group, err := h.scimService.CreateGroup(ctx, orgID, &scimGroup)
	if err != nil {
		scimError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, group)
}

// UpdateGroup handles PUT /scim/v2/Groups/:id
func (h *SCIMHandler) UpdateGroup(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)
	groupID := c.Param("id")

	var scimGroup domain.SCIMGroup
	if err := c.ShouldBindJSON(&scimGroup); err != nil {
		scimError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	group, err := h.scimService.UpdateGroup(ctx, orgID, groupID, &scimGroup)
	if err != nil {
		if errors.Is(err, service.ErrSCIMGroupNotFound) {
			scimError(c, http.StatusNotFound, "Group not found")
			return
		}
		scimError(c, http.StatusInternalServerError, "failed to update group")
		return
	}

	c.JSON(http.StatusOK, group)
}

// PatchGroup handles PATCH /scim/v2/Groups/:id
func (h *SCIMHandler) PatchGroup(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)
	groupID := c.Param("id")

	var patch domain.SCIMPatchOp
	if err := c.ShouldBindJSON(&patch); err != nil {
		scimError(c, http.StatusBadRequest, "invalid PATCH body")
		return
	}

	group, err := h.scimService.PatchGroup(ctx, orgID, groupID, &patch)
	if err != nil {
		if errors.Is(err, service.ErrSCIMGroupNotFound) {
			scimError(c, http.StatusNotFound, "Group not found")
			return
		}
		scimError(c, http.StatusInternalServerError, "failed to patch group")
		return
	}

	c.JSON(http.StatusOK, group)
}

// DeleteGroup handles DELETE /scim/v2/Groups/:id
func (h *SCIMHandler) DeleteGroup(c *gin.Context) {
	ctx := c.Request.Context()
	orgID := getSCIMOrgID(c)
	groupID := c.Param("id")

	if err := h.scimService.DeleteGroup(ctx, orgID, groupID); err != nil {
		if errors.Is(err, service.ErrSCIMGroupNotFound) {
			scimError(c, http.StatusNotFound, "Group not found")
			return
		}
		scimError(c, http.StatusInternalServerError, "failed to delete group")
		return
	}

	c.Status(http.StatusNoContent)
}

// scimError returns a SCIM 2.0 compliant error response
func scimError(c *gin.Context, status int, detail string) {
	c.JSON(status, domain.SCIMError{
		Schemas: []string{domain.SCIMSchemaError},
		Detail:  detail,
		Status:  strconv.Itoa(status),
	})
}

func (h *SCIMHandler) ensureOrgAdmin(c *gin.Context, ctx context.Context, userID, orgID uint) bool {
	membership, err := h.orgService.GetMembership(ctx, userID, orgID)
	if err != nil || membership == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "organization access denied"})
		return false
	}
	if !membership.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "organization admin access required"})
		return false
	}
	return true
}
