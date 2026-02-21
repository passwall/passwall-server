package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
)

// SSOHandler handles SSO connection management and authentication flows
type SSOHandler struct {
	ssoService service.SSOService
	orgService interface {
		GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error)
	}
}

// NewSSOHandler creates a new SSO handler
func NewSSOHandler(
	ssoService service.SSOService,
	orgService interface {
		GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error)
	},
) *SSOHandler {
	return &SSOHandler{ssoService: ssoService, orgService: orgService}
}

// CreateConnection creates a new SSO connection for an organization
func (h *SSOHandler) CreateConnection(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	var req domain.CreateSSOConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	conn, err := h.ssoService.CreateConnection(ctx, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, service.ErrSSOProtocolMismatch) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create SSO connection"})
		return
	}

	c.JSON(http.StatusCreated, domain.ToSSOConnectionDTO(conn))
}

// ListConnections lists SSO connections for an organization
func (h *SSOHandler) ListConnections(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	conns, err := h.ssoService.ListConnections(ctx, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list SSO connections"})
		return
	}

	dtos := make([]*domain.SSOConnectionDTO, len(conns))
	for i, conn := range conns {
		dtos[i] = domain.ToSSOConnectionDTO(conn)
	}

	c.JSON(http.StatusOK, dtos)
}

// GetConnection gets a specific SSO connection
func (h *SSOHandler) GetConnection(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}
	connID, ok := GetUintParam(c, "connId")
	if !ok {
		return
	}

	conn, err := h.ssoService.GetConnection(ctx, connID)
	if err != nil {
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SSO connection"})
		return
	}

	if conn.OrganizationID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}
	c.JSON(http.StatusOK, domain.ToSSOConnectionDTO(conn))
}

// UpdateConnection updates an SSO connection
func (h *SSOHandler) UpdateConnection(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	connID, ok := GetUintParam(c, "connId")
	if !ok {
		return
	}
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	var req domain.UpdateSSOConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	conn, err := h.ssoService.UpdateConnection(ctx, connID, userID, &req)
	if err != nil {
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update SSO connection"})
		return
	}

	if conn.OrganizationID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}
	c.JSON(http.StatusOK, domain.ToSSOConnectionDTO(conn))
}

// DeleteConnection deletes an SSO connection
func (h *SSOHandler) DeleteConnection(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	connID, ok := GetUintParam(c, "connId")
	if !ok {
		return
	}
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}
	conn, err := h.ssoService.GetConnection(ctx, connID)
	if err != nil || conn.OrganizationID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}
	if err := h.ssoService.DeleteConnection(ctx, connID, userID); err != nil {
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete SSO connection"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ActivateConnection activates an SSO connection (validates config first)
func (h *SSOHandler) ActivateConnection(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	connID, ok := GetUintParam(c, "connId")
	if !ok {
		return
	}
	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	conn, err := h.ssoService.ActivateConnection(ctx, connID, userID)
	if err != nil {
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if conn.OrganizationID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}
	c.JSON(http.StatusOK, domain.ToSSOConnectionDTO(conn))
}

// InitiateLogin starts the SSO authentication flow (public, no auth required)
func (h *SSOHandler) InitiateLogin(c *gin.Context) {
	ctx := c.Request.Context()

	var req domain.SSOInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	baseURL := getBaseURL(c)
	redirectURL, err := h.ssoService.InitiateLogin(ctx, &req, baseURL)
	if err != nil {
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no SSO connection found for this domain"})
			return
		}
		if errors.Is(err, service.ErrSSOConnectionInactive) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SSO connection is not active"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initiate SSO login"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"redirect_url": redirectURL,
		"auth_url":     redirectURL,
	})
}

// OIDCCallback handles OIDC/SAML callback from IdP.
func (h *SSOHandler) OIDCCallback(c *gin.Context) {
	ctx := c.Request.Context()

	state := c.Query("state")
	code := c.Query("code")
	samlResponse := c.PostForm("SAMLResponse")
	if samlResponse == "" {
		samlResponse = c.Query("SAMLResponse")
	}
	relayState := c.PostForm("RelayState")
	if relayState == "" {
		relayState = c.Query("RelayState")
	}
	errParam := c.Query("error")

	if errParam != "" {
		errDesc := c.Query("error_description")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       errParam,
			"description": errDesc,
		})
		return
	}

	var result *domain.SSOCallbackResult
	var err error
	if samlResponse != "" {
		if relayState == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing RelayState parameter"})
			return
		}
		result, err = h.ssoService.HandleSAMLCallback(ctx, relayState, samlResponse)
	} else {
		if state == "" || code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing state or code parameter"})
			return
		}
		result, err = h.ssoService.HandleOIDCCallback(ctx, state, code)
	}
	if err != nil {
		if errors.Is(err, service.ErrSSOInvalidState) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired SSO state"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "SSO authentication failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetSPMetadata returns SAML SP metadata for a connection
func (h *SSOHandler) GetSPMetadata(c *gin.Context) {
	ctx := c.Request.Context()

	connID, ok := GetUintParam(c, "connId")
	if !ok {
		return
	}

	metadata, err := h.ssoService.GetSPMetadata(ctx, connID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}

	c.Header("Content-Type", "application/xml")
	c.String(http.StatusOK, metadata)
}

func getBaseURL(c *gin.Context) string {
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + c.Request.Host
}

func (h *SSOHandler) ensureOrgAdmin(c *gin.Context, ctx context.Context, userID, orgID uint) bool {
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
