package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/authz"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubOrganizationItemService struct {
	createCalled bool
	createErr    error
	createItem   *domain.OrganizationItem
	updateErr    error
	updateItem   *domain.OrganizationItem
	deleteErr    error
	deleteItem   *domain.OrganizationItem
	v2Request    service.OrganizationItemsV2Request
	v2Response   *service.OrganizationItemsV2Response
	v2Err        error
}

func (s *stubOrganizationItemService) Create(ctx context.Context, orgID, userID uint, req *service.CreateOrgItemRequest) (*domain.OrganizationItem, error) {
	s.createCalled = true
	if s.createErr != nil {
		return nil, s.createErr
	}
	return s.createItem, nil
}

func (s *stubOrganizationItemService) GetByID(ctx context.Context, id, userID uint) (*domain.OrganizationItem, error) {
	return nil, repository.ErrNotFound
}

func (s *stubOrganizationItemService) ListByOrganization(ctx context.Context, orgID, userID uint, filter repository.OrganizationItemFilter) ([]*domain.OrganizationItem, int64, error) {
	return nil, 0, nil
}

func (s *stubOrganizationItemService) ListV2(ctx context.Context, orgID, userID uint, req service.OrganizationItemsV2Request) (*service.OrganizationItemsV2Response, error) {
	s.v2Request = req
	if s.v2Err != nil {
		return nil, s.v2Err
	}
	if s.v2Response != nil {
		return s.v2Response, nil
	}
	return &service.OrganizationItemsV2Response{Items: []*domain.OrganizationItemSyncDTO{}}, nil
}

func (s *stubOrganizationItemService) ListByCollection(ctx context.Context, collectionID, userID uint) ([]*domain.OrganizationItem, error) {
	return nil, nil
}

func (s *stubOrganizationItemService) Update(ctx context.Context, id, userID uint, req *service.UpdateOrgItemRequest) (*domain.OrganizationItem, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return s.updateItem, nil
}

func (s *stubOrganizationItemService) Delete(ctx context.Context, id, userID uint) (*domain.OrganizationItem, error) {
	if s.deleteErr != nil {
		return nil, s.deleteErr
	}
	return s.deleteItem, nil
}

func (s *stubOrganizationItemService) GetCollectionAccess(ctx context.Context, orgID, userID, collectionID uint) (*authz.CollectionAccess, error) {
	return &authz.CollectionAccess{CanRead: true}, nil
}

func (s *stubOrganizationItemService) GetAutofillSecret(ctx context.Context, itemID, userID uint) (*domain.OrganizationItem, error) {
	return nil, repository.ErrNotFound
}

type stubPolicyEnforcementService struct {
	checkCardCalled bool
	checkCardErr    error
}

func (s *stubPolicyEnforcementService) CheckTwoFactorRequired(ctx context.Context, orgID uint, userHas2FA bool) error {
	return nil
}

func (s *stubPolicyEnforcementService) GetMasterPasswordRequirements(ctx context.Context, orgID uint) (*service.MasterPasswordPolicy, error) {
	return nil, nil
}

func (s *stubPolicyEnforcementService) GetPasswordGeneratorRequirements(ctx context.Context, orgID uint) (*service.PasswordGeneratorPolicy, error) {
	return nil, nil
}

func (s *stubPolicyEnforcementService) CheckExternalSharingAllowed(ctx context.Context, orgID uint) error {
	return nil
}

func (s *stubPolicyEnforcementService) CheckPersonalExportAllowed(ctx context.Context, orgID uint, role domain.OrganizationRole) error {
	return nil
}

func (s *stubPolicyEnforcementService) CheckSendAllowed(ctx context.Context, orgID uint, role domain.OrganizationRole) error {
	return nil
}

func (s *stubPolicyEnforcementService) GetSessionTimeoutPolicy(ctx context.Context, orgID uint) (*service.SessionTimeoutPolicy, error) {
	return nil, nil
}

func (s *stubPolicyEnforcementService) CheckCardTypeAllowed(ctx context.Context, orgID uint) error {
	s.checkCardCalled = true
	return s.checkCardErr
}

