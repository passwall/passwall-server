package service

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type catalogPlanRepo map[string]*domain.Plan

func (r catalogPlanRepo) GetByCode(_ context.Context, code string) (*domain.Plan, error) {
	if plan, ok := r[code]; ok {
		return plan, nil
	}
	return nil, repository.ErrNotFound
}

func TestResolveInitialPlan(t *testing.T) {
	plans := catalogPlanRepo{
		"free-monthly":  {ID: 1, Code: "free-monthly", IsActive: true},
		"team-yearly":   {ID: 5, Code: "team-yearly", Name: "Team", IsActive: true, MaxUsers: intPointer(10)},
		"team-monthly":  {ID: 4, Code: "team-monthly", Name: "Team", IsActive: true, MaxUsers: intPointer(10)},
		"family-yearly": {ID: 3, Code: "family-yearly", Name: "Family", IsActive: false, MaxUsers: intPointer(6)},
	}
	svc := &organizationService{planRepo: plans}

	tests := []struct {
		name      string
		req       domain.CreateOrganizationRequest
		wantCode  string
		wantSeats int
		wantErr   bool
	}{
		{name: "no plan keeps legacy free org", req: domain.CreateOrganizationRequest{}, wantCode: "free-monthly", wantSeats: 1},
		{name: "explicit free", req: domain.CreateOrganizationRequest{Plan: "free"}, wantCode: "free-monthly", wantSeats: 1},
		{name: "shared plan defaults to yearly", req: domain.CreateOrganizationRequest{Plan: "team", Seats: 4}, wantCode: "team-yearly", wantSeats: 4},
		{name: "monthly cycle", req: domain.CreateOrganizationRequest{Plan: "team", BillingCycle: "monthly"}, wantCode: "team-monthly", wantSeats: 1},
		{name: "seats above plan limit", req: domain.CreateOrganizationRequest{Plan: "team", Seats: 11}, wantErr: true},
		{name: "inactive plan", req: domain.CreateOrganizationRequest{Plan: "family"}, wantErr: true},
		{name: "personal plan rejected", req: domain.CreateOrganizationRequest{Plan: "pro"}, wantErr: true},
		{name: "unknown cycle", req: domain.CreateOrganizationRequest{Plan: "team", BillingCycle: "weekly"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, seats, err := svc.resolveInitialPlan(context.Background(), &tt.req)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidOrganizationPlan) {
					t.Fatalf("error = %v, want ErrInvalidOrganizationPlan", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveInitialPlan() error = %v", err)
			}
			if plan.Code != tt.wantCode || seats != tt.wantSeats {
				t.Fatalf("got plan %s seats %d, want %s seats %d", plan.Code, seats, tt.wantCode, tt.wantSeats)
			}
		})
	}
}

type fakeTrialHistory struct {
	used         bool
	err          error
	gotUserID    uint
	gotPersonal  bool
	lookupCalled bool
}

func (f *fakeTrialHistory) HasPaidHistoryForUser(_ context.Context, userID uint, personal bool) (bool, error) {
	f.lookupCalled = true
	f.gotUserID = userID
	f.gotPersonal = personal
	return f.used, f.err
}

func TestIsTrialEligible(t *testing.T) {
	ownerID := uint(42)
	tests := []struct {
		name    string
		history *fakeTrialHistory
		org     *domain.Organization
		want    bool
	}{
		{"first shared subscription gets trial", &fakeTrialHistory{}, &domain.Organization{ID: 1, CreatedByUserID: &ownerID}, true},
		{"previous shared subscription blocks trial", &fakeTrialHistory{used: true}, &domain.Organization{ID: 1, CreatedByUserID: &ownerID}, false},
		{"lookup failure fails closed", &fakeTrialHistory{err: errors.New("db down")}, &domain.Organization{ID: 1, CreatedByUserID: &ownerID}, false},
		{"personal vault checks personal history", &fakeTrialHistory{}, &domain.Organization{ID: 2, IsPersonal: true, PersonalOwnerUserID: &ownerID}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &paymentService{trialHistory: tt.history, logger: noopLogger{}}
			if got := svc.isTrialEligible(context.Background(), tt.org, 99); got != tt.want {
				t.Fatalf("isTrialEligible() = %v, want %v", got, tt.want)
			}
			if tt.history.gotUserID != ownerID || tt.history.gotPersonal != tt.org.IsPersonal {
				t.Fatalf("lookup used user %d personal %v, want owner %d personal %v",
					tt.history.gotUserID, tt.history.gotPersonal, ownerID, tt.org.IsPersonal)
			}
		})
	}
}

