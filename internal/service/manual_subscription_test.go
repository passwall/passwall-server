package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/stripe/stripe-go/v81"
)

type reminderSender struct {
	sent int
	fail bool
}

func (s *reminderSender) Send(context.Context, *email.EmailMessage) error {
	if s.fail {
		return errors.New("send failed")
	}
	s.sent++
	return nil
}
func (s *reminderSender) Provider() email.Provider { return email.ProviderSMTP }
func (s *reminderSender) Close() error             { return nil }

func TestManualGrantRejectsExternalSubscription(t *testing.T) {
	stripeID := "sub_active"
	plan := &domain.Plan{ID: 2, Code: "pro-monthly", IsActive: true}
	repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		State: domain.SubStateActive, StripeSubscriptionID: &stripeID, Plan: plan,
	}}
	service := NewSubscriptionService(
		repo,
		lifecyclePlanRepo{free: plan},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)

	_, err := service.GrantManual(context.Background(), 7, ManualSubscriptionInput{
		PlanCode: plan.Code,
		EndsAt:   time.Now().Add(24 * time.Hour),
		Note:     "support grant",
	})
	var typed *ManualSubscriptionError
	if !errors.As(err, &typed) || typed.Code != ManualSubscriptionCodeExternalActive {
		t.Fatalf("error = %v, want %s", err, ManualSubscriptionCodeExternalActive)
	}
	if len(repo.created) != 0 {
		t.Fatalf("external subscription was overwritten")
	}
}

func TestNewProviderPurchaseSupersedesManualGrant(t *testing.T) {
	end := time.Now().Add(24 * time.Hour)
	plan := &domain.Plan{ID: 2, Code: "pro-monthly", IsActive: true, PriceCents: 300}
	repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 8, OrganizationID: 7, State: domain.SubStateActive, RenewAt: &end, Plan: plan,
	}}
	tx := &lifecycleTxManager{}
	service := NewSubscriptionService(
		repo,
		lifecyclePlanRepo{free: plan},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
		nil, nil, nil, noopLogger{}, tx,
	)

	// The owner paid while a grant was active: the paid subscription must be
	// recorded, otherwise Stripe charges without the DB knowing about it.
	sub, err := service.CreateFromProvider(context.Background(), 7, plan.Code, "sub_provider", nil, nil)
	if err != nil {
		t.Fatalf("CreateFromProvider() error = %v", err)
	}
	if len(repo.created) != 1 || *sub.StripeSubscriptionID != "sub_provider" || !repo.expiredInTx {
		t.Fatalf("provider purchase not recorded atomically: created=%d expiredInTx=%v", len(repo.created), repo.expiredInTx)
	}
}

func TestGenericProviderUpdateCannotMutateActiveManualGrant(t *testing.T) {
	end := time.Now().Add(24 * time.Hour)
	plan := &domain.Plan{ID: 2, Code: "pro-monthly"}
	manual := &domain.Subscription{
		ID: 8, OrganizationID: 7, State: domain.SubStateActive, RenewAt: &end, Plan: plan,
	}
	repo := &lifecycleSubscriptionRepo{sub: manual}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: plan}, nil,
		nil, nil, nil, noopLogger{}, nil,
	)
	proposed := *manual
	proposed.State = domain.SubStatePastDue
	if err := service.Update(context.Background(), &proposed); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repo.sub.State != domain.SubStateActive {
		t.Fatalf("manual state = %q, want active", repo.sub.State)
	}
}

