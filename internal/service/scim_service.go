package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

var (
	ErrSCIMTokenInvalid        = errors.New("invalid SCIM token")
	ErrSCIMUserNotFound        = errors.New("SCIM user not found")
	ErrSCIMGroupNotFound       = errors.New("SCIM group not found")
	ErrSCIMUserExists          = errors.New("SCIM user already exists in organization")
	ErrSCIMProvisioningBlocked = errors.New("SCIM auto-provisioning is blocked until secure org-key exchange is implemented")
	ErrSCIMInvalidFilter       = errors.New("unsupported SCIM filter")
	ErrSCIMLastOwner           = errors.New("the last owner of an organization cannot be deprovisioned through SCIM")
	ErrSCIMDomainNotVerified   = errors.New("the user's email domain is not verified for this organization; invite the user from Passwall instead")
)

// pendingOrgKey marks a membership that has not received the org key yet.
const pendingOrgKey = "pending_key_exchange"

// SCIMService handles SCIM 2.0 provisioning operations
type SCIMService interface {
	// Token management
	CreateToken(ctx context.Context, orgID uint, req *domain.CreateSCIMTokenRequest) (*domain.SCIMTokenCreatedDTO, error)
	ListTokens(ctx context.Context, orgID uint) ([]*domain.SCIMTokenDTO, error)
	RevokeToken(ctx context.Context, orgID, tokenID uint) error
	ValidateToken(ctx context.Context, bearerToken string) (orgID uint, err error)

	// SCIM User operations
	ListUsers(ctx context.Context, orgID uint, filter string, startIndex, count int) (*domain.SCIMListResponse, error)
	GetUser(ctx context.Context, orgID uint, userID string) (*domain.SCIMUser, error)
	CreateUser(ctx context.Context, orgID uint, scimUser *domain.SCIMUser) (*domain.SCIMUser, error)
	UpdateUser(ctx context.Context, orgID uint, userID string, scimUser *domain.SCIMUser) (*domain.SCIMUser, error)
	PatchUser(ctx context.Context, orgID uint, userID string, patch *domain.SCIMPatchOp) (*domain.SCIMUser, error)
	DeleteUser(ctx context.Context, orgID uint, userID string) error

	// SCIM Group operations
	ListGroups(ctx context.Context, orgID uint, filter string, startIndex, count int) (*domain.SCIMListResponse, error)
	GetGroup(ctx context.Context, orgID uint, groupID string) (*domain.SCIMGroup, error)
	CreateGroup(ctx context.Context, orgID uint, scimGroup *domain.SCIMGroup) (*domain.SCIMGroup, error)
	UpdateGroup(ctx context.Context, orgID uint, groupID string, scimGroup *domain.SCIMGroup) (*domain.SCIMGroup, error)
	PatchGroup(ctx context.Context, orgID uint, groupID string, patch *domain.SCIMPatchOp) (*domain.SCIMGroup, error)
	DeleteGroup(ctx context.Context, orgID uint, groupID string) error
}

type scimService struct {
	connRepo     repository.SSOConnectionRepository
	joinPolicies interface {
		CheckJoinPolicies(ctx context.Context, orgID, userID uint) error
	}
	tokenRepo    repository.SCIMTokenRepository
	userRepo     repository.UserRepository
	orgUserRepo  repository.OrganizationUserRepository
	teamRepo     repository.TeamRepository
	teamUserRepo repository.TeamUserRepository
	logger       Logger
	baseURL      string
	entitlements OrganizationEntitlementService
}

// NewSCIMService creates a new SCIM service
func NewSCIMService(
	tokenRepo repository.SCIMTokenRepository,
	userRepo repository.UserRepository,
	orgUserRepo repository.OrganizationUserRepository,
	teamRepo repository.TeamRepository,
	teamUserRepo repository.TeamUserRepository,
	logger Logger,
	baseURL string,
	entitlements ...OrganizationEntitlementService,
) SCIMService {
	service := &scimService{
		tokenRepo:    tokenRepo,
		userRepo:     userRepo,
		orgUserRepo:  orgUserRepo,
		teamRepo:     teamRepo,
		teamUserRepo: teamUserRepo,
		logger:       logger,
		baseURL:      baseURL,
	}
	if len(entitlements) > 0 {
		service.entitlements = entitlements[0]
	}
	return service
}

