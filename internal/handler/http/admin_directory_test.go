package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAdminDirectoryKey = "test-admin-directory-key"

type stubDirectoryUsers struct {
	listFilter repository.ListFilter
	listUsers  []*domain.User
	listTotal  int64
	listErr    error

	byID    map[uint]*domain.User
	byUUID  map[string]*domain.User
	byEmail map[string]*domain.User
	getErr  error
}

func (s *stubDirectoryUsers) List(_ context.Context, filter repository.ListFilter) ([]*domain.User, *repository.ListResult, error) {
	s.listFilter = filter
	if s.listErr != nil {
		return nil, nil, s.listErr
	}
	return s.listUsers, &repository.ListResult{Total: s.listTotal, Filtered: s.listTotal}, nil
}

func (s *stubDirectoryUsers) GetByID(_ context.Context, id uint) (*domain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	user, ok := s.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return user, nil
}

func (s *stubDirectoryUsers) GetByUUID(_ context.Context, id string) (*domain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	user, ok := s.byUUID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return user, nil
}

func (s *stubDirectoryUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	user, ok := s.byEmail[strings.ToLower(email)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return user, nil
}

type stubDirectoryActivities struct {
	times map[uint]time.Time
	err   error
	seen  []uint
}

func (s *stubDirectoryActivities) GetLastSignInTimes(_ context.Context, userIDs []uint) (map[uint]time.Time, error) {
	s.seen = append([]uint(nil), userIDs...)
	if s.err != nil {
		return nil, s.err
	}
	if s.times == nil {
		return map[uint]time.Time{}, nil
	}
	return s.times, nil
}

type stubDirectorySubs struct {
	byOrg map[uint]*domain.Subscription
	err   error
	seen  []uint
}

func (s *stubDirectorySubs) GetEffectiveByOrganizationIDs(_ context.Context, orgIDs []uint) (map[uint]*domain.Subscription, error) {
	s.seen = append([]uint(nil), orgIDs...)
	if s.err != nil {
		return nil, s.err
	}
	if s.byOrg == nil {
		return map[uint]*domain.Subscription{}, nil
	}
	return s.byOrg, nil
}

type stubDirectoryStats struct {
	byUser map[uint]domain.AdminDirectorySignals
	err    error
	users  []*domain.User
}

func (s *stubDirectoryStats) LoadAccountSignals(_ context.Context, users []*domain.User, _ time.Time) (map[uint]domain.AdminDirectorySignals, error) {
	if s == nil {
		return map[uint]domain.AdminDirectorySignals{}, nil
	}
	s.users = users
	if s.err != nil {
		return nil, s.err
	}
	if s.byUser == nil {
		return map[uint]domain.AdminDirectorySignals{}, nil
	}
	return s.byUser, nil
}

func newDirectoryTestRouter(users *stubDirectoryUsers, activities *stubDirectoryActivities, subs *stubDirectorySubs, stats *stubDirectoryStats, key string) *gin.Engine {
	if stats == nil {
		stats = &stubDirectoryStats{}
	}
	handler := NewAdminDirectoryHandler(users, activities, subs, stats, nil)
	router := gin.New()
	group := router.Group("/api/admin/directory")
	group.Use(AdminServiceAuthMiddleware(key))
	group.GET("/users", handler.ListUsers)
	group.GET("/users/:id", handler.GetUser)
	return router
}

func directoryRequest(t *testing.T, router http.Handler, method, path, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload), string(body))
	return payload
}

func walkJSONKeys(value any, visit func(key string)) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			visit(key)
			walkJSONKeys(child, visit)
		}
	case []any:
		for _, child := range typed {
			walkJSONKeys(child, visit)
		}
	}
}

func assertNoVaultFields(t *testing.T, payload map[string]any, secretMarkers ...string) {
	t.Helper()
	forbidden := map[string]struct{}{
		"master_password_hash": {},
		"protected_user_key":   {},
		"kdf_salt":             {},
		"secret":               {},
		"token":                {},
		"private_key":          {},
		"rsa_private_key_enc":  {},
		"two_factor_secret":    {},
		"recovery_codes":       {},
		"schema":               {},
		"items":                {},
		"notes":                {},
		"card_number":          {},
		"account_number":       {},
		"ciphertext":           {},
		"stripe_customer_id":   {},
		"metadata":             {},
		"title":                {},
		"username":             {},
		"uri":                  {},
		"uri_hint":             {},
		"url":                  {},
		"device_id":            {},
		"session_uuid":         {},
		"ip_address":           {},
		"user_agent":           {},
		"details":              {},
	}
	walkJSONKeys(payload, func(key string) {
		_, blocked := forbidden[strings.ToLower(key)]
		assert.Falsef(t, blocked, "response included forbidden field %q", key)
	})

	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	body := string(encoded)
	for _, marker := range secretMarkers {
		assert.NotContains(t, body, marker)
	}
}

