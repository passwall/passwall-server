package http

import (
	"context"
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
	activityLogger *service.ActivityLogger
	// fallbackRedirect is where the IdP callback returns when the login did
	// not carry a valid client origin (the configured frontend URL).
	fallbackRedirect string
}

// NewSSOHandler creates a new SSO handler
func NewSSOHandler(
	ssoService service.SSOService,
	orgService interface {
		GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error)
	},
	activityLogger *service.ActivityLogger,
	fallbackRedirect string,
) *SSOHandler {
	return &SSOHandler{ssoService: ssoService, orgService: orgService, activityLogger: activityLogger, fallbackRedirect: fallbackRedirect}
}

// respondSSOError maps SSO service errors to stable API codes.
func respondSSOError(c *gin.Context, err error, fallback string) {
	if respondEntitlementError(c, err) {
		return
	}
	type mapping struct {
		target error
		status int
		code   string
	}
	for _, m := range []mapping{
		{service.ErrSSOConnectionNotFound, http.StatusNotFound, "SSO_CONNECTION_NOT_FOUND"},
		{service.ErrSSOInvalidDomain, http.StatusBadRequest, "SSO_INVALID_DOMAIN"},
		{service.ErrSSODomainTaken, http.StatusConflict, "SSO_DOMAIN_TAKEN"},
		{service.ErrSSODomainNotVerified, http.StatusBadRequest, "SSO_DOMAIN_NOT_VERIFIED"},
		{service.ErrSSODefaultRoleNotAllowed, http.StatusBadRequest, "SSO_DEFAULT_ROLE_NOT_ALLOWED"},
		{service.ErrSSOProtocolMismatch, http.StatusBadRequest, "SSO_CONFIG_INVALID"},
		{service.ErrSSOConfigInvalid, http.StatusBadRequest, "SSO_CONFIG_INVALID"},
		{service.ErrSSOInvalidCodeChallenge, http.StatusBadRequest, "SSO_INVALID_CODE_CHALLENGE"},
		{service.ErrSSOInvalidLoginCode, http.StatusUnauthorized, "SSO_INVALID_LOGIN_CODE"},
		{service.ErrSSOConnectionInactive, http.StatusBadRequest, "SSO_CONNECTION_INACTIVE"},
	} {
		if errors.Is(err, m.target) {
			c.JSON(m.status, gin.H{"error": err.Error(), "code": m.code})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
}

func (h *SSOHandler) logOrgActivity(c *gin.Context, userID uint, activityType domain.ActivityType, orgID uint, details service.ActivityDetails) {
	if h.activityLogger == nil {
		return
	}
	if details == nil {
		details = service.ActivityDetails{}
	}
	details[service.ActivityFieldOrganizationID] = orgID
	_ = h.activityLogger.LogActivity(c.Request.Context(), userID, activityType, GetIPAddress(c), c.Request.UserAgent(), details)
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
		logger.Errorf("SSO CreateConnection failed: user_id=%d org_id=%d protocol=%s err=%v", userID, orgID, req.Protocol, err)
		respondSSOError(c, err, "failed to create SSO connection")
		return
	}

	h.logOrgActivity(c, userID, domain.ActivityTypeSSOConnectionChanged, orgID, service.ActivityDetails{"action": "created", "connection_id": conn.ID, "domain": conn.Domain})
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
		respondSSOError(c, err, "failed to update SSO connection")
		return
	}

	h.logOrgActivity(c, userID, domain.ActivityTypeSSOConnectionChanged, orgID, service.ActivityDetails{"action": "updated", "connection_id": conn.ID, "domain": conn.Domain, "status": conn.Status})
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
		respondSSOError(c, err, "failed to delete SSO connection")
		return
	}

	h.logOrgActivity(c, userID, domain.ActivityTypeSSOConnectionChanged, orgID, service.ActivityDetails{"action": "deleted", "connection_id": connID, "domain": conn.Domain})
	c.Status(http.StatusNoContent)
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
		respondSSOError(c, err, "failed to activate SSO connection")
		return
	}

	h.logOrgActivity(c, userID, domain.ActivityTypeSSOConnectionChanged, orgID, service.ActivityDetails{"action": "activated", "connection_id": conn.ID, "domain": conn.Domain})
	c.JSON(http.StatusOK, domain.ToSSOConnectionDTO(conn))
}