func TestManualGrantPlanMatrixAndFreeOverQuota(t *testing.T) {
	family := &domain.Plan{ID: 3, Code: "family-monthly", IsActive: true}
	service := NewSubscriptionService(
		&lifecycleSubscriptionRepo{},
		lifecyclePlanRepo{free: family},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)
	_, err := service.GrantManual(context.Background(), 7, ManualSubscriptionInput{
		PlanCode: family.Code, EndsAt: time.Now().Add(24 * time.Hour), Note: "invalid target",
	})
	var typed *ManualSubscriptionError
	if !errors.As(err, &typed) || typed.Code != ManualSubscriptionCodePlanNotAllowed {
		t.Fatalf("error = %v, want %s", err, ManualSubscriptionCodePlanNotAllowed)
	}

	free := &domain.Plan{
		ID: 1, Code: "free-monthly", IsActive: true,
		MaxUsers: intPointer(1), MaxCollections: intPointer(10), MaxItems: intPointer(100),
	}
	repo := &lifecycleSubscriptionRepo{}
	service = NewSubscriptionService(
		repo,
		lifecyclePlanRepo{free: free},
		lifecycleOrgRepo{
			org:     &domain.Organization{ID: 7, IsPersonal: true},
			members: 4, collections: 50, items: 1000,
		},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)
	if _, err := service.GrantManual(context.Background(), 7, ManualSubscriptionInput{
		PlanCode: free.Code, EndsAt: time.Now().Add(24 * time.Hour), Note: "manage down",
	}); err != nil {
		t.Fatalf("free over-quota transition was blocked: %v", err)
	}
}

func TestExtendManualPreservesPlanAndSeatsAndResetsReminder(t *testing.T) {
	oldEnd := time.Now().Add(24 * time.Hour)
	sentAt := time.Now()
	seats := 8
	plan := &domain.Plan{ID: 4, Code: "team-yearly", IsActive: true, PriceCents: 1000}
	sub := &domain.Subscription{
		ID: 9, OrganizationID: 7, PlanID: plan.ID, Plan: plan,
		State: domain.SubStateActive, RenewAt: &oldEnd, SeatsPurchased: &seats,
		ManualEndNoticeSentAt: &sentAt,
	}
	repo := &lifecycleSubscriptionRepo{sub: sub}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: plan},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7}},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)
	newEnd := time.Now().Add(30 * 24 * time.Hour)
	change, err := service.ExtendManual(context.Background(), 7, newEnd, "renew support grant")
	if err != nil {
		t.Fatalf("ExtendManual() error = %v", err)
	}
	if change.Subscription.PlanID != plan.ID || change.Subscription.SeatsPurchased == nil || *change.Subscription.SeatsPurchased != seats {
		t.Fatalf("plan/seats changed: %+v", change.Subscription)
	}
	if change.Subscription.ManualEndNoticeSentAt != nil || !change.Subscription.RenewAt.Equal(newEnd) {
		t.Fatalf("extension did not reset reminder/update end: %+v", change.Subscription)
	}
}

func TestManualReminderMarksOnlyAfterSuccess(t *testing.T) {
	started := time.Now().Add(-10 * 24 * time.Hour)
	ends := time.Now().Add(3 * 24 * time.Hour)
	plan := &domain.Plan{ID: 2, Code: "pro-monthly"}
	sub := &domain.Subscription{
		ID: 9, OrganizationID: 7, Plan: plan, State: domain.SubStateActive,
		StartedAt: &started, RenewAt: &ends,
		Organization: &domain.Organization{ID: 7, Name: "Personal Vault", BillingEmail: "billing@example.com", IsPersonal: true},
	}
	repo := &lifecycleSubscriptionRepo{sub: sub, endingSoon: []*domain.Subscription{sub}}
	builder, err := email.NewEmailBuilder("https://vault.passwall.io", "hello@passwall.io")
	if err != nil {
		t.Fatal(err)
	}
	sender := &reminderSender{fail: true}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: plan},
		lifecycleOrgRepo{org: sub.Organization},
		nil, nil, nil, noopLogger{}, nil,
		WithManualSubscriptionEmails(sender, builder, nil, "https://vault.example.test"),
	)
	if err := service.SendManualExpiryReminders(context.Background()); err == nil {
		t.Fatal("expected send failure")
	}
	if sub.ManualEndNoticeSentAt != nil {
		t.Fatal("failed send marked reminder")
	}

	sender.fail = false
	if err := service.SendManualExpiryReminders(context.Background()); err != nil {
		t.Fatalf("SendManualExpiryReminders() error = %v", err)
	}
	if sender.sent != 1 || sub.ManualEndNoticeSentAt == nil {
		t.Fatalf("reminder result: sent=%d marker=%v", sender.sent, sub.ManualEndNoticeSentAt)
	}
	if err := service.SendManualExpiryReminders(context.Background()); err != nil {
		t.Fatalf("dedupe reminder call error = %v", err)
	}
	if sender.sent != 1 {
		t.Fatalf("duplicate reminder sent: %d", sender.sent)
	}
}