// WithProvisioningGuards adds the verified-domain and join-policy checks
// used when SCIM creates memberships.
func WithProvisioningGuards(svc SCIMService, connRepo repository.SSOConnectionRepository, joinPolicies interface {
	CheckJoinPolicies(ctx context.Context, orgID, userID uint) error
}) SCIMService {
	if s, ok := svc.(*scimService); ok {
		s.connRepo = connRepo
		s.joinPolicies = joinPolicies
	}
	return svc
}

// --- Token Management ---

func (s *scimService) CreateToken(ctx context.Context, orgID uint, req *domain.CreateSCIMTokenRequest) (*domain.SCIMTokenCreatedDTO, error) {
	if err := s.authorizeSCIMMutation(ctx, orgID); err != nil {
		return nil, err
	}
	plainToken, err := domain.GenerateSCIMToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate SCIM token: %w", err)
	}

	token := &domain.SCIMToken{
		UUID:           uuid.New(),
		OrganizationID: orgID,
		Label:          req.Label,
		TokenHash:      hashToken(plainToken),
		ExpiresAt:      req.ExpiresAt,
		IsActive:       true,
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to create SCIM token: %w", err)
	}

	s.logger.Info("SCIM token created", "org_id", orgID, "label", req.Label)

	dto := domain.ToSCIMTokenDTO(token)
	return &domain.SCIMTokenCreatedDTO{
		SCIMTokenDTO: *dto,
		Token:        plainToken,
	}, nil
}

func (s *scimService) ListTokens(ctx context.Context, orgID uint) ([]*domain.SCIMTokenDTO, error) {
	tokens, err := s.tokenRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}
	dtos := make([]*domain.SCIMTokenDTO, len(tokens))
	for i, t := range tokens {
		dtos[i] = domain.ToSCIMTokenDTO(t)
	}
	return dtos, nil
}

func (s *scimService) RevokeToken(ctx context.Context, orgID, tokenID uint) error {
	if err := s.authorizeSCIM(ctx, orgID, domain.CapabilityAccessRevoke); err != nil {
		return err
	}
	token, err := s.tokenRepo.GetByID(ctx, tokenID)
	if err != nil {
		return err
	}
	if token.OrganizationID != orgID {
		return repository.ErrForbidden
	}
	token.IsActive = false
	return s.tokenRepo.Update(ctx, token)
}

func (s *scimService) authorizeSCIMMutation(ctx context.Context, orgID uint) error {
	return s.authorizeSCIM(ctx, orgID, domain.CapabilitySCIMManage)
}

// authorizeSCIM checks every capability in order. Deprovisioning paths use
// scim.deprovision so an IdP can always remove access, even while frozen.
func (s *scimService) authorizeSCIM(ctx context.Context, orgID uint, capabilities ...domain.Capability) error {
	if s.entitlements == nil {
		return nil
	}
	for _, capability := range capabilities {
		if err := s.entitlements.Authorize(ctx, orgID, capability); err != nil {
			return err
		}
	}
	return nil
}

func (s *scimService) ValidateToken(ctx context.Context, bearerToken string) (uint, error) {
	h := hashToken(bearerToken)
	token, err := s.tokenRepo.GetByTokenHash(ctx, h)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, ErrSCIMTokenInvalid
		}
		return 0, err
	}
	if !token.IsValid() {
		return 0, ErrSCIMTokenInvalid
	}
	if err := s.authorizeSCIM(ctx, token.OrganizationID, domain.CapabilitySCIMDeprovision); err != nil {
		return 0, err
	}

	// Update last_used_at
	now := time.Now()
	token.LastUsedAt = &now
	_ = s.tokenRepo.Update(ctx, token)

	return token.OrganizationID, nil
}

// --- SCIM User Operations ---

func (s *scimService) ListUsers(ctx context.Context, orgID uint, filter string, startIndex, count int) (*domain.SCIMListResponse, error) {
	if startIndex < 1 {
		startIndex = 1
	}
	if count < 1 || count > 100 {
		count = 100
	}

	orgUsers, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if filter != "" {
		orgUsers, err = s.filterOrgUsers(orgUsers, filter)
		if err != nil {
			return nil, err
		}
	}

	total := len(orgUsers)

	// Pagination
	start := startIndex - 1
	if start > total {
		start = total
	}
	end := start + count
	if end > total {
		end = total
	}
	paged := orgUsers[start:end]

	resources := make([]*domain.SCIMUser, 0, len(paged))
	for _, ou := range paged {
		scimUser := s.orgUserToSCIMUser(ou, orgID)
		if scimUser != nil {
			resources = append(resources, scimUser)
		}
	}

	return &domain.SCIMListResponse{
		Schemas:      []string{domain.SCIMSchemaListResponse},
		TotalResults: total,
		StartIndex:   startIndex,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}, nil
}