func (s *stubPolicyEnforcementService) CheckPersonalVaultAllowed(ctx context.Context, orgID uint, role domain.OrganizationRole) error {
	return nil
}

func (s *stubPolicyEnforcementService) GetPasswordExpirationPolicy(ctx context.Context, orgID uint) (*service.PasswordExpirationPolicy, error) {
	return nil, nil
}

func TestOrganizationItemHandler_ListV2(t *testing.T) {
	t.Parallel()
	itemSvc := &stubOrganizationItemService{
		v2Response: &service.OrganizationItemsV2Response{
			Items: []*domain.OrganizationItemSyncDTO{
				{ID: 1, UUID: uuid.New(), OrganizationID: 99, Revision: 12, Deleted: true},
			},
			Revision: 12,
		},
	}
	handler := &OrganizationItemHandler{service: itemSvc}
	router := gin.New()
	router.GET("/api/v2/organizations/:id/items", func(c *gin.Context) {
		c.Set(constants.ContextKeyUserID, uint(42))
		c.Set(constants.ContextKeyOrgID, uint(99))
		handler.ListV2(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v2/organizations/abc/items?limit=25&since_revision=8", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 25, itemSvc.v2Request.Limit)
	assert.EqualValues(t, 8, itemSvc.v2Request.SinceRevision)
	assert.Contains(t, rec.Body.String(), `"deleted":true`)
	assert.NotContains(t, rec.Body.String(), `"data"`)

	invalid := httptest.NewRequest(http.MethodGet, "/api/v2/organizations/abc/items?limit=501", nil)
	invalidRec := httptest.NewRecorder()
	router.ServeHTTP(invalidRec, invalid)
	assert.Equal(t, http.StatusBadRequest, invalidRec.Code)
}

func TestOrganizationItemHandler_Create(t *testing.T) {
	t.Parallel()

	buildRouter := func(itemSvc service.OrganizationItemService, policySvc service.PolicyEnforcementService) *gin.Engine {
		handler := &OrganizationItemHandler{
			service:           itemSvc,
			policyEnforcement: policySvc,
			activityLogger:    nil,
		}

		r := gin.New()
		r.POST("/organizations/:id/items", func(c *gin.Context) {
			c.Set(constants.ContextKeyUserID, uint(42))
			c.Set(constants.ContextKeyOrgID, uint(99))
			handler.Create(c)
		})
		return r
	}

	baseBody := map[string]interface{}{
		"item_type": int(domain.ItemTypePassword),
		"data":      "encrypted-payload",
		"metadata": map[string]interface{}{
			"name": "My Item",
		},
	}

	t.Run("blocks card item when remove_card_type policy is enabled", func(t *testing.T) {
		t.Parallel()
		itemSvc := &stubOrganizationItemService{}
		policySvc := &stubPolicyEnforcementService{
			checkCardErr: errors.New("organization policy prohibits credit card items"),
		}
		router := buildRouter(itemSvc, policySvc)

		body := map[string]interface{}{
			"item_type": int(domain.ItemTypeCard),
			"data":      "encrypted-card",
			"metadata": map[string]interface{}{
				"name": "Company Card",
			},
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/organizations/abc/items", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rr.Code)
		}
		if !policySvc.checkCardCalled {
			t.Fatal("expected CheckCardTypeAllowed to be called for card item")
		}
		if itemSvc.createCalled {
			t.Fatal("expected Create not to be called when policy blocks card item")
		}
	})

	t.Run("does not call card policy check for non-card items", func(t *testing.T) {
		t.Parallel()
		itemSvc := &stubOrganizationItemService{
			createItem: &domain.OrganizationItem{
				ID:              1,
				UUID:            uuid.New(),
				SupportID:       1234,
				OrganizationID:  99,
				ItemType:        domain.ItemTypePassword,
				Data:            "encrypted-payload",
				Metadata:        domain.ItemMetadata{Name: "My Item"},
				CreatedByUserID: 42,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
				AutoFill:        true,
				AutoLogin:       false,
				Revision:        1,
				SyncVersion:     1,
			},
		}
		policySvc := &stubPolicyEnforcementService{}
		router := buildRouter(itemSvc, policySvc)

		raw, _ := json.Marshal(baseBody)
		req := httptest.NewRequest(http.MethodPost, "/organizations/abc/items", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", rr.Code, rr.Body.String())
		}
		if policySvc.checkCardCalled {
			t.Fatal("expected CheckCardTypeAllowed not to be called for non-card item")
		}
		if !itemSvc.createCalled {
			t.Fatal("expected Create to be called")
		}
	})

	t.Run("calls card policy check and creates when allowed", func(t *testing.T) {
		t.Parallel()
		itemSvc := &stubOrganizationItemService{
			createItem: &domain.OrganizationItem{
				ID:              2,
				UUID:            uuid.New(),
				SupportID:       5678,
				OrganizationID:  99,
				ItemType:        domain.ItemTypeCard,
				Data:            "encrypted-card",
				Metadata:        domain.ItemMetadata{Name: "Company Card"},
				CreatedByUserID: 42,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
				AutoFill:        false,
				AutoLogin:       false,
				Revision:        1,
				SyncVersion:     1,
			},
		}
		policySvc := &stubPolicyEnforcementService{}
		router := buildRouter(itemSvc, policySvc)

		body := map[string]interface{}{
			"item_type": int(domain.ItemTypeCard),
			"data":      "encrypted-card",
			"metadata": map[string]interface{}{
				"name": "Company Card",
			},
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/organizations/abc/items", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", rr.Code, rr.Body.String())
		}
		if !policySvc.checkCardCalled {
			t.Fatal("expected CheckCardTypeAllowed to be called for card item")
		}
		if !itemSvc.createCalled {
			t.Fatal("expected Create to be called when policy allows card item")
		}
	})
}

func TestOrganizationItemHandler_WriteEntitlementErrors(t *testing.T) {
	t.Parallel()

	errorCases := []struct {
		name    string
		err     error
		message string
		code    string
	}{
		{
			name:    "expired subscription",
			err:     service.ErrSubscriptionExpired,
			message: "subscription expired",
			code:    "SUBSCRIPTION_EXPIRED",
		},
		{
			name:    "plan limit reached",
			err:     service.ErrPlanLimitReached,
			message: "plan limit reached",
			code:    "PLAN_LIMIT_REACHED",
		},
		{
			name:    "feature unavailable",
			err:     service.ErrFeatureNotAvailable,
			message: "feature not available in current plan",
			code:    "FEATURE_NOT_AVAILABLE",
		},
	}

	operations := []struct {
		name   string
		method string
		route  string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, route: "/organizations/:id/items", path: "/organizations/99/items", body: `{"item_type":1,"data":"encrypted","metadata":{"name":"item"}}`},
		{name: "update", method: http.MethodPut, route: "/org-items/:id", path: "/org-items/9", body: `{"is_favorite":true}`},
		{name: "delete", method: http.MethodDelete, route: "/org-items/:id", path: "/org-items/9"},
	}

	for _, errorCase := range errorCases {
		errorCase := errorCase
		for _, operation := range operations {
			operation := operation
			t.Run(errorCase.name+"/"+operation.name, func(t *testing.T) {
				t.Parallel()

				wrappedErr := fmt.Errorf("write entitlement check failed: %w", errorCase.err)
				itemSvc := &stubOrganizationItemService{
					createErr: wrappedErr,
					updateErr: wrappedErr,
					deleteErr: wrappedErr,
				}
				handler := &OrganizationItemHandler{service: itemSvc}
				router := gin.New()
				router.Handle(operation.method, operation.route, func(c *gin.Context) {
					c.Set(constants.ContextKeyUserID, uint(42))
					c.Set(constants.ContextKeyOrgID, uint(99))
					switch operation.name {
					case "create":
						handler.Create(c)
					case "update":
						handler.Update(c)
					case "delete":
						handler.Delete(c)
					}
				})

				req := httptest.NewRequest(operation.method, operation.path, bytes.NewBufferString(operation.body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)

				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				assert.JSONEq(t, fmt.Sprintf(`{"error":%q,"code":%q}`, errorCase.message, errorCase.code), rec.Body.String())
				assert.NotContains(t, rec.Body.String(), "write entitlement check failed")
			})
		}
	}
}
