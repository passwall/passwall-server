package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

const (
	directoryDefaultLimit = 50
	directoryMaxLimit     = 100
	directoryMaxEmailLen  = 254
)

// adminDirectoryUsers reads account rows. Implementations must not load vault items.
type adminDirectoryUsers interface {
	List(ctx context.Context, filter repository.ListFilter) ([]*domain.User, *repository.ListResult, error)
	GetByID(ctx context.Context, id uint) (*domain.User, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

// adminDirectoryActivities reads stored sign-in times. A missing map entry means unknown.
type adminDirectoryActivities interface {
	GetLastSignInTimes(ctx context.Context, userIDs []uint) (map[uint]time.Time, error)
}

// adminDirectorySubscriptions loads effective subscriptions without vault data.
type adminDirectorySubscriptions interface {
	GetEffectiveByOrganizationIDs(ctx context.Context, orgIDs []uint) (map[uint]*domain.Subscription, error)
}

// adminDirectoryUsage reads stored counts and activity timestamps. It must not load vault contents.
type adminDirectoryUsage interface {
	ForUsers(ctx context.Context, users []*domain.User) (map[uint]domain.AdminDirectoryUsage, error)
}

// AdminDirectoryHandler serves the read-only account directory used by external bots.
// Responses contain account metadata and counts only. Vault contents, secrets, tokens, and keys are never loaded.
type AdminDirectoryHandler struct {
	users         adminDirectoryUsers
	activities    adminDirectoryActivities
	subscriptions adminDirectorySubscriptions
	usage         adminDirectoryUsage
	logger        service.Logger
}

// NewAdminDirectoryHandler creates the directory handler.
func NewAdminDirectoryHandler(
	users adminDirectoryUsers,
	activities adminDirectoryActivities,
	subscriptions adminDirectorySubscriptions,
	usage adminDirectoryUsage,
	logger service.Logger,
) *AdminDirectoryHandler {
	return &AdminDirectoryHandler{
		users:         users,
		activities:    activities,
		subscriptions: subscriptions,
		usage:         usage,
		logger:        logger,
	}
}

// ListUsers returns a page of registered accounts.
//
// GET /api/admin/directory/users?limit=&offset=&email=
func (h *AdminDirectoryHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()
	limit, offset := directoryPage(c)

	email, filtered, err := parseDirectoryEmail(c.Query("email"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
		return
	}

	var (
		users []*domain.User
		total int64
	)
	if filtered {
		users, total, err = h.usersByEmail(ctx, email, offset)
	} else {
		var result *repository.ListResult
		users, result, err = h.users.List(ctx, repository.ListFilter{
			Limit:  limit,
			Offset: offset,
			Sort:   "id",
			Order:  "asc",
		})
		if result != nil {
			total = result.Total
		}
	}
	if err != nil {
		h.logError("admin directory: failed to list users", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	items, err := h.directoryUsers(ctx, users)
	if err != nil {
		h.logError("admin directory: failed to load account activity", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	c.JSON(http.StatusOK, domain.AdminDirectoryListResponse{
		SchemaVersion: domain.AdminDirectorySchemaVersion,
		Data:          items,
		Pagination: domain.AdminDirectoryPagination{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	})
}

// GetUser returns one account by numeric id or UUID.
//
// GET /api/admin/directory/users/:id
func (h *AdminDirectoryHandler) GetUser(c *gin.Context) {
	ctx := c.Request.Context()

	user, err := h.lookupUser(ctx, c.Param("id"))
	if err != nil {
		if errors.Is(err, errInvalidDirectoryID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		h.logError("admin directory: failed to fetch user", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}

	items, err := h.directoryUsers(ctx, []*domain.User{user})
	if err != nil {
		h.logError("admin directory: failed to load account activity", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}
	if len(items) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, domain.AdminDirectoryUserResponse{
		SchemaVersion: domain.AdminDirectorySchemaVersion,
		Data:          items[0],
	})
}

func (h *AdminDirectoryHandler) usersByEmail(ctx context.Context, email string, offset int) ([]*domain.User, int64, error) {
	user, err := h.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return []*domain.User{}, 0, nil
		}
		return nil, 0, err
	}
	if offset > 0 {
		return []*domain.User{}, 1, nil
	}
	return []*domain.User{user}, 1, nil
}

func (h *AdminDirectoryHandler) lookupUser(ctx context.Context, rawID string) (*domain.User, error) {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return nil, errInvalidDirectoryID
	}
	if parsed, err := uuid.Parse(rawID); err == nil {
		return h.users.GetByUUID(ctx, parsed.String())
	}
	id, err := strconv.ParseUint(rawID, 10, 32)
	if err != nil || id == 0 {
		return nil, errInvalidDirectoryID
	}
	return h.users.GetByID(ctx, uint(id))
}

func (h *AdminDirectoryHandler) directoryUsers(ctx context.Context, users []*domain.User) ([]domain.AdminDirectoryUser, error) {
	out := make([]domain.AdminDirectoryUser, 0)
	if len(users) == 0 {
		return out, nil
	}

	ids := make([]uint, 0, len(users))
	orgIDs := make([]uint, 0, len(users))
	seenOrg := make(map[uint]struct{}, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		ids = append(ids, user.ID)
		if user.PersonalOrganizationID == 0 {
			continue
		}
		if _, ok := seenOrg[user.PersonalOrganizationID]; ok {
			continue
		}
		seenOrg[user.PersonalOrganizationID] = struct{}{}
		orgIDs = append(orgIDs, user.PersonalOrganizationID)
	}

	logins, err := h.activities.GetLastSignInTimes(ctx, ids)
	if err != nil {
		return nil, err
	}
	subs, err := h.subscriptions.GetEffectiveByOrganizationIDs(ctx, orgIDs)
	if err != nil {
		return nil, err
	}
	if h.usage == nil {
		return nil, errors.New("admin directory usage reader is not configured")
	}
	usageByUser, err := h.usage.ForUsers(ctx, users)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user == nil {
			continue
		}
		var last *time.Time
		if logged, ok := logins[user.ID]; ok {
			last = &logged
		}
		var sub *domain.Subscription
		if user.PersonalOrganizationID != 0 {
			sub = subs[user.PersonalOrganizationID]
		}
		out = append(out, domain.NewAdminDirectoryUser(user, last, sub, usageByUser[user.ID]))
	}
	return out, nil
}

func (h *AdminDirectoryHandler) logError(msg string, err error) {
	if h.logger == nil {
		return
	}
	h.logger.Error(msg, "error", err)
}

func directoryPage(c *gin.Context) (int, int) {
	limit := parseIntWithDefault(c.Query("limit"), directoryDefaultLimit)
	offset := parseIntWithDefault(c.Query("offset"), 0)
	if limit <= 0 {
		limit = directoryDefaultLimit
	}
	if limit > directoryMaxLimit {
		limit = directoryMaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

var errInvalidDirectoryEmail = errors.New("invalid email")
var errInvalidDirectoryID = errors.New("invalid id")

func parseDirectoryEmail(raw string) (string, bool, error) {
	email := strings.TrimSpace(raw)
	if email == "" {
		return "", false, nil
	}
	if len(email) > directoryMaxEmailLen || strings.ContainsAny(email, " \t\r\n") {
		return "", false, errInvalidDirectoryEmail
	}
	return email, true, nil
}