func (s *scimService) GetUser(ctx context.Context, orgID uint, userID string) (*domain.SCIMUser, error) {
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return nil, ErrSCIMUserNotFound
	}

	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, uint(id))
	if err != nil {
		return nil, ErrSCIMUserNotFound
	}

	scimUser := s.orgUserToSCIMUser(orgUser, orgID)
	if scimUser == nil {
		return nil, ErrSCIMUserNotFound
	}
	return scimUser, nil
}

func (s *scimService) CreateUser(ctx context.Context, orgID uint, scimUser *domain.SCIMUser) (*domain.SCIMUser, error) {
	if err := s.authorizeSCIMMutation(ctx, orgID); err != nil {
		return nil, err
	}
	email := extractPrimaryEmail(scimUser)
	if email == "" {
		return nil, fmt.Errorf("SCIM user must have a primary email")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if err := s.ensureVerifiedDomain(ctx, orgID, email); err != nil {
		return nil, err
	}

	existingUser, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		s.logger.Info("SCIM CreateUser: user not found in system, cannot provision", "org_id", orgID)
		return nil, fmt.Errorf("user %s does not have a Passwall account yet; they must sign up first", email)
	}

	_, err = s.orgUserRepo.GetByOrgAndUser(ctx, orgID, existingUser.ID)
	if err == nil {
		return nil, ErrSCIMUserExists
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if s.joinPolicies != nil {
		if err := s.joinPolicies.CheckJoinPolicies(ctx, orgID, existingUser.ID); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	orgUser := &domain.OrganizationUser{
		UUID:            uuid.New(),
		OrganizationID:  orgID,
		UserID:          existingUser.ID,
		Role:            domain.OrgRoleMember,
		EncryptedOrgKey: pendingOrgKey,
		AccessAll:       false,
		Status:          domain.OrgUserStatusProvisioned,
		InvitedAt:       &now,
	}
	if scimUser.ExternalID != "" {
		orgUser.ExternalID = ptrString(scimUser.ExternalID)
	}

	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, orgID, domain.CapabilityMemberInvite); err != nil {
			return nil, err
		}
	}
	if err := s.orgUserRepo.Create(ctx, orgUser); err != nil {
		return nil, fmt.Errorf("failed to provision SCIM user: %w", err)
	}

	s.logger.Info("SCIM user provisioned", "user_id", existingUser.ID, "org_id", orgID, "status", "provisioned")

	orgUser.User = existingUser
	return s.orgUserToSCIMUser(orgUser, orgID), nil
}

func (s *scimService) UpdateUser(ctx context.Context, orgID uint, userID string, scimUser *domain.SCIMUser) (*domain.SCIMUser, error) {
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return nil, ErrSCIMUserNotFound
	}

	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, uint(id))
	if err != nil {
		return nil, ErrSCIMUserNotFound
	}

	// Update external ID if changed
	if scimUser.ExternalID != "" {
		orgUser.ExternalID = ptrString(scimUser.ExternalID)
	}

	// Handle active/inactive (suspend/reactivate)
	if !scimUser.Active && orgUser.Status != domain.OrgUserStatusSuspended {
		if err := s.authorizeSCIM(ctx, orgID, domain.CapabilitySCIMDeprovision); err != nil {
			return nil, err
		}
		if err := s.ensureNotLastOwner(ctx, orgUser); err != nil {
			return nil, err
		}
		orgUser.Status = domain.OrgUserStatusSuspended
	} else if scimUser.Active && orgUser.Status == domain.OrgUserStatusSuspended {
		if err := s.authorizeSCIM(ctx, orgID, domain.CapabilitySCIMManage, domain.CapabilityMemberInvite); err != nil {
			return nil, err
		}
		orgUser.Status = reactivatedStatus(orgUser)
	} else if err := s.authorizeSCIMMutation(ctx, orgID); err != nil {
		return nil, err
	}

	if err := s.orgUserRepo.Update(ctx, orgUser); err != nil {
		return nil, fmt.Errorf("failed to update SCIM user: %w", err)
	}

	return s.orgUserToSCIMUser(orgUser, orgID), nil
}

