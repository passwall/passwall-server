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

	uuid "github.com/satori/go.uuid"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

var (
	ErrSCIMTokenInvalid   = errors.New("invalid SCIM token")
	ErrSCIMUserNotFound   = errors.New("SCIM user not found")
	ErrSCIMGroupNotFound  = errors.New("SCIM group not found")
	ErrSCIMUserExists     = errors.New("SCIM user already exists in organization")
	ErrSCIMProvisioningBlocked = errors.New("SCIM auto-provisioning is blocked until secure org-key exchange is implemented")
)

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
	tokenRepo   repository.SCIMTokenRepository
	userRepo    repository.UserRepository
	orgUserRepo repository.OrganizationUserRepository
	teamRepo    repository.TeamRepository
	teamUserRepo repository.TeamUserRepository
	logger      Logger
	baseURL     string
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
) SCIMService {
	return &scimService{
		tokenRepo:    tokenRepo,
		userRepo:     userRepo,
		orgUserRepo:  orgUserRepo,
		teamRepo:     teamRepo,
		teamUserRepo: teamUserRepo,
		logger:       logger,
		baseURL:      baseURL,
	}
}

// --- Token Management ---

func (s *scimService) CreateToken(ctx context.Context, orgID uint, req *domain.CreateSCIMTokenRequest) (*domain.SCIMTokenCreatedDTO, error) {
	plainToken, err := domain.GenerateSCIMToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate SCIM token: %w", err)
	}

	token := &domain.SCIMToken{
		UUID:           uuid.NewV4(),
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

	// Apply filter if present (basic "userName eq" support)
	if filter != "" {
		orgUsers = s.filterOrgUsers(orgUsers, filter)
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
	email := extractPrimaryEmail(scimUser)
	if email == "" {
		return nil, fmt.Errorf("SCIM user must have a primary email")
	}

	// Check if user already exists in the system
	existingUser, err := s.userRepo.GetByEmail(ctx, strings.ToLower(email))
	if err == nil {
		// User exists in system, check org membership
		_, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, existingUser.ID)
		if err == nil {
			return nil, ErrSCIMUserExists
		}
		return nil, ErrSCIMProvisioningBlocked
	}
	return nil, ErrSCIMProvisioningBlocked
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
		orgUser.Status = domain.OrgUserStatusSuspended
	} else if scimUser.Active && orgUser.Status == domain.OrgUserStatusSuspended {
		orgUser.Status = domain.OrgUserStatusConfirmed
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

	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, uint(id))
	if err != nil {
		return nil, ErrSCIMUserNotFound
	}

	for _, op := range patch.Operations {
		switch strings.ToLower(op.Op) {
		case "replace":
			if op.Path == "active" || op.Path == "" {
				active := parseBoolValue(op.Value)
				if !active {
					orgUser.Status = domain.OrgUserStatusSuspended
				} else {
					orgUser.Status = domain.OrgUserStatusConfirmed
				}
			}
			if op.Path == "externalId" {
				if v, ok := op.Value.(string); ok {
					orgUser.ExternalID = &v
				}
			}
		}
	}

	if err := s.orgUserRepo.Update(ctx, orgUser); err != nil {
		return nil, fmt.Errorf("failed to patch SCIM user: %w", err)
	}

	return s.orgUserToSCIMUser(orgUser, orgID), nil
}

func (s *scimService) DeleteUser(ctx context.Context, orgID uint, userID string) error {
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return ErrSCIMUserNotFound
	}

	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, uint(id))
	if err != nil {
		return ErrSCIMUserNotFound
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
	team := &domain.Team{
		UUID:           uuid.NewV4(),
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

func (s *scimService) filterOrgUsers(orgUsers []*domain.OrganizationUser, filter string) []*domain.OrganizationUser {
	// Basic SCIM filter: userName eq "user@example.com"
	filter = strings.TrimSpace(filter)
	if strings.HasPrefix(filter, `userName eq "`) && strings.HasSuffix(filter, `"`) {
		email := filter[len(`userName eq "`) : len(filter)-1]
		email = strings.ToLower(email)
		var result []*domain.OrganizationUser
		for _, ou := range orgUsers {
			if ou.User != nil && strings.ToLower(ou.User.Email) == email {
				result = append(result, ou)
			}
		}
		return result
	}
	return orgUsers
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