// VerifyDomain checks the DNS TXT record for the connection's domain.
func (h *SSOHandler) VerifyDomain(c *gin.Context) {
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
	existing, err := h.ssoService.GetConnection(ctx, connID)
	if err != nil || existing.OrganizationID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO connection not found", "code": "SSO_CONNECTION_NOT_FOUND"})
		return
	}
	wasVerified := existing.IsDomainVerified()

	conn, err := h.ssoService.VerifyDomain(ctx, connID, userID)
	if err != nil {
		if errors.Is(err, service.ErrSSODomainNotVerified) {
			current, _ := h.ssoService.GetConnection(ctx, connID)
			if current == nil {
				current = existing
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "the TXT record was not found; DNS changes can take a while to appear",
				"code":       "SSO_DOMAIN_NOT_VERIFIED",
				"connection": domain.ToSSOConnectionDTO(current),
			})
			return
		}
		respondSSOError(c, err, "failed to verify domain")
		return
	}
	if !wasVerified {
		h.logOrgActivity(c, userID, domain.ActivityTypeSSODomainVerified, orgID, service.ActivityDetails{"connection_id": conn.ID, "domain": conn.Domain})
	}
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

	redirectURL, err := h.ssoService.InitiateLogin(ctx, &req)
	if err != nil {
		if errors.Is(err, service.ErrSSOConnectionNotFound) || errors.Is(err, service.ErrSSOConnectionInactive) {
			// One answer for unknown and inactive domains.
			c.JSON(http.StatusNotFound, gin.H{"error": "no SSO connection found for this domain", "code": "SSO_CONNECTION_NOT_FOUND"})
			return
		}
		logger.Errorf("SSO InitiateLogin failed: err=%v", err)
		respondSSOError(c, err, "failed to initiate SSO login")
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

	// Resolve the client redirect base BEFORE processing, because the SSO state
	// (which carries the redirect URL) is single-use and gets deleted during
	// callback handling. Looking it up afterwards on the error path would fail
	// and send the user to the default vault URL instead of their origin.
	redirectBase := ""
	if stateLookup := state; stateLookup != "" {
		if base, err := h.ssoService.GetRedirectURLByState(ctx, stateLookup); err == nil {
			redirectBase = base
		}
	} else if relayState != "" {
		if base, err := h.ssoService.GetRedirectURLByState(ctx, relayState); err == nil {
			redirectBase = base
		}
	}

	if errParam != "" {
		errDesc := c.Query("error_description")
		logger.Warnf("SSO callback provider error: error=%s", errParam)
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
		result, err = h.ssoService.HandleSAMLCallback(ctx, relayState, samlResponse)
	} else {
		if state == "" || code == "" {
			logger.Warnf("SSO OIDC callback missing state/code: state_present=%t code_present=%t", state != "", code != "")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing state or code parameter"})
			return
		}
		result, err = h.ssoService.HandleOIDCCallback(ctx, state, code)
	}
	if err != nil {
		logger.Errorf("SSO callback failed: err=%v", err)
		switch {
		case errors.Is(err, service.ErrSSOInvalidState):
			h.redirectToVaultCallback(c, redirectBase, false, "", "invalid_state", "invalid or expired SSO state")
		case errors.Is(err, service.ErrSSOUserNotFound):
			h.redirectToVaultCallback(c, redirectBase, false, "", "user_not_found", "create a Passwall account with this email first")
		case errors.Is(err, service.ErrSSONotMember):
			h.redirectToVaultCallback(c, redirectBase, false, "", "not_member", "you are not a member of this organization")
		case errors.Is(err, service.ErrSSOMembershipInactive):
			h.redirectToVaultCallback(c, redirectBase, false, "", "membership_inactive", "your organization membership is not active")
		default:
			h.redirectToVaultCallback(c, redirectBase, false, "", "sso_callback_failed", "SSO authentication failed")
		}
		return
	}
	if result.Provisioned {
		h.logOrgActivity(c, result.UserID, domain.ActivityTypeSSOMemberProvisioned, result.OrgID, nil)
	}
	h.redirectToVaultCallback(c, result.RedirectURL, true, result.Code, "", "")
}

// ExchangeLoginCode trades the single-use code from the callback for a
// session (public; the PKCE verifier proves the caller started the login).
func (h *SSOHandler) ExchangeLoginCode(c *gin.Context) {
	var req domain.SSOExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	resp, err := h.ssoService.ExchangeLoginCode(c.Request.Context(), &req)
	if err != nil {
		logger.Warnf("SSO code exchange failed: err=%v", err)
		if errors.Is(err, service.ErrDeviceLimit) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error(), "code": "DEVICE_LIMIT_REACHED"})
			return
		}
		respondSSOError(c, err, "failed to complete SSO sign-in")
		return
	}
	if resp.AuthResponse != nil && !resp.TwoFactorRequired && resp.User != nil && resp.Organization != nil {
		h.logOrgActivity(c, resp.User.ID, domain.ActivityTypeSSOSignIn, resp.Organization.ID, nil)
	}
	c.JSON(http.StatusOK, resp)
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

func (h *SSOHandler) redirectToVaultCallback(
	c *gin.Context,
	redirectBase string,
	success bool,
	code string,
	errCode string,
	errDesc string,
) {
	target := buildVaultCallbackURL(redirectBase, h.fallbackRedirect)
	if success {
		q := target.Query()
		q.Set("status", "success")
		target.RawQuery = q.Encode()
		// Fragment: never sent to servers or logged by proxies.
		target.Fragment = "code=" + url.QueryEscape(code)
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

func buildVaultCallbackURL(base, fallbackBase string) *url.URL {
	fallback, err := url.Parse(strings.TrimRight(strings.TrimSpace(fallbackBase), "/") + "/sign-in")
	if err != nil || !fallback.IsAbs() {
		fallback, _ = url.Parse("https://vault.passwall.io/sign-in")
	}
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
	if membership.Status != domain.OrgUserStatusAccepted && membership.Status != domain.OrgUserStatusConfirmed {
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