func (s *scimService) PatchUser(ctx context.Context, orgID uint, userID string, patch *domain.SCIMPatchOp) (*domain.SCIMUser, error) {
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return nil, ErrSCIMUserNotFound
	}

	current, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, uint(id))
	if err != nil {
		return nil, ErrSCIMUserNotFound
	}
	// Apply the operations to a copy; nothing changes unless all checks pass.
	updated := *current
	orgUser := &updated
	wasActive := orgUser.Status == domain.OrgUserStatusAccepted ||
		orgUser.Status == domain.OrgUserStatusConfirmed

	setActive := func(active bool) {
		switch {
		case !active:
			orgUser.Status = domain.OrgUserStatusSuspended
		case orgUser.Status == domain.OrgUserStatusSuspended:
			orgUser.Status = reactivatedStatus(orgUser)
		}
	}
	for _, op := range patch.Operations {
		opName := strings.ToLower(op.Op)
		if opName != "replace" && opName != "add" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(op.Path)) {
		case "active":
			setActive(parseBoolValue(op.Value))
		case "externalid":
			if v, ok := op.Value.(string); ok {
				orgUser.ExternalID = ptrString(v)
			}
		case "":
			// Path-less op: the value is a partial resource. Only touch
			// attributes that are present.
			values, ok := op.Value.(map[string]interface{})
			if !ok {
				continue
			}
			for key, value := range values {
				switch strings.ToLower(key) {
				case "active":
					setActive(parseBoolValue(value))
				case "externalid":
					if v, ok := value.(string); ok {
						orgUser.ExternalID = ptrString(v)
					}
				}
			}
		}
	}
	isActive := orgUser.Status == domain.OrgUserStatusAccepted ||
		orgUser.Status == domain.OrgUserStatusConfirmed
	switch {
	case !wasActive && isActive:
		if err := s.authorizeSCIM(ctx, orgID, domain.CapabilitySCIMManage, domain.CapabilityMemberInvite); err != nil {
			return nil, err
		}
	case orgUser.Status == domain.OrgUserStatusSuspended && wasActive:
		if err := s.authorizeSCIM(ctx, orgID, domain.CapabilitySCIMDeprovision); err != nil {
			return nil, err
		}
		if err := s.ensureNotLastOwner(ctx, orgUser); err != nil {
			return nil, err
		}
	default:
		if err := s.authorizeSCIMMutation(ctx, orgID); err != nil {
			return nil, err
		}
	}

	if err := s.orgUserRepo.Update(ctx, orgUser); err != nil {
		return nil, fmt.Errorf("failed to patch SCIM user: %w", err)
	}

	return s.orgUserToSCIMUser(orgUser, orgID), nil
}

func (s *scimService) DeleteUser(ctx context.Context, orgID uint, userID string) error {
	if err := s.authorizeSCIM(ctx, orgID, domain.CapabilitySCIMDeprovision, domain.CapabilityMemberRemove); err != nil {
		return err
	}
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return ErrSCIMUserNotFound
	}

	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, uint(id))
	if err != nil {
		return ErrSCIMUserNotFound
	}

	if err := s.ensureNotLastOwner(ctx, orgUser); err != nil {
		return err
	}
	s.logger.Info("SCIM deprovisioning user from org", "user_id", id, "org_id", orgID)
	return s.orgUserRepo.Delete(ctx, orgUser.ID)
}

// --- SCIM Group Operations ---

func (s *scimService) ListGroups(ctx context.Context, orgID uint, filter string, startIndex, count int) (*domain.SCIMListResponse, error) {
	if startIndex < 1 {
		startIndex = 1
	}
	if count < 1 || count > 100 {
		count = 100
	}

	teams, err := s.teamRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}

	total := len(teams)
	start := startIndex - 1
	if start > total {
		start = total
	}
	end := start + count
	if end > total {
		end = total
	}

	resources := make([]*domain.SCIMGroup, 0, end-start)
	for _, team := range teams[start:end] {
		resources = append(resources, s.teamToSCIMGroup(team, orgID))
	}

	return &domain.SCIMListResponse{
		Schemas:      []string{domain.SCIMSchemaListResponse},
		TotalResults: total,
		StartIndex:   startIndex,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}, nil
}

