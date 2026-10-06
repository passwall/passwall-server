package service

import (
	"context"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
)

type lifecycleSubscriptionRepo struct {
	sub         *domain.Subscription
	created     []*domain.Subscription
	endedTrials []*domain.Subscription
	expiredInTx bool
	createdInTx bool
}

type lifecycleTxContextKey struct{}

func (r *lifecycleSubscriptionRepo) ExpireActiveByOrganizationID(ctx context.Context, _ uint, _ time.Time) error {
	r.expiredInTx, _ = ctx.Value(lifecycleTxContextKey{}).(bool)
	return nil
}
func (r *lifecycleSubscriptionRepo) Create(ctx context.Context, sub *domain.Subscription) error {
	r.createdInTx, _ = ctx.Value(lifecycleTxContextKey{}).(bool)
	r.created = append(r.created, sub)
	return nil
}
func (r *lifecycleSubscriptionRepo) GetByID(context.Context, uint) (*domain.Subscription, error) {
	return r.sub, nil
}
func (r *lifecycleSubscriptionRepo) GetByOrganizationID(context.Context, uint) (*domain.Subscription, error) {
	return r.sub, nil
}
func (r *lifecycleSubscriptionRepo) GetByStripeSubscriptionID(context.Context, string) (*domain.Subscription, error) {
	return r.sub, nil
}
func (r *lifecycleSubscriptionRepo) Update(_ context.Context, sub *domain.Subscription) error {
	r.sub = sub
	return nil
}
func (r *lifecycleSubscriptionRepo) ListPastDueExpired(context.Context) ([]*domain.Subscription, error) {
	return nil, nil
}
func (r *lifecycleSubscriptionRepo) ListCanceledExpired(context.Context) ([]*domain.Subscription, error) {
	return nil, nil
}
func (r *lifecycleSubscriptionRepo) ListTrialEnding(context.Context, time.Time) ([]*domain.Subscription, error) {
	return r.endedTrials, nil
}
func (r *lifecycleSubscriptionRepo) ListManualExpired(context.Context) ([]*domain.Subscription, error) {
	return nil, nil
}

type lifecyclePlanRepo struct {
	free *domain.Plan
}

func (r lifecyclePlanRepo) GetByCode(context.Context, string) (*domain.Plan, error) {
	return r.free, nil
}
func (r lifecyclePlanRepo) GetByID(context.Context, uint) (*domain.Plan, error) {
	return r.free, nil
}

type lifecycleOrgRepo struct {
	org *domain.Organization
}

func (r lifecycleOrgRepo) GetByID(context.Context, uint) (*domain.Organization, error) {
	return r.org, nil
}

type lifecycleTxManager struct {
	called bool
}

func (m *lifecycleTxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	m.called = true
	return fn(context.WithValue(ctx, lifecycleTxContextKey{}, true))
}

func TestCreateReplacesEffectiveSubscriptionAtomically(t *testing.T) {
	repo := &lifecycleSubscriptionRepo{}
	plan := &domain.Plan{
		ID:           2,
		Code:         "pro-monthly",
		BillingCycle: domain.BillingCycleMonthly,
		IsActive:     true,
	}
	txManager := &lifecycleTxManager{}
	service := NewSubscriptionService(
		repo,
		lifecyclePlanRepo{free: plan},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
		nil,
		nil,
		nil,
		noopLogger{},
		txManager,
	)

	created, err := service.Create(context.Background(), 7, plan.Code, "sub_atomic", nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created == nil || len(repo.created) != 1 {
		t.Fatalf("created subscription = %+v, persisted rows = %d", created, len(repo.created))
	}
	if !txManager.called || !repo.expiredInTx || !repo.createdInTx {
		t.Fatalf(
			"transaction coverage: manager=%v expire=%v create=%v",
			txManager.called,
			repo.expiredInTx,
			repo.createdInTx,
		)
	}
}

func TestExpireSubscriptionUsesCatalogExpiryBehavior(t *testing.T) {
	free := &domain.Plan{ID: 1, Code: "free-monthly"}
	tests := []struct {
		name           string
		expiryBehavior domain.ExpiryBehavior
		wantFree       bool
	}{
		{"personal downgrades to free", domain.ExpiryBehaviorDowngradeToFree, true},
		{"shared organization freezes without replacement", domain.ExpiryBehaviorFreeze, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paidPlan := &domain.Plan{
				ID:             2,
				Code:           "paid-monthly",
				PriceCents:     999,
				ExpiryBehavior: tt.expiryBehavior,
			}
			repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
				ID: 9, OrganizationID: 7, PlanID: paidPlan.ID,
				State: domain.SubStateActive, Plan: paidPlan,
			}}
			service := NewSubscriptionService(
				repo, lifecyclePlanRepo{free: free}, nil, nil, nil, nil, nil, nil,
			)

			if err := service.ExpireSubscription(context.Background(), 9); err != nil {
				t.Fatalf("ExpireSubscription() error = %v", err)
			}
			if repo.sub.State != domain.SubStateExpired || repo.sub.EndedAt == nil {
				t.Fatalf("paid subscription was not expired: %+v", repo.sub)
			}
			if got := len(repo.created); got != boolCount(tt.wantFree) {
				t.Fatalf("created subscriptions = %d, want %d", got, boolCount(tt.wantFree))
			}
			if tt.wantFree && (repo.created[0].PlanID != free.ID || repo.created[0].State != domain.SubStateActive) {
				t.Fatalf("unexpected free subscription: %+v", repo.created[0])
			}
		})
	}
}