func TestManualGrantSucceedsWhenOrganizationHasNoSubscription(t *testing.T) {
	plan := &domain.Plan{ID: 2, Code: "pro-yearly", IsActive: true, PriceCents: 1000}
	repo := &lifecycleSubscriptionRepo{missing: true}
	tx := &lifecycleTxManager{}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: plan},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
		nil, nil, nil, noopLogger{}, tx,
	)

	change, err := service.GrantManual(context.Background(), 7, ManualSubscriptionInput{
		PlanCode: plan.Code, EndsAt: time.Now().Add(24 * time.Hour), Note: "support grant",
	})
	if err != nil {
		t.Fatalf("GrantManual() error = %v", err)
	}
	if len(repo.created) != 1 || !repo.lockedInTx || !repo.createdInTx {
		t.Fatalf("grant not created under lock in tx: created=%d locked=%v inTx=%v", len(repo.created), repo.lockedInTx, repo.createdInTx)
	}
	created := repo.created[0]
	if created.StripeSubscriptionID != nil {
		t.Fatalf("manual grant stored provider id %q, want NULL", *created.StripeSubscriptionID)
	}
	if change.OldPlan != "" || change.NewPlan != plan.Code {
		t.Fatalf("change = %+v", change)
	}
}

func TestManualPlanChangeKeepsReminderForSameEndDate(t *testing.T) {
	end := time.Now().Add(20 * 24 * time.Hour).Truncate(time.Second)
	sentAt := time.Now()
	team := &domain.Plan{ID: 4, Code: "team-yearly", IsActive: true, PriceCents: 1000}
	business := &domain.Plan{ID: 5, Code: "business-yearly", IsActive: true, PriceCents: 1000}
	repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 9, OrganizationID: 7, Plan: team, State: domain.SubStateActive,
		RenewAt: &end, ManualEndNoticeSentAt: &sentAt,
	}}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: business},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7}},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)

	if _, err := service.GrantManual(context.Background(), 7, ManualSubscriptionInput{
		PlanCode: business.Code, EndsAt: end, Note: "upgrade",
	}); err != nil {
		t.Fatalf("GrantManual() error = %v", err)
	}
	if got := repo.created[0].ManualEndNoticeSentAt; got == nil || !got.Equal(sentAt) {
		t.Fatalf("notice marker = %v, want preserved %v", got, sentAt)
	}

	repo.created = nil
	if _, err := service.GrantManual(context.Background(), 7, ManualSubscriptionInput{
		PlanCode: business.Code, EndsAt: end.Add(24 * time.Hour), Note: "upgrade with new end",
	}); err != nil {
		t.Fatalf("GrantManual() error = %v", err)
	}
	if repo.created[0].ManualEndNoticeSentAt != nil {
		t.Fatal("new end date kept the previous reminder marker")
	}
}

func TestExtendManualAcceptsGrantThatLapsedBeforeWorker(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	plan := &domain.Plan{ID: 2, Code: "pro-yearly", IsActive: true, PriceCents: 1000}
	repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 9, OrganizationID: 7, Plan: plan, State: domain.SubStateActive, RenewAt: &past,
	}}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: plan},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)
	newEnd := time.Now().Add(30 * 24 * time.Hour)
	if _, err := service.ExtendManual(context.Background(), 7, newEnd, "renew"); err != nil {
		t.Fatalf("ExtendManual() error = %v", err)
	}
	if !repo.sub.RenewAt.Equal(newEnd) {
		t.Fatalf("renew_at = %v, want %v", repo.sub.RenewAt, newEnd)
	}
}