func (s *scimService) GetGroup(ctx context.Context, orgID uint, groupID string) (*domain.SCIMGroup, error) {
	id, err := strconv.ParseUint(groupID, 10, 64)
	if err != nil {
		return nil, ErrSCIMGroupNotFound
	}

	team, err := s.teamRepo.GetByID(ctx, uint(id))
	if err != nil || team.OrganizationID != orgID {
		return nil, ErrSCIMGroupNotFound
	}

	return s.teamToSCIMGroup(team, orgID), nil
}

func (s *scimService) CreateGroup(ctx context.Context, orgID uint, scimGroup *domain.SCIMGroup) (*domain.SCIMGroup, error) {
	if err := s.authorizeSCIMGroupMutation(ctx, orgID); err != nil {
		return nil, err
	}
	team := &domain.Team{
		UUID:           uuid.New(),
		OrganizationID: orgID,
		Name:           scimGroup.DisplayName,
	}
	if scimGroup.ExternalID != "" {
		team.ExternalID = &scimGroup.ExternalID
	}

	if err := s.teamRepo.Create(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to create SCIM group: %w", err)
	}

	s.logger.Info("SCIM group created", "team_id", team.ID, "name", team.Name, "org_id", orgID)
	return s.teamToSCIMGroup(team, orgID), nil
}

func (s *scimService) UpdateGroup(ctx context.Context, orgID uint, groupID string, scimGroup *domain.SCIMGroup) (*domain.SCIMGroup, error) {
	if err := s.authorizeSCIMGroupMutation(ctx, orgID); err != nil {
		return nil, err
	}
	id, err := strconv.ParseUint(groupID, 10, 64)
	if err != nil {
		return nil, ErrSCIMGroupNotFound
	}

	team, err := s.teamRepo.GetByID(ctx, uint(id))
	if err != nil || team.OrganizationID != orgID {
		return nil, ErrSCIMGroupNotFound
	}

	team.Name = scimGroup.DisplayName
	if scimGroup.ExternalID != "" {
		team.ExternalID = &scimGroup.ExternalID
	}

	if err := s.teamRepo.Update(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to update SCIM group: %w", err)
	}

	// Sync members
	if err := s.syncGroupMembers(ctx, team, scimGroup.Members); err != nil {
		s.logger.Error("failed to sync SCIM group members", "error", err)
	}

	return s.teamToSCIMGroup(team, orgID), nil
}

func (s *scimService) PatchGroup(ctx context.Context, orgID uint, groupID string, patch *domain.SCIMPatchOp) (*domain.SCIMGroup, error) {
	if err := s.authorizeSCIMGroupMutation(ctx, orgID); err != nil {
		return nil, err
	}
	id, err := strconv.ParseUint(groupID, 10, 64)
	if err != nil {
		return nil, ErrSCIMGroupNotFound
	}

	team, err := s.teamRepo.GetByID(ctx, uint(id))
	if err != nil || team.OrganizationID != orgID {
		return nil, ErrSCIMGroupNotFound
	}

	for _, op := range patch.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			if op.Path == "displayName" {
				if v, ok := op.Value.(string); ok {
					team.Name = v
				}
			}
		case "add":
			if op.Path == "members" {
				s.handleGroupMemberAdd(ctx, team, op.Value)
			}
		case "remove":
			if strings.HasPrefix(op.Path, "members") {
				s.handleGroupMemberRemove(ctx, team, op.Path)
			}
		}
	}

	if err := s.teamRepo.Update(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to patch SCIM group: %w", err)
	}

	return s.teamToSCIMGroup(team, orgID), nil
}

func (s *scimService) DeleteGroup(ctx context.Context, orgID uint, groupID string) error {
	if err := s.authorizeSCIM(ctx, orgID, domain.CapabilitySCIMDeprovision, domain.CapabilityAccessRevoke); err != nil {
		return err
	}
	id, err := strconv.ParseUint(groupID, 10, 64)
	if err != nil {
		return ErrSCIMGroupNotFound
	}

	team, err := s.teamRepo.GetByID(ctx, uint(id))
	if err != nil || team.OrganizationID != orgID {
		return ErrSCIMGroupNotFound
	}

	s.logger.Info("SCIM group deleted", "team_id", team.ID, "org_id", orgID)
	return s.teamRepo.Delete(ctx, team.ID)
}

func (s *scimService) authorizeSCIMGroupMutation(ctx context.Context, orgID uint) error {
	return s.authorizeSCIM(ctx, orgID, domain.CapabilitySCIMManage, domain.CapabilityTeamsManage)
}

