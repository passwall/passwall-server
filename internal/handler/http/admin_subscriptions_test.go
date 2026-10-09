package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

func TestAdminManualGrantBlockedReasonOnlyExternalProviders(t *testing.T) {
	now := time.Now()
	end := now.Add(24 * time.Hour)

	free := &domain.Subscription{
		State:   domain.SubStateActive,
		RenewAt: &end,
		Plan:    &domain.Plan{Code: "free-monthly"},
	}
	if reason := adminManualGrantBlockedReason(free, now); reason != nil {
		t.Fatalf("free active should be grantable, got %s", *reason)
	}

	manual := &domain.Subscription{
		State:   domain.SubStateActive,
		RenewAt: &end,
		Plan:    &domain.Plan{Code: "pro-yearly", PriceCents: 1900},
	}
	if reason := adminManualGrantBlockedReason(manual, now); reason != nil {
		t.Fatalf("manual paid should be grantable, got %s", *reason)
	}

	stripeID := "sub_123"
	stripe := &domain.Subscription{
		State:                domain.SubStateActive,
		StripeSubscriptionID: &stripeID,
		RenewAt:              &end,
		Plan:                 &domain.Plan{Code: "pro-yearly", PriceCents: 1900},
	}
	reason := adminManualGrantBlockedReason(stripe, now)
	if reason == nil || *reason != service.ManualSubscriptionCodeExternalActive {
		t.Fatalf("stripe active should block with %s, got %v", service.ManualSubscriptionCodeExternalActive, reason)
	}

	expiredStripe := &domain.Subscription{
		State:                domain.SubStateExpired,
		StripeSubscriptionID: &stripeID,
		Plan:                 &domain.Plan{Code: "pro-yearly", PriceCents: 1900},
	}
	if reason := adminManualGrantBlockedReason(expiredStripe, now); reason != nil {
		t.Fatalf("expired stripe should be grantable, got %s", *reason)
	}
}

// adminListOrgRepo leaves per-organization stat methods unimplemented (nil
// embedded interface), so any N+1 regression panics the test.
type adminListOrgRepo struct {
	repository.OrganizationRepository
	orgs   []*domain.Organization
	filter repository.ListFilter
}

func (r *adminListOrgRepo) List(_ context.Context, filter repository.ListFilter) ([]*domain.Organization, *repository.ListResult, error) {
	r.filter = filter
	return r.orgs, &repository.ListResult{Total: int64(len(r.orgs)), Filtered: int64(len(r.orgs))}, nil
}

func (r *adminListOrgRepo) GetCountsByIDs(_ context.Context, ids []uint) (map[uint]repository.OrganizationCounts, error) {
	out := map[uint]repository.OrganizationCounts{}
	for _, id := range ids {
		out[id] = repository.OrganizationCounts{Members: int(id) + 1}
	}
	return out, nil
}

type adminListOrgUserRepo struct {
	repository.OrganizationUserRepository
}

func (adminListOrgUserRepo) ListOwnersByOrganizationIDs(_ context.Context, ids []uint) (map[uint]*domain.OrganizationUser, error) {
	out := map[uint]*domain.OrganizationUser{}
	for _, id := range ids {
		out[id] = &domain.OrganizationUser{OrganizationID: id, UserID: id * 10, User: &domain.User{Email: "owner@example.com", Name: "Owner"}}
	}
	return out, nil
}

type adminListSubRepo struct {
	subs map[uint]*domain.Subscription
}

func (r adminListSubRepo) GetEffectiveByOrganizationIDs(context.Context, []uint) (map[uint]*domain.Subscription, error) {
	return r.subs, nil
}

func TestAdminSubscriptionsListBatchesAndMapsFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	end := time.Now().Add(3 * 24 * time.Hour)
	stripeID := "sub_live"
	orgRepo := &adminListOrgRepo{orgs: []*domain.Organization{{ID: 1, Name: "Manual"}, {ID: 2, Name: "Stripe"}}}
	handler := NewAdminSubscriptionsHandler(
		orgRepo, adminListOrgUserRepo{},
		adminListSubRepo{subs: map[uint]*domain.Subscription{
			1: {OrganizationID: 1, State: domain.SubStateActive, RenewAt: &end, Plan: &domain.Plan{Code: "pro-yearly", PriceCents: 3600}},
			2: {OrganizationID: 2, State: domain.SubStateActive, RenewAt: &end, StripeSubscriptionID: &stripeID, Plan: &domain.Plan{Code: "team-yearly", PriceCents: 9900}},
		}},
		nil, nil, nil, noopHandlerLogger{},
	)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/subscriptions?search=alice&owner_user_id=10&ending_within_days=7&sort=access_end", nil)
	handler.List(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	f := orgRepo.filter
	if f.Search != "alice" || !f.SearchOwners || f.OwnerUserID != 10 || !f.SortByAccessEndAsc || f.ManualGrantEndsBefore == nil {
		t.Fatalf("filter not mapped: %+v", f)
	}
	var body adminSubscriptionListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	manual, stripe := body.Items[0], body.Items[1]
	if manual.Provider != domain.PaymentProviderManual || manual.AutoRenews || !manual.CanManualGrant || !manual.RiskExpiring || manual.CurrentUsers != 2 || manual.Owner == nil {
		t.Fatalf("manual item = %+v", manual)
	}
	if stripe.Provider != domain.PaymentProviderStripe || !stripe.AutoRenews || stripe.CanManualGrant || stripe.BlockedReason == nil || stripe.RiskExpiring {
		t.Fatalf("stripe item = %+v", stripe)
	}
}

type noopHandlerLogger struct{}

func (noopHandlerLogger) Debug(string, ...interface{}) {}
func (noopHandlerLogger) Info(string, ...interface{})  {}
func (noopHandlerLogger) Infof(string, ...interface{}) {}
func (noopHandlerLogger) Warn(string, ...interface{})  {}
func (noopHandlerLogger) Error(string, ...interface{}) {}