func TestHandleProviderCanceledPreservesPaidThroughDate(t *testing.T) {
	renewAt := time.Now().Add(24 * time.Hour)
	plan := &domain.Plan{ID: 2, Code: "pro-monthly", ExpiryBehavior: domain.ExpiryBehaviorDowngradeToFree}
	repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 9, OrganizationID: 7, PlanID: plan.ID,
		State: domain.SubStateActive, Plan: plan, RenewAt: &renewAt,
	}}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: &domain.Plan{ID: 1, Code: "free-monthly"}},
		nil, nil, nil, nil, nil, nil,
	)

	if err := service.HandleProviderCanceled(context.Background(), 9, nil); err != nil {
		t.Fatalf("HandleProviderCanceled() error = %v", err)
	}
	if repo.sub.State != domain.SubStateCanceled || repo.sub.EndedAt != nil {
		t.Fatalf("subscription did not preserve paid-through access: %+v", repo.sub)
	}
	if len(repo.created) != 0 {
		t.Fatalf("unexpected downgrade before period end: %+v", repo.created)
	}
}

func TestHandlePaymentSuccessPreservesProviderPeriodEnd(t *testing.T) {
	periodEnd := time.Now().Add(45 * 24 * time.Hour).Round(time.Second)
	plan := &domain.Plan{
		ID:             2,
		Code:           "pro-yearly",
		PriceCents:     1900,
		BillingCycle:   domain.BillingCycleYearly,
		ExpiryBehavior: domain.ExpiryBehaviorDowngradeToFree,
	}
	repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 9, OrganizationID: 7, PlanID: plan.ID,
		State: domain.SubStateActive, Plan: plan,
		StripeSubscriptionID: stringPointer("sub_period"),
		RenewAt:              &periodEnd,
	}}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: &domain.Plan{ID: 1, Code: "free-monthly"}},
		nil, nil, nil, nil, nil, nil,
	)

	if err := service.HandlePaymentSuccess(context.Background(), "sub_period"); err != nil {
		t.Fatalf("HandlePaymentSuccess() error = %v", err)
	}
	if repo.sub.RenewAt == nil || !repo.sub.RenewAt.Equal(periodEnd) {
		t.Fatalf("RenewAt = %v, want provider period end %v", repo.sub.RenewAt, periodEnd)
	}
}

func TestCheckExpiredSubscriptionsExpiresEndedTrial(t *testing.T) {
	trialEnd := time.Now().Add(-time.Hour)
	plan := &domain.Plan{
		ID:             2,
		Code:           "pro-monthly",
		PriceCents:     199,
		ExpiryBehavior: domain.ExpiryBehaviorDowngradeToFree,
	}
	subscription := &domain.Subscription{
		ID: 9, OrganizationID: 7, PlanID: plan.ID,
		State: domain.SubStateTrialing, Plan: plan, TrialEndsAt: &trialEnd,
	}
	repo := &lifecycleSubscriptionRepo{
		sub:         subscription,
		endedTrials: []*domain.Subscription{subscription},
	}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: &domain.Plan{ID: 1, Code: "free-monthly"}},
		nil, nil, nil, nil, nil, nil,
	)

	if err := service.CheckExpiredSubscriptions(context.Background()); err != nil {
		t.Fatalf("CheckExpiredSubscriptions() error = %v", err)
	}
	if repo.sub.State != domain.SubStateExpired || len(repo.created) != 1 {
		t.Fatalf("ended trial transition = state %q, free rows %d", repo.sub.State, len(repo.created))
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func stringPointer(value string) *string {
	return &value
}