func TestExtendManualRejectsMissingOrCatalogFreeSubscription(t *testing.T) {
	free := &domain.Plan{ID: 1, Code: "free-monthly", IsActive: true}
	end := time.Now().Add(24 * time.Hour)
	for name, repo := range map[string]*lifecycleSubscriptionRepo{
		"missing":      {missing: true},
		"catalog free": {sub: &domain.Subscription{ID: 9, Plan: free, State: domain.SubStateActive}},
		"dated free":   {sub: &domain.Subscription{ID: 9, Plan: free, State: domain.SubStateActive, RenewAt: &end}},
	} {
		t.Run(name, func(t *testing.T) {
			service := NewSubscriptionService(
				repo, lifecyclePlanRepo{free: free},
				lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
				nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
			)
			_, err := service.ExtendManual(context.Background(), 7, time.Now().Add(48*time.Hour), "renew")
			var typed *ManualSubscriptionError
			if !errors.As(err, &typed) || typed.Code != ManualSubscriptionCodeNotActiveManual {
				t.Fatalf("error = %v, want %s", err, ManualSubscriptionCodeNotActiveManual)
			}
		})
	}
}

func TestEndManualPersonalMovesToFreeAndSharedFreezes(t *testing.T) {
	end := time.Now().Add(30 * 24 * time.Hour)
	seats := 5
	pro := &domain.Plan{ID: 2, Code: "pro-yearly", IsActive: true, PriceCents: 1000}
	free := &domain.Plan{ID: 1, Code: "free-monthly", IsActive: true}

	personalRepo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 9, OrganizationID: 7, Plan: pro, State: domain.SubStateActive, RenewAt: &end,
	}}
	personal := NewSubscriptionService(
		personalRepo, lifecyclePlanRepo{free: free},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)
	change, err := personal.EndManual(context.Background(), 7, "move to free")
	if err != nil {
		t.Fatalf("EndManual(personal) error = %v", err)
	}
	if personalRepo.sub.State != domain.SubStateExpired || personalRepo.sub.EndedAt == nil {
		t.Fatalf("manual row not expired: %+v", personalRepo.sub)
	}
	if len(personalRepo.created) != 1 || personalRepo.created[0].PlanID != free.ID || personalRepo.created[0].RenewAt != nil {
		t.Fatalf("personal end did not create an open-ended free row: %+v", personalRepo.created)
	}
	if change.OldPlan != pro.Code || change.NewPlan != free.Code {
		t.Fatalf("change = %+v", change)
	}

	team := &domain.Plan{ID: 4, Code: "team-yearly", IsActive: true, PriceCents: 1000}
	sharedRepo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 10, OrganizationID: 8, Plan: team, State: domain.SubStateActive, RenewAt: &end, SeatsPurchased: &seats,
	}}
	shared := NewSubscriptionService(
		sharedRepo, lifecyclePlanRepo{free: free},
		lifecycleOrgRepo{org: &domain.Organization{ID: 8}},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)
	change, err = shared.EndManual(context.Background(), 8, "end now")
	if err != nil {
		t.Fatalf("EndManual(shared) error = %v", err)
	}
	if sharedRepo.sub.State != domain.SubStateExpired || len(sharedRepo.created) != 0 {
		t.Fatalf("shared end must expire without a replacement row: state=%s created=%d", sharedRepo.sub.State, len(sharedRepo.created))
	}
	if change.Seats == nil || *change.Seats != seats {
		t.Fatalf("audit lost previous seats: %+v", change.Seats)
	}
}