// --- Helpers ---

func (s *scimService) orgUserToSCIMUser(ou *domain.OrganizationUser, orgID uint) *domain.SCIMUser {
	if ou == nil || ou.User == nil {
		return nil
	}
	u := ou.User

	scimUser := &domain.SCIMUser{
		Schemas:  []string{domain.SCIMSchemaUser},
		ID:       strconv.FormatUint(uint64(u.ID), 10),
		UserName: u.Email,
		Active:   ou.Status != domain.OrgUserStatusSuspended,
		Emails: []domain.SCIMEmail{
			{Value: u.Email, Type: "work", Primary: true},
		},
		Meta: &domain.SCIMMeta{
			ResourceType: "User",
			Created:      u.CreatedAt.Format(time.RFC3339),
			LastModified: u.UpdatedAt.Format(time.RFC3339),
			Location:     fmt.Sprintf("%s/scim/v2/Users/%d", s.baseURL, u.ID),
		},
	}

	if u.Name != "" {
		scimUser.Name = &domain.SCIMName{
			Formatted: u.Name,
		}
	}

	if ou.ExternalID != nil {
		scimUser.ExternalID = *ou.ExternalID
	}

	return scimUser
}

func (s *scimService) teamToSCIMGroup(team *domain.Team, orgID uint) *domain.SCIMGroup {
	group := &domain.SCIMGroup{
		Schemas:     []string{domain.SCIMSchemaGroup},
		ID:          strconv.FormatUint(uint64(team.ID), 10),
		DisplayName: team.Name,
		Meta: &domain.SCIMMeta{
			ResourceType: "Group",
			Created:      team.CreatedAt.Format(time.RFC3339),
			LastModified: team.UpdatedAt.Format(time.RFC3339),
			Location:     fmt.Sprintf("%s/scim/v2/Groups/%d", s.baseURL, team.ID),
		},
	}
	if team.ExternalID != nil {
		group.ExternalID = *team.ExternalID
	}

	// Populate members if loaded
	if team.Members != nil {
		for _, m := range team.Members {
			if m.OrganizationUser != nil && m.OrganizationUser.User != nil {
				group.Members = append(group.Members, domain.SCIMMemberRef{
					Value:   strconv.FormatUint(uint64(m.OrganizationUser.UserID), 10),
					Display: m.OrganizationUser.User.Email,
				})
			}
		}
	}

	return group
}

func (s *scimService) syncGroupMembers(ctx context.Context, team *domain.Team, members []domain.SCIMMemberRef) error {
	// Get current members
	currentMembers, err := s.teamUserRepo.ListByTeam(ctx, team.ID)
	if err != nil {
		return err
	}

	currentMap := make(map[uint]bool)
	for _, m := range currentMembers {
		currentMap[m.OrganizationUserID] = true
	}

	desiredMap := make(map[uint]bool)
	for _, m := range members {
		uid, err := strconv.ParseUint(m.Value, 10, 64)
		if err != nil {
			continue
		}

		orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, team.OrganizationID, uint(uid))
		if err != nil {
			continue
		}
		desiredMap[orgUser.ID] = true

		if !currentMap[orgUser.ID] {
			tu := &domain.TeamUser{
				TeamID:             team.ID,
				OrganizationUserID: orgUser.ID,
			}
			_ = s.teamUserRepo.Create(ctx, tu)
		}
	}

	// Remove members not in desired set
	for _, m := range currentMembers {
		if !desiredMap[m.OrganizationUserID] {
			_ = s.teamUserRepo.Delete(ctx, m.ID)
		}
	}

	return nil
}

func (s *scimService) handleGroupMemberAdd(ctx context.Context, team *domain.Team, value interface{}) {
	members, ok := value.([]interface{})
	if !ok {
		return
	}
	for _, m := range members {
		mMap, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		userIDStr, ok := mMap["value"].(string)
		if !ok {
			continue
		}
		uid, err := strconv.ParseUint(userIDStr, 10, 64)
		if err != nil {
			continue
		}
		orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, team.OrganizationID, uint(uid))
		if err != nil {
			continue
		}
		tu := &domain.TeamUser{TeamID: team.ID, OrganizationUserID: orgUser.ID}
		_ = s.teamUserRepo.Create(ctx, tu)
	}
}