func sampleDirectoryUser() *domain.User {
	return &domain.User{
		ID:                     7,
		UUID:                   uuid.MustParse("6f1c4c3e-1b2a-4d5e-8f70-112233445566"),
		CreatedAt:              time.Date(2024, 3, 2, 15, 4, 5, 0, time.FixedZone("UTC+3", 3*60*60)),
		Email:                  "ada@example.com",
		Name:                   "Ada",
		Schema:                 "user_7_vault",
		PersonalOrganizationID: 42,
		MasterPasswordHash:     "bcrypt-hash-do-not-leak",
		ProtectedUserKey:       "2.iv|ciphertext|mac",
		KdfSalt:                "salthex-do-not-leak",
		TwoFactorSecret:        stringPtr("totp-secret-do-not-leak"),
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestAdminDirectory_Auth(t *testing.T) {
	t.Parallel()

	users := &stubDirectoryUsers{}
	activities := &stubDirectoryActivities{}
	subs := &stubDirectorySubs{}

	t.Run("disabled when credential is unset", func(t *testing.T) {
		t.Parallel()
		router := newDirectoryTestRouter(users, activities, subs, nil, "  ")
		rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users", "anything")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		payload := decodeJSON(t, rec)
		assert.Equal(t, "admin directory api is disabled", payload["error"])
	})

	t.Run("rejects missing and wrong credentials", func(t *testing.T) {
		t.Parallel()
		router := newDirectoryTestRouter(users, activities, subs, nil, testAdminDirectoryKey)

		missing := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users", "")
		assert.Equal(t, http.StatusUnauthorized, missing.Code)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/directory/users", nil)
		req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.user-session")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid or missing admin credential")
	})

	t.Run("rejects non-bearer authorization", func(t *testing.T) {
		t.Parallel()
		router := newDirectoryTestRouter(users, activities, subs, nil, testAdminDirectoryKey)
		req := httptest.NewRequest(http.MethodGet, "/api/admin/directory/users", nil)
		req.Header.Set("Authorization", "Basic "+testAdminDirectoryKey)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAdminDirectory_ListUsers(t *testing.T) {
	t.Parallel()

	registered := time.Date(2024, 3, 2, 12, 4, 5, 0, time.UTC)
	lastLogin := time.Date(2026, 1, 9, 8, 0, 0, 0, time.UTC)
	user := sampleDirectoryUser()
	user.CreatedAt = registered

	users := &stubDirectoryUsers{
		listUsers: []*domain.User{user},
		listTotal: 12,
	}
	activities := &stubDirectoryActivities{times: map[uint]time.Time{user.ID: lastLogin}}
	subs := &stubDirectorySubs{byOrg: map[uint]*domain.Subscription{
		42: {
			State: domain.SubStateActive,
			Plan:  &domain.Plan{Code: "pro-monthly", Name: "Pro"},
		},
	}}

	router := newDirectoryTestRouter(users, activities, subs, nil, testAdminDirectoryKey)
	rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users?limit=500&offset=-4", testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	payload := decodeJSON(t, rec)
	assert.EqualValues(t, domain.AdminDirectorySchemaVersion, payload["schema_version"])
	pagination := payload["pagination"].(map[string]any)
	assert.EqualValues(t, 100, pagination["limit"])
	assert.EqualValues(t, 0, pagination["offset"])
	assert.EqualValues(t, 12, pagination["total"])
	assert.Equal(t, 100, users.listFilter.Limit)
	assert.Equal(t, 0, users.listFilter.Offset)
	assert.Equal(t, "id", users.listFilter.Sort)
	assert.Equal(t, "asc", users.listFilter.Order)

	data := payload["data"].([]any)
	require.Len(t, data, 1)
	record := data[0].(map[string]any)
	assert.EqualValues(t, 7, record["id"])
	assert.Equal(t, user.UUID.String(), record["uuid"])
	assert.Equal(t, "ada@example.com", record["email"])
	assert.Equal(t, registered.Format(time.RFC3339Nano), record["registered_at"])
	assert.Equal(t, lastLogin.Format(time.RFC3339Nano), record["last_login_at"])

	plan := record["plan"].(map[string]any)
	assert.Equal(t, "pro-monthly", plan["code"])
	assert.Equal(t, "Pro", plan["name"])
	subscription := record["subscription"].(map[string]any)
	assert.Equal(t, "active", subscription["status"])
	assert.Len(t, plan, 2)
	assert.Len(t, subscription, 1)

	assertNoVaultFields(t, payload,
		"bcrypt-hash-do-not-leak",
		"2.iv|ciphertext|mac",
		"salthex-do-not-leak",
		"totp-secret-do-not-leak",
		"user_7_vault",
	)
}

func TestAdminDirectory_UnknownActivityAndPlan(t *testing.T) {
	t.Parallel()

	user := sampleDirectoryUser()
	user.PersonalOrganizationID = 0
	users := &stubDirectoryUsers{listUsers: []*domain.User{user}, listTotal: 1}
	router := newDirectoryTestRouter(users, &stubDirectoryActivities{}, &stubDirectorySubs{}, nil, testAdminDirectoryKey)

	rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users", testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeJSON(t, rec)
	record := payload["data"].([]any)[0].(map[string]any)
	assert.Nil(t, record["last_login_at"])
	assert.Nil(t, record["plan"])
	subscription := record["subscription"].(map[string]any)
	assert.Equal(t, domain.AdminDirectorySubscriptionUnknown, subscription["status"])
}

func TestAdminDirectory_SubscriptionWithoutPlan(t *testing.T) {
	t.Parallel()

	user := sampleDirectoryUser()
	users := &stubDirectoryUsers{listUsers: []*domain.User{user}, listTotal: 1}
	subs := &stubDirectorySubs{byOrg: map[uint]*domain.Subscription{
		user.PersonalOrganizationID: {State: domain.SubStatePastDue},
	}}
	router := newDirectoryTestRouter(users, &stubDirectoryActivities{}, subs, nil, testAdminDirectoryKey)

	rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users", testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, rec.Code)
	record := decodeJSON(t, rec)["data"].([]any)[0].(map[string]any)
	assert.Nil(t, record["plan"])
	assert.Equal(t, "past_due", record["subscription"].(map[string]any)["status"])
}

func TestAdminDirectory_EmailFilter(t *testing.T) {
	t.Parallel()

	user := sampleDirectoryUser()
	users := &stubDirectoryUsers{byEmail: map[string]*domain.User{"ada@example.com": user}}
	router := newDirectoryTestRouter(users, &stubDirectoryActivities{}, &stubDirectorySubs{}, nil, testAdminDirectoryKey)

	rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users?email=Ada@Example.com", testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeJSON(t, rec)
	assert.EqualValues(t, 1, payload["pagination"].(map[string]any)["total"])
	require.Len(t, payload["data"].([]any), 1)
	assert.Equal(t, repository.ListFilter{}, users.listFilter)

	missing := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users?email=missing@example.com", testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, missing.Code)
	missingPayload := decodeJSON(t, missing)
	assert.Empty(t, missingPayload["data"].([]any))
	assert.EqualValues(t, 0, missingPayload["pagination"].(map[string]any)["total"])

	invalid := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users?email=not%20an%20email", testAdminDirectoryKey)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
}

func TestAdminDirectory_GetUser(t *testing.T) {
	t.Parallel()

	user := sampleDirectoryUser()
	users := &stubDirectoryUsers{
		byID:   map[uint]*domain.User{user.ID: user},
		byUUID: map[string]*domain.User{user.UUID.String(): user},
	}
	activities := &stubDirectoryActivities{times: map[uint]time.Time{user.ID: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)}}
	router := newDirectoryTestRouter(users, activities, &stubDirectorySubs{}, nil, testAdminDirectoryKey)

	byID := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users/7", testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, byID.Code)
	payload := decodeJSON(t, byID)
	assert.EqualValues(t, 1, payload["schema_version"])
	_, hasPagination := payload["pagination"]
	assert.False(t, hasPagination)
	assert.Equal(t, "ada@example.com", payload["data"].(map[string]any)["email"])
	assertNoVaultFields(t, payload, "bcrypt-hash-do-not-leak", "2.iv|ciphertext|mac")

	byUUID := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users/"+user.UUID.String(), testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, byUUID.Code)

	missing := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users/99", testAdminDirectoryKey)
	assert.Equal(t, http.StatusNotFound, missing.Code)

	invalid := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users/not-an-id", testAdminDirectoryKey)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
}

func TestAdminDirectory_StoreErrors(t *testing.T) {
	t.Parallel()

	users := &stubDirectoryUsers{listErr: errors.New("db down")}
	router := newDirectoryTestRouter(users, &stubDirectoryActivities{}, &stubDirectorySubs{}, nil, testAdminDirectoryKey)
	rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users", testAdminDirectoryKey)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "db down")

	user := sampleDirectoryUser()
	users = &stubDirectoryUsers{listUsers: []*domain.User{user}, listTotal: 1}
	activities := &stubDirectoryActivities{err: errors.New("activity store unavailable")}
	router = newDirectoryTestRouter(users, activities, &stubDirectorySubs{}, nil, testAdminDirectoryKey)
	rec = directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users", testAdminDirectoryKey)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "bcrypt-hash-do-not-leak")
	assert.NotContains(t, rec.Body.String(), user.Email)
}

func TestNewAdminDirectoryUser_DoesNotCopySecrets(t *testing.T) {
	t.Parallel()

	user := sampleDirectoryUser()
	login := time.Date(2026, 2, 2, 3, 4, 5, 0, time.UTC)
	active := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	dto := domain.NewAdminDirectoryUser(user, &login, &domain.Subscription{
		State: domain.SubStateTrialing,
		Plan:  &domain.Plan{Code: "family-yearly", Name: "Family"},
	}, domain.AdminDirectorySignals{
		LastActivityAt:    &active,
		VaultKnown:        true,
		VaultByType:       map[domain.ItemType]int{domain.ItemTypePassword: 2, domain.ItemTypeSecureNote: 1},
		OrgItemsByType:    map[domain.ItemType]int{domain.ItemTypeCard: 4},
		OrganizationCount: 1,
		CollectionCount:   2,
		SignInCount:       3,
		ActivityCount:     8,
		DeviceCount:       1,
		ClientCount:       1,
	})

	encoded, err := json.Marshal(dto)
	require.NoError(t, err)
	body := string(encoded)
	assert.NotContains(t, body, user.MasterPasswordHash)
	assert.NotContains(t, body, user.ProtectedUserKey)
	assert.NotContains(t, body, user.KdfSalt)
	assert.NotContains(t, body, *user.TwoFactorSecret)
	assert.NotContains(t, body, user.Schema)
	assert.Contains(t, body, `"status":"trialing"`)
	assert.Contains(t, body, `"code":"family-yearly"`)
	assert.Contains(t, body, `"email_verified":false`)
	assert.Contains(t, body, `"item_count":3`)
	assert.Contains(t, body, `"password":2`)
	assert.Contains(t, body, `"secure_note":1`)
	assert.Contains(t, body, `"card":4`)
	assert.Contains(t, body, `"organization_count":1`)
	assert.Contains(t, body, `"signin_count":3`)
	assert.Contains(t, body, `"activity_count":8`)
	assert.NotContains(t, body, "device-")
	assert.NotContains(t, body, user.Name)
}

func TestAdminDirectory_UsageCounts(t *testing.T) {
	t.Parallel()

	user := sampleDirectoryUser()
	user.IsVerified = true
	lastActivity := time.Date(2026, 4, 1, 9, 30, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	users := &stubDirectoryUsers{listUsers: []*domain.User{user}, listTotal: 1}
	stats := &stubDirectoryStats{byUser: map[uint]domain.AdminDirectorySignals{
		user.ID: {
			LastActivityAt: &lastActivity,
			VaultKnown:     true,
			VaultByType: map[domain.ItemType]int{
				domain.ItemTypePassword:   8,
				domain.ItemTypeSecureNote: 3,
				domain.ItemTypeCard:       1,
				domain.ItemType(77):       2,
			},
			OrgItemsByType:    map[domain.ItemType]int{domain.ItemTypePassword: 4},
			OrganizationCount: 2,
			CollectionCount:   3,
			SignInCount:       5,
			ActivityCount:     11,
			DeviceCount:       2,
			ClientCount:       2,
		},
	}}
	router := newDirectoryTestRouter(users, &stubDirectoryActivities{}, &stubDirectorySubs{}, stats, testAdminDirectoryKey)

	rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users", testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodeJSON(t, rec)
	assert.EqualValues(t, 1, payload["schema_version"])
	record := payload["data"].([]any)[0].(map[string]any)
	assert.Equal(t, true, record["email_verified"])
	assert.Equal(t, lastActivity.UTC().Format(time.RFC3339Nano), record["last_activity_at"])
	assert.Nil(t, record["last_login_at"])

	vault := record["vault"].(map[string]any)
	assert.EqualValues(t, 14, vault["item_count"])
	byType := vault["by_type"].(map[string]any)
	assert.EqualValues(t, 8, byType["password"])
	assert.EqualValues(t, 3, byType["secure_note"])
	assert.EqualValues(t, 1, byType["card"])
	assert.EqualValues(t, 2, byType["unknown"])
	assert.Len(t, byType, 4)

	orgItems := record["organization_items"].(map[string]any)
	assert.EqualValues(t, 4, orgItems["item_count"])
	assert.EqualValues(t, 4, orgItems["by_type"].(map[string]any)["password"])
	assert.EqualValues(t, 2, record["organization_count"])
	assert.EqualValues(t, 3, record["collection_count"])
	assert.EqualValues(t, 5, record["signin_count"])
	assert.EqualValues(t, 11, record["activity_count"])
	assert.EqualValues(t, 2, record["device_count"])
	assert.EqualValues(t, 2, record["client_count"])
	assertNoVaultFields(t, payload, "bcrypt-hash-do-not-leak", "2.iv|ciphertext|mac", "Ada")
	require.Len(t, stats.users, 1)
	assert.Equal(t, user.ID, stats.users[0].ID)
}

func TestAdminDirectory_UnknownUsageIsNullOrZero(t *testing.T) {
	t.Parallel()

	user := sampleDirectoryUser()
	users := &stubDirectoryUsers{
		byID: map[uint]*domain.User{user.ID: user},
	}
	router := newDirectoryTestRouter(users, &stubDirectoryActivities{}, &stubDirectorySubs{}, &stubDirectoryStats{}, testAdminDirectoryKey)

	rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users/7", testAdminDirectoryKey)
	require.Equal(t, http.StatusOK, rec.Code)
	record := decodeJSON(t, rec)["data"].(map[string]any)
	assert.Equal(t, false, record["email_verified"])
	assert.Nil(t, record["last_activity_at"])
	vault := record["vault"].(map[string]any)
	assert.Nil(t, vault["item_count"])
	assert.Nil(t, vault["by_type"])
	orgItems := record["organization_items"].(map[string]any)
	assert.EqualValues(t, 0, orgItems["item_count"])
	assert.Empty(t, orgItems["by_type"].(map[string]any))
	assert.EqualValues(t, 0, record["organization_count"])
	assert.EqualValues(t, 0, record["collection_count"])
	assert.EqualValues(t, 0, record["signin_count"])
	assert.EqualValues(t, 0, record["activity_count"])
	assert.EqualValues(t, 0, record["device_count"])
	assert.EqualValues(t, 0, record["client_count"])
}

func TestAdminDirectory_StatsErrorDoesNotLeakAccount(t *testing.T) {
	t.Parallel()

	user := sampleDirectoryUser()
	users := &stubDirectoryUsers{listUsers: []*domain.User{user}, listTotal: 1}
	stats := &stubDirectoryStats{err: errors.New("stats down")}
	router := newDirectoryTestRouter(users, &stubDirectoryActivities{}, &stubDirectorySubs{}, stats, testAdminDirectoryKey)

	rec := directoryRequest(t, router, http.MethodGet, "/api/admin/directory/users", testAdminDirectoryKey)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "stats down")
	assert.NotContains(t, rec.Body.String(), user.Email)
	assert.NotContains(t, rec.Body.String(), "bcrypt-hash-do-not-leak")
}