func TestEndManualRefusesExternalSubscription(t *testing.T) {
	stripeID := "sub_live"
	plan := &domain.Plan{ID: 2, Code: "pro-yearly", IsActive: true, PriceCents: 1000}
	repo := &lifecycleSubscriptionRepo{sub: &domain.Subscription{
		ID: 9, Plan: plan, State: domain.SubStateActive, StripeSubscriptionID: &stripeID,
	}}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: plan},
		lifecycleOrgRepo{org: &domain.Organization{ID: 7, IsPersonal: true}},
		nil, nil, nil, noopLogger{}, &lifecycleTxManager{},
	)
	_, err := service.EndManual(context.Background(), 7, "end")
	var typed *ManualSubscriptionError
	if !errors.As(err, &typed) || typed.Code != ManualSubscriptionCodeExternalActive {
		t.Fatalf("error = %v, want %s", err, ManualSubscriptionCodeExternalActive)
	}
	if repo.sub.State != domain.SubStateActive {
		t.Fatal("external subscription was ended")
	}
}

func TestManualReminderSkipsShortGrantsAndLinksToBilling(t *testing.T) {
	plan := &domain.Plan{ID: 2, Code: "pro-yearly", PriceCents: 1000, Name: "Pro"}
	org := &domain.Organization{ID: 7, PublicID: "org_pub", Name: "Personal Vault", BillingEmail: "owner@example.com", IsPersonal: true}
	shortStart := time.Now().Add(-time.Hour)
	shortEnd := time.Now().Add(3 * 24 * time.Hour)
	short := &domain.Subscription{ID: 9, OrganizationID: 7, Plan: plan, State: domain.SubStateActive, StartedAt: &shortStart, RenewAt: &shortEnd, Organization: org}

	longStart := time.Now().Add(-60 * 24 * time.Hour)
	longEnd := time.Now().Add(5 * 24 * time.Hour)
	long := &domain.Subscription{ID: 10, OrganizationID: 7, Plan: plan, State: domain.SubStateActive, StartedAt: &longStart, RenewAt: &longEnd, Organization: org}

	repo := &lifecycleSubscriptionRepo{endingSoon: []*domain.Subscription{short, long}}
	builder, err := email.NewEmailBuilder("https://vault.passwall.io", "hello@passwall.io")
	if err != nil {
		t.Fatal(err)
	}
	sender := &capturingSender{}
	service := NewSubscriptionService(
		repo, lifecyclePlanRepo{free: plan}, lifecycleOrgRepo{org: org},
		nil, nil, nil, noopLogger{}, nil,
		WithManualSubscriptionEmails(sender, builder, nil, "https://vault.example.test/"),
	)
	if err := service.SendManualExpiryReminders(context.Background()); err != nil {
		t.Fatalf("SendManualExpiryReminders() error = %v", err)
	}
	if len(sender.messages) != 1 || short.ManualEndNoticeSentAt != nil || long.ManualEndNoticeSentAt == nil {
		t.Fatalf("sent=%d short=%v long=%v", len(sender.messages), short.ManualEndNoticeSentAt, long.ManualEndNoticeSentAt)
	}
	body := sender.messages[0].Body
	for _, want := range []string{"https://vault.example.test/organizations/org_pub/billing", "Pro", "Free plan"} {
		if !strings.Contains(body, want) {
			t.Fatalf("reminder body missing %q", want)
		}
	}
}

type capturingSender struct {
	messages []*email.EmailMessage
}

func (s *capturingSender) Send(_ context.Context, message *email.EmailMessage) error {
	s.messages = append(s.messages, message)
	return nil
}
func (s *capturingSender) Provider() email.Provider { return email.ProviderSMTP }
func (s *capturingSender) Close() error             { return nil }

func TestStripeWebhookForSubscriptionWithoutAccessIsNoop(t *testing.T) {
	// No collaborators: reaching the org/subscription update would panic.
	svc := &paymentService{logger: noopLogger{}}
	for _, status := range []stripe.SubscriptionStatus{
		stripe.SubscriptionStatusCanceled,
		stripe.SubscriptionStatusIncompleteExpired,
		stripe.SubscriptionStatusUnpaid,
	} {
		sub := &stripe.Subscription{ID: "sub_old", Status: status, Metadata: map[string]string{"organization_id": "7"}}
		if err := svc.updateOrgFromSubscription(context.Background(), sub); err != nil {
			t.Fatalf("status %s: error = %v, want nil so Stripe does not retry", status, err)
		}
	}
}