type seatEntitlements struct {
	OrganizationEntitlementService
	maxUsers *int
}

func (s seatEntitlements) Resolve(context.Context, uint) (*domain.EntitlementSnapshot, error) {
	return &domain.EntitlementSnapshot{Limits: domain.EntitlementLimits{MaxUsers: s.maxUsers}}, nil
}

type seatOrgUserRepo struct {
	repository.OrganizationUserRepository
	members []*domain.OrganizationUser
}

func (r seatOrgUserRepo) ListByOrganization(context.Context, uint) ([]*domain.OrganizationUser, error) {
	return r.members, nil
}

type seatInvitationService struct {
	InvitationService
	pending []*domain.Invitation
}

func (s seatInvitationService) ListAwaitingSignup(context.Context, uint) ([]*domain.Invitation, error) {
	return s.pending, nil
}

func TestEnsureSeatAvailableCountsPendingInvitations(t *testing.T) {
	members := []*domain.OrganizationUser{
		{ID: 1, Status: domain.OrgUserStatusAccepted},
		{ID: 2, Status: domain.OrgUserStatusInvited},
		{ID: 3, Status: domain.OrgUserStatusSuspended},
	}
	tests := []struct {
		name     string
		maxUsers *int
		pending  int
		wantDeny bool
	}{
		{"unlimited plan", nil, 5, false},
		{"seat left", intPointer(4), 1, false},
		{"pending invitation fills last seat", intPointer(3), 1, true},
		{"members fill seats", intPointer(2), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pending := make([]*domain.Invitation, tt.pending)
			svc := &organizationService{
				entitlements:      seatEntitlements{maxUsers: tt.maxUsers},
				orgUserRepo:       seatOrgUserRepo{members: members},
				invitationService: seatInvitationService{pending: pending},
			}
			err := svc.ensureSeatAvailable(context.Background(), 1)
			var denied *EntitlementDeniedError
			if tt.wantDeny != errors.As(err, &denied) {
				t.Fatalf("ensureSeatAvailable() error = %v, wantDeny %v", err, tt.wantDeny)
			}
			if tt.wantDeny && denied.Reason != domain.EntitlementReasonPlanLimitReached {
				t.Fatalf("reason = %q, want plan limit reached", denied.Reason)
			}
		})
	}
}

func TestInvitationIsAwaitingSignup(t *testing.T) {
	orgID := uint(1)
	role := "member"
	key := "wrapped"
	empty := ""
	tests := []struct {
		name string
		inv  domain.Invitation
		want bool
	}{
		{"platform invitation", domain.Invitation{}, false},
		{"registered invitee with key", domain.Invitation{OrganizationID: &orgID, OrgRole: &role, EncryptedOrgKey: &key}, false},
		{"keyless org invitation", domain.Invitation{OrganizationID: &orgID, OrgRole: &role}, true},
		{"empty key org invitation", domain.Invitation{OrganizationID: &orgID, OrgRole: &role, EncryptedOrgKey: &empty}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.inv.IsAwaitingSignup(); got != tt.want {
				t.Fatalf("IsAwaitingSignup() = %v, want %v", got, tt.want)
			}
		})
	}
}