func (s *scimService) handleGroupMemberRemove(ctx context.Context, team *domain.Team, path string) {
	// path format: members[value eq "123"]
	start := strings.Index(path, `"`)
	end := strings.LastIndex(path, `"`)
	if start < 0 || end <= start {
		return
	}
	userIDStr := path[start+1 : end]
	uid, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return
	}
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, team.OrganizationID, uint(uid))
	if err != nil {
		return
	}
	_ = s.teamUserRepo.DeleteByTeamAndOrgUser(ctx, team.ID, orgUser.ID)
}

// filterOrgUsers supports the equality filters IdPs use to look users up:
// userName, externalId and emails.value. Anything else is rejected rather
// than ignored, so an IdP never mistakes "all users" for a match.
func (s *scimService) filterOrgUsers(orgUsers []*domain.OrganizationUser, filter string) ([]*domain.OrganizationUser, error) {
	attr, value, ok := parseSCIMEqFilter(filter)
	if !ok {
		return nil, ErrSCIMInvalidFilter
	}
	var result []*domain.OrganizationUser
	for _, ou := range orgUsers {
		if ou.User == nil {
			continue
		}
		switch attr {
		case "username", "emails.value", "emails[type eq \"work\"].value":
			if strings.EqualFold(ou.User.Email, value) {
				result = append(result, ou)
			}
		case "externalid":
			if ou.ExternalID != nil && *ou.ExternalID == value {
				result = append(result, ou)
			}
		default:
			return nil, ErrSCIMInvalidFilter
		}
	}
	return result, nil
}

// parseSCIMEqFilter parses `<attr> eq "<value>"` (attribute is lowercased).
func parseSCIMEqFilter(filter string) (string, string, bool) {
	filter = strings.TrimSpace(filter)
	idx := strings.Index(strings.ToLower(filter), " eq ")
	if idx <= 0 {
		return "", "", false
	}
	attr := strings.ToLower(strings.TrimSpace(filter[:idx]))
	value := strings.TrimSpace(filter[idx+4:])
	if len(value) < 2 || !strings.HasPrefix(value, `"`) || !strings.HasSuffix(value, `"`) {
		return "", "", false
	}
	value = strings.ReplaceAll(value[1:len(value)-1], `\"`, `"`)
	return attr, value, true
}

// reactivatedStatus restores a suspended member. Members who never received
// the org key go back to "provisioned" so an admin still has to confirm them.
func reactivatedStatus(ou *domain.OrganizationUser) domain.OrganizationUserStatus {
	key := strings.TrimSpace(ou.EncryptedOrgKey)
	if key == "" || key == pendingOrgKey {
		return domain.OrgUserStatusProvisioned
	}
	return domain.OrgUserStatusConfirmed
}

// ensureNotLastOwner keeps the IdP from removing an organization's last
// active owner.
func (s *scimService) ensureNotLastOwner(ctx context.Context, target *domain.OrganizationUser) error {
	if target.Role != domain.OrgRoleOwner {
		return nil
	}
	members, err := s.orgUserRepo.ListByOrganization(ctx, target.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to check owners: %w", err)
	}
	for _, m := range members {
		if m.ID == target.ID || m.Role != domain.OrgRoleOwner {
			continue
		}
		if m.Status == domain.OrgUserStatusAccepted || m.Status == domain.OrgUserStatusConfirmed {
			return nil
		}
	}
	return ErrSCIMLastOwner
}

// ensureVerifiedDomain requires the email's domain to be verified by an SSO
// connection of this organization before SCIM adds the user without an
// invitation.
func (s *scimService) ensureVerifiedDomain(ctx context.Context, orgID uint, email string) error {
	if s.connRepo == nil {
		return ErrSCIMDomainNotVerified
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return fmt.Errorf("SCIM user must have a valid email")
	}
	conn, err := s.connRepo.GetVerifiedByDomain(ctx, email[at+1:])
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSCIMDomainNotVerified
		}
		return err
	}
	if conn.OrganizationID != orgID {
		return ErrSCIMDomainNotVerified
	}
	return nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func parseBoolValue(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.ToLower(val) == "true"
	case map[string]interface{}:
		if active, ok := val["active"]; ok {
			return parseBoolValue(active)
		}
	}
	return false
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func extractPrimaryEmail(u *domain.SCIMUser) string {
	for _, e := range u.Emails {
		if e.Primary {
			return e.Value
		}
	}
	if len(u.Emails) > 0 {
		return u.Emails[0].Value
	}
	return u.UserName
}
