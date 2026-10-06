package service

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type organizationItemRepoGetStub struct {
	repository.OrganizationItemRepository
	item *domain.OrganizationItem
}

func (r organizationItemRepoGetStub) GetByID(context.Context, uint) (*domain.OrganizationItem, error) {
	return r.item, nil
}

type autofillFeatureServiceStub struct {
	FeatureService
	err error
}

func (s autofillFeatureServiceStub) CanAutofill(context.Context, uint) (bool, error) {
	return false, s.err
}

type entitlementAuthorizationStub struct {
	errors   map[domain.Capability]error
	calls    []domain.Capability
	snapshot *domain.EntitlementSnapshot
}

func (s *entitlementAuthorizationStub) Resolve(context.Context, uint) (*domain.EntitlementSnapshot, error) {
	return s.snapshot, nil
}

func (s *entitlementAuthorizationStub) Authorize(
	_ context.Context,
	_ uint,
	capability domain.Capability,
) error {
	s.calls = append(s.calls, capability)
	return s.errors[capability]
}

type sendMutationRepoStub struct {
	repository.SendRepository
	send        *domain.Send
	updateCalls int
}

func (r *sendMutationRepoStub) GetByUUID(context.Context, string) (*domain.Send, error) {
	return r.send, nil
}

func (r *sendMutationRepoStub) Update(context.Context, *domain.Send) error {
	r.updateCalls++
	return nil
}

type scimTokenValidationRepoStub struct {
	repository.SCIMTokenRepository
	token       *domain.SCIMToken
	updateCalls int
}

func (r *scimTokenValidationRepoStub) GetByTokenHash(context.Context, string) (*domain.SCIMToken, error) {
	return r.token, nil
}

func (r *scimTokenValidationRepoStub) Update(context.Context, *domain.SCIMToken) error {
	r.updateCalls++
	return nil
}

type scimTeamCreateRepoStub struct {
	repository.TeamRepository
	createCalls int
}

func (r *scimTeamCreateRepoStub) Create(context.Context, *domain.Team) error {
	r.createCalls++
	return nil
}

func TestGetAutofillSecretEnforcesFrozenCapability(t *testing.T) {
	const (
		orgID  = uint(7)
		userID = uint(11)
	)
	members := newFakeOrgUserRepo()
	members.add(&domain.OrganizationUser{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           domain.OrgRoleOwner,
		Status:         domain.OrgUserStatusAccepted,
	})
	service := &organizationItemService{
		itemRepo: organizationItemRepoGetStub{
			item: &domain.OrganizationItem{ID: 9, OrganizationID: orgID},
		},
		orgUserRepo:    members,
		featureService: autofillFeatureServiceStub{err: ErrAccountFrozen},
	}

	_, err := service.GetAutofillSecret(context.Background(), 9, userID)
	if !errors.Is(err, ErrAccountFrozen) {
		t.Fatalf("GetAutofillSecret() error = %v, want ErrAccountFrozen", err)
	}
}

func TestResolveSendOrganizationRequiresActiveMembership(t *testing.T) {
	const (
		orgID  = uint(7)
		userID = uint(11)
	)
	members := newFakeOrgUserRepo()
	service := &sendService{orgUserRepo: members}

	if _, err := service.resolveSendOrganizationID(context.Background(), userID, orgID); !errors.Is(err, repository.ErrForbidden) {
		t.Fatalf("resolveSendOrganizationID() error = %v, want ErrForbidden", err)
	}

	members.add(&domain.OrganizationUser{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           domain.OrgRoleMember,
		Status:         domain.OrgUserStatusAccepted,
	})
	resolved, err := service.resolveSendOrganizationID(context.Background(), userID, orgID)
	if err != nil {
		t.Fatalf("resolveSendOrganizationID() error = %v", err)
	}
	if resolved != orgID {
		t.Fatalf("resolved organization = %d, want %d", resolved, orgID)
	}
}

func TestSendReadRemainsAvailableButUpdateEnforcesFrozenCapability(t *testing.T) {
	const (
		orgID  = uint(7)
		userID = uint(11)
	)
	members := newFakeOrgUserRepo()
	members.add(&domain.OrganizationUser{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           domain.OrgRoleOwner,
		Status:         domain.OrgUserStatusAccepted,
	})
	repo := &sendMutationRepoStub{
		send: &domain.Send{CreatorID: userID, OrganizationID: orgID},
	}
	entitlements := &entitlementAuthorizationStub{
		errors: map[domain.Capability]error{
			domain.CapabilityItemUpdate: ErrAccountFrozen,
		},
	}
	service := &sendService{
		sendRepo:     repo,
		orgUserRepo:  members,
		entitlements: entitlements,
	}

	if _, err := service.GetByUUID(context.Background(), userID, "send-id"); err != nil {
		t.Fatalf("GetByUUID() error = %v, want frozen reads to remain available", err)
	}
	if len(entitlements.calls) != 0 {
		t.Fatalf("GetByUUID() authorization calls = %v, want none", entitlements.calls)
	}

	_, err := service.Update(
		context.Background(),
		userID,
		"send-id",
		&domain.UpdateSendRequest{},
	)
	if !errors.Is(err, ErrAccountFrozen) {
		t.Fatalf("Update() error = %v, want ErrAccountFrozen", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("Update() persisted %d times, want 0", repo.updateCalls)
	}
	if len(entitlements.calls) != 1 ||
		entitlements.calls[0] != domain.CapabilityItemUpdate {
		t.Fatalf("Update() authorization calls = %v, want item.update", entitlements.calls)
	}
}

func TestValidateSCIMTokenRechecksLiveEntitlement(t *testing.T) {
	repo := &scimTokenValidationRepoStub{
		token: &domain.SCIMToken{OrganizationID: 7, IsActive: true},
	}
	entitlements := &entitlementAuthorizationStub{
		errors: map[domain.Capability]error{
			domain.CapabilitySCIMDeprovision: ErrFeatureNotAvailable,
		},
	}
	service := &scimService{
		tokenRepo:    repo,
		entitlements: entitlements,
	}

	_, err := service.ValidateToken(context.Background(), "pwscim_test")
	if !errors.Is(err, ErrFeatureNotAvailable) {
		t.Fatalf("ValidateToken() error = %v, want ErrFeatureNotAvailable", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("ValidateToken() updated last-used %d times, want 0", repo.updateCalls)
	}
}

func TestSCIMGroupMutationRequiresTeamsCapability(t *testing.T) {
	teams := &scimTeamCreateRepoStub{}
	entitlements := &entitlementAuthorizationStub{
		errors: map[domain.Capability]error{
			domain.CapabilityTeamsManage: ErrFeatureNotAvailable,
		},
	}
	service := &scimService{
		teamRepo:     teams,
		entitlements: entitlements,
	}

	_, err := service.CreateGroup(
		context.Background(),
		7,
		&domain.SCIMGroup{DisplayName: "Engineering"},
	)
	if !errors.Is(err, ErrFeatureNotAvailable) {
		t.Fatalf("CreateGroup() error = %v, want ErrFeatureNotAvailable", err)
	}
	if teams.createCalls != 0 {
		t.Fatalf("CreateGroup() persisted %d times, want 0", teams.createCalls)
	}
	if len(entitlements.calls) != 2 ||
		entitlements.calls[0] != domain.CapabilitySCIMManage ||
		entitlements.calls[1] != domain.CapabilityTeamsManage {
		t.Fatalf(
			"CreateGroup() authorization calls = %v, want scim.manage then teams.manage",
			entitlements.calls,
		)
	}
}
