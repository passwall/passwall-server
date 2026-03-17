package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/logger"
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

	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	var req domain.CreateSSOConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf("SSO CreateConnection bind failed: user_id=%d org_id=%d err=%v", userID, orgID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	conn, err := h.ssoService.CreateConnection(ctx, orgID, userID, &req)
	if err != nil {
		logger.Errorf("SSO CreateConnection failed: user_id=%d org_id=%d protocol=%s domain=%s err=%v", userID, orgID, req.Protocol, req.Domain, err)
		if errors.Is(err, service.ErrSSOProtocolMismatch) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create SSO connection"})
		return
	}

	logger.Infof("SSO CreateConnection success: user_id=%d org_id=%d conn_id=%d protocol=%s domain=%s", userID, orgID, conn.ID, conn.Protocol, conn.Domain)
	c.JSON(http.StatusCreated, domain.ToSSOConnectionDTO(conn))
}

// ListConnections lists SSO connections for an organization
func (h *SSOHandler) ListConnections(c *gin.Context) {
	ctx := c.Request.Context()

	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	userID := GetCurrentUserID(c)
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	conns, err := h.ssoService.ListConnections(ctx, orgID)
	if err != nil {
		logger.Errorf("SSO ListConnections failed: user_id=%d org_id=%d err=%v", userID, orgID, err)
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

	orgID, ok := GetResolvedOrgID(c)
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
		logger.Errorf("SSO GetConnection failed: user_id=%d org_id=%d conn_id=%d err=%v", userID, orgID, connID, err)
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SSO connection"})
		return
	}

	if conn.OrganizationID != orgID {
		logger.Warnf("SSO GetConnection org mismatch: user_id=%d org_id=%d conn_id=%d conn_org_id=%d", userID, orgID, connID, conn.OrganizationID)
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}
	logger.Infof("SSO GetConnection success: user_id=%d org_id=%d conn_id=%d", userID, orgID, connID)
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
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	// Verify connection belongs to this org BEFORE any mutation
	existing, err := h.ssoService.GetConnection(ctx, connID)
	if err != nil || existing.OrganizationID != orgID {
		logger.Warnf("SSO UpdateConnection org mismatch or not found: user_id=%d org_id=%d conn_id=%d err=%v", userID, orgID, connID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}

	var req domain.UpdateSSOConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf("SSO UpdateConnection bind failed: user_id=%d org_id=%d conn_id=%d err=%v", userID, orgID, connID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	conn, err := h.ssoService.UpdateConnection(ctx, connID, userID, &req)
	if err != nil {
		logger.Errorf("SSO UpdateConnection failed: user_id=%d org_id=%d conn_id=%d err=%v", userID, orgID, connID, err)
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update SSO connection"})
		return
	}

	logger.Infof("SSO UpdateConnection success: user_id=%d org_id=%d conn_id=%d", userID, orgID, connID)
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
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}
	conn, err := h.ssoService.GetConnection(ctx, connID)
	if err != nil || conn.OrganizationID != orgID {
		logger.Warnf("SSO DeleteConnection lookup failed: user_id=%d org_id=%d conn_id=%d err=%v", userID, orgID, connID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}
	if err := h.ssoService.DeleteConnection(ctx, connID, userID); err != nil {
		logger.Errorf("SSO DeleteConnection failed: user_id=%d org_id=%d conn_id=%d err=%v", userID, orgID, connID, err)
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete SSO connection"})
		return
	}

	logger.Infof("SSO DeleteConnection success: user_id=%d org_id=%d conn_id=%d", userID, orgID, connID)
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
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		return
	}
	if !h.ensureOrgAdmin(c, ctx, userID, orgID) {
		return
	}

	// Verify connection belongs to this org BEFORE any mutation
	existing, err := h.ssoService.GetConnection(ctx, connID)
	if err != nil || existing.OrganizationID != orgID {
		logger.Warnf("SSO ActivateConnection org mismatch or not found: user_id=%d org_id=%d conn_id=%d err=%v", userID, orgID, connID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}

	conn, err := h.ssoService.ActivateConnection(ctx, connID, userID)
	if err != nil {
		logger.Errorf("SSO ActivateConnection failed: user_id=%d org_id=%d conn_id=%d err=%v", userID, orgID, connID, err)
		if errors.Is(err, service.ErrSSOConnectionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Infof("SSO ActivateConnection success: user_id=%d org_id=%d conn_id=%d", userID, orgID, connID)
	c.JSON(http.StatusOK, domain.ToSSOConnectionDTO(conn))
}

// InitiateLogin starts the SSO authentication flow (public, no auth required)
func (h *SSOHandler) InitiateLogin(c *gin.Context) {
	ctx := c.Request.Context()

	var req domain.SSOInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf("SSO InitiateLogin bind failed: host=%s err=%v", c.Request.Host, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	logger.Infof("SSO InitiateLogin attempt: domain=%s redirect_url_present=%t", req.Domain, strings.TrimSpace(req.RedirectURL) != "")
	redirectURL, err := h.ssoService.InitiateLogin(ctx, &req)
	if err != nil {
		logger.Errorf("SSO InitiateLogin failed: domain=%s err=%v", req.Domain, err)
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

	logger.Infof("SSO InitiateLogin success: domain=%s", req.Domain)
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
		logger.Warnf("SSO OIDCCallback provider error: state=%s error=%s description=%s", state, errParam, errDesc)
		redirectBase := ""
		if state != "" {
			base, err := h.ssoService.GetRedirectURLByState(ctx, state)
			if err == nil {
				redirectBase = base
			}
		}
		h.redirectToVaultCallback(c, redirectBase, false, "", errParam, errDesc)
		return
	}

	var result *domain.SSOCallbackResult
	var err error
	if samlResponse != "" {
		if relayState == "" {
			logger.Warnf("SSO OIDCCallback missing RelayState for SAML callback")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing RelayState parameter"})
			return
		}
		logger.Infof("SSO SAML callback start: relay_state_present=%t", relayState != "")
		result, err = h.ssoService.HandleSAMLCallback(ctx, relayState, samlResponse)
	} else {
		if state == "" || code == "" {
			logger.Warnf("SSO OIDC callback missing state/code: state_present=%t code_present=%t", state != "", code != "")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing state or code parameter"})
			return
		}
		logger.Infof("SSO OIDC callback start: state=%s", state)
		result, err = h.ssoService.HandleOIDCCallback(ctx, state, code)
	}
	if err != nil {
		logger.Errorf("SSO callback failed: state=%s relay_state_present=%t err=%v", state, relayState != "", err)
		redirectBase := ""
		if state != "" {
			base, lookupErr := h.ssoService.GetRedirectURLByState(ctx, state)
			if lookupErr == nil {
				redirectBase = base
			}
		} else if relayState != "" {
			base, lookupErr := h.ssoService.GetRedirectURLByState(ctx, relayState)
			if lookupErr == nil {
				redirectBase = base
			}
		}
		if errors.Is(err, service.ErrSSOInvalidState) {
			h.redirectToVaultCallback(c, redirectBase, false, "", "invalid_state", "invalid or expired SSO state")
			return
		}
		h.redirectToVaultCallback(c, redirectBase, false, "", "sso_callback_failed", "SSO authentication failed")
		return
	}
	userID := uint(0)
	orgID := uint(0)
	if result != nil && result.User != nil {
		userID = result.User.ID
	}
	if result != nil && result.Organization != nil {
		orgID = result.Organization.ID
	}
	logger.Infof("SSO callback success: user_id=%d org_id=%d", userID, orgID)
	payload, err := buildSSOCallbackPayload(result)
	if err != nil {
		logger.Errorf("SSO callback payload encode failed: user_id=%d org_id=%d err=%v", userID, orgID, err)
		h.redirectToVaultCallback(c, result.RedirectURL, false, "", "sso_callback_failed", "SSO authentication succeeded but response packaging failed")
		return
	}
	h.redirectToVaultCallback(c, result.RedirectURL, true, payload, "", "")
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
		logger.Errorf("SSO GetSPMetadata failed: conn_id=%d err=%v", connID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found"})
		return
	}

	c.Header("Content-Type", "application/xml")
	c.String(http.StatusOK, metadata)
}

func buildSSOCallbackPayload(result *domain.SSOCallbackResult) (string, error) {
	data := map[string]interface{}{
		"user":               result.AuthUser,
		"organization":       result.Organization,
		"access_token":       result.AccessToken,
		"refresh_token":      result.RefreshToken,
		"protected_user_key": result.ProtectedUserKey,
		"kdf_config":         result.KdfConfig,
		"redirect_url":       result.RedirectURL,
		"key_escrow_used":    result.KeyEscrowUsed,
	}
	if result.OrgKey != "" {
		data["org_key"] = result.OrgKey
		data["org_id"] = result.OrgID
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (h *SSOHandler) redirectToVaultCallback(
	c *gin.Context,
	redirectBase string,
	success bool,
	payload string,
	errCode string,
	errDesc string,
) {
	target := buildVaultCallbackURL(redirectBase)
	if success {
		q := target.Query()
		q.Set("status", "success")
		target.RawQuery = q.Encode()
		target.Fragment = "payload=" + url.QueryEscape(payload)
		c.Redirect(http.StatusFound, target.String())
		return
	}

	q := target.Query()
	q.Set("status", "error")
	if strings.TrimSpace(errCode) != "" {
		q.Set("error", errCode)
	}
	if strings.TrimSpace(errDesc) != "" {
		q.Set("description", errDesc)
	}
	target.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, target.String())
}

func buildVaultCallbackURL(base string) *url.URL {
	fallback, _ := url.Parse("https://vault.passwall.io/sign-in")
	base = strings.TrimSpace(base)
	if base == "" {
		return fallback
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return fallback
	}
	if !parsed.IsAbs() {
		// Relative redirect path is not trusted here; use fallback.
		return fallback
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fallback
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/sign-in"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed
}

func (h *SSOHandler) ensureOrgAdmin(c *gin.Context, ctx context.Context, userID, orgID uint) bool {
	membership, err := h.orgService.GetMembership(ctx, userID, orgID)
	if err != nil || membership == nil {
		logger.Warnf("SSO org access denied: user_id=%d org_id=%d err=%v", userID, orgID, err)
		c.JSON(http.StatusForbidden, gin.H{"error": "organization access denied"})
		return false
	}
	if !membership.IsAdmin() {
		logger.Warnf("SSO org admin required: user_id=%d org_id=%d role=%s", userID, orgID, membership.Role)
		c.JSON(http.StatusForbidden, gin.H{"error": "organization admin access required"})
		return false
	}
	return true
}
