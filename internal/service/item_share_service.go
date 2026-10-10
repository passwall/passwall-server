package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/authz"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
)

// ErrShareInviteSent indicates a signup email was sent for non-registered recipient.
var ErrShareInviteSent = errors.New("share invite email sent")

// ErrSecureSharingUnavailable is returned for shares outside the item's
// organization. Today a share hands the recipient a key that decrypts the
// whole vault, so it is only allowed between members who already hold that
// key. Sharing with anyone else returns with per-item keys.
var ErrSecureSharingUnavailable = errors.New("sharing outside the organization is temporarily unavailable")

// CreateItemShareRequest represents a request to share an organization item.
type CreateItemShareRequest struct {
	ItemUUID         string
	SharedWithUserID *uint
	SharedWithEmail  string
	CanView          *bool
	CanEdit          *bool
	CanShare         *bool
	EncryptedKey     string
	ExpiresAt        *time.Time
}

// UpdateSharedItemRequest represents updates from a share recipient.
type UpdateSharedItemRequest struct {
	Data     string
	Metadata domain.ItemMetadata
}

// UpdateItemSharePermissionsRequest represents an owner-side update to share permissions/expiry.
// Note: CanView is always enforced as true for valid shares.
type UpdateItemSharePermissionsRequest struct {
	CanEdit        *bool
	CanShare       *bool
	ExpiresAt      *time.Time
	ClearExpiresAt bool
}

// ItemShareWithItem bundles share + organization item data for responses.
type ItemShareWithItem struct {
	Share *domain.ItemShare
	Item  *domain.OrganizationItem
}

type itemShareItemRepository interface {
	GetByUUID(ctx context.Context, uuid string) (*domain.OrganizationItem, error)
	Update(ctx context.Context, item *domain.OrganizationItem) error
}

type activeOrganizationMembershipReader interface {
	GetActiveByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error)
}

type itemShareService struct {
	shareRepo       repository.ItemShareRepository
	orgItemRepo     itemShareItemRepository
	orgUserRepo     activeOrganizationMembershipReader
	collectionUsers authz.CollectionUserAccessReader
	collectionTeams authz.CollectionTeamAccessReader
	teamMemberships authz.TeamMembershipReader
	userRepo        repository.UserRepository
	emailSender     email.Sender
	emailBuilder    *email.EmailBuilder
	logger          Logger
	entitlements    OrganizationEntitlementService
}

func NewItemShareService(
	shareRepo repository.ItemShareRepository,
	orgItemRepo itemShareItemRepository,
	orgUserRepo activeOrganizationMembershipReader,
	collectionUsers authz.CollectionUserAccessReader,
	collectionTeams authz.CollectionTeamAccessReader,
	teamMemberships authz.TeamMembershipReader,
	userRepo repository.UserRepository,
	emailSender email.Sender,
	emailBuilder *email.EmailBuilder,
	logger Logger,
	entitlements ...OrganizationEntitlementService,
) ItemShareService {
	service := &itemShareService{
		shareRepo:       shareRepo,
		orgItemRepo:     orgItemRepo,
		orgUserRepo:     orgUserRepo,
		collectionUsers: collectionUsers,
		collectionTeams: collectionTeams,
		teamMemberships: teamMemberships,
		userRepo:        userRepo,
		emailSender:     emailSender,
		emailBuilder:    emailBuilder,
		logger:          logger,
	}
	if len(entitlements) > 0 {
		service.entitlements = entitlements[0]
	}
	return service
}

func (s *itemShareService) Create(ctx context.Context, ownerID uint, req *CreateItemShareRequest) (*ItemShareWithItem, error) {
	if strings.TrimSpace(req.ItemUUID) == "" {
		return nil, repository.ErrInvalidInput
	}
	if req.SharedWithUserID == nil && strings.TrimSpace(req.SharedWithEmail) == "" {
		return nil, repository.ErrInvalidInput
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		return nil, repository.ErrInvalidInput
	}

	item, err := s.orgItemRepo.GetByUUID(ctx, req.ItemUUID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeItemShare(ctx, ownerID, item); err != nil {
		return nil, err
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, item.OrganizationID, domain.CapabilitySharingCreate); err != nil {
			return nil, err
		}
	}

	return s.createShareInternal(ctx, ownerID, item, req)
}

func (s *itemShareService) authorizeItemShare(
	ctx context.Context,
	userID uint,
	item *domain.OrganizationItem,
) error {
	if item == nil {
		return repository.ErrNotFound
	}

	orgUser, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, item.OrganizationID, userID)
	if err != nil || orgUser == nil {
		return repository.ErrForbidden
	}
	if orgUser.IsAdmin() || orgUser.AccessAll {
		return nil
	}

	if item.CollectionID == nil {
		if item.CreatedByUserID != userID {
			return repository.ErrForbidden
		}
		return nil
	}

	access, err := authz.ComputeCollectionAccess(
		ctx,
		orgUser,
		*item.CollectionID,
		s.collectionUsers,
		s.collectionTeams,
		s.teamMemberships,
	)
	if err != nil {
		return err
	}
	if !access.CanWrite && !access.CanAdmin {
		return repository.ErrForbidden
	}
	// Sharing hands the recipient the item's secrets, so it needs password
	// visibility too (edit_except_passwords cannot share).
	if access.HidePasswords {
		return repository.ErrForbidden
	}

	return nil
}

// externalSharingEnabled stays false until shares carry a per-item key
// instead of the organization key.
const externalSharingEnabled = false

func (s *itemShareService) isActiveMember(ctx context.Context, orgID, userID uint) bool {
	member, err := s.orgUserRepo.GetActiveByOrgAndUser(ctx, orgID, userID)
	return err == nil && member != nil
}

// shareStillAuthorized re-checks a share against current access: the owner
// must still be allowed to share the item and the recipient must still be an
// active member. A share never outlives the access it was created from.
func (s *itemShareService) shareStillAuthorized(ctx context.Context, share *domain.ItemShare, item *domain.OrganizationItem) bool {
	if err := s.authorizeItemShare(ctx, share.OwnerID, item); err != nil {
		return false
	}
	if share.SharedWithUserID == nil {
		return false
	}
	return s.isActiveMember(ctx, item.OrganizationID, *share.SharedWithUserID)
}

func (s *itemShareService) createShareInternal(
	ctx context.Context,
	ownerID uint,
	item *domain.OrganizationItem,
	req *CreateItemShareRequest,
) (*ItemShareWithItem, error) {
	sharedWithUserID := req.SharedWithUserID
	var sharedWithUser *domain.User
	if sharedWithUserID == nil && req.SharedWithEmail != "" {
		user, err := s.userRepo.GetByEmail(ctx, req.SharedWithEmail)
		if err != nil || user == nil {
			// An unregistered recipient cannot be an organization member.
			if !externalSharingEnabled {
				return nil, ErrSecureSharingUnavailable
			}
			if s.emailSender == nil || s.emailBuilder == nil {
				return nil, repository.ErrNotFound
			}
			owner, ownerErr := s.userRepo.GetByID(ctx, ownerID)
			if ownerErr != nil || owner == nil {
				return nil, repository.ErrNotFound
			}
			itemName := item.Metadata.Name
			if strings.TrimSpace(itemName) == "" {
				itemName = "Shared item"
			}
			message, buildErr := s.emailBuilder.BuildShareInviteEmail(
				req.SharedWithEmail,
				owner.Name,
				itemName,
			)
			if buildErr != nil {
				return nil, fmt.Errorf("failed to build share invite email: %w", buildErr)
			}
			if sendErr := s.emailSender.Send(ctx, message); sendErr != nil {
				return nil, fmt.Errorf("failed to send share invite email: %w", sendErr)
			}
			return nil, ErrShareInviteSent
		}
		sharedWithUserID = &user.ID
		sharedWithUser = user
	}
	if sharedWithUserID != nil {
		if sharedWithUser == nil {
			user, err := s.userRepo.GetByID(ctx, *sharedWithUserID)
			if err != nil || user == nil {
				return nil, repository.ErrNotFound
			}
			sharedWithUser = user
		}
		if *sharedWithUserID == ownerID {
			return nil, repository.ErrInvalidInput
		}
		if !externalSharingEnabled && !s.isActiveMember(ctx, item.OrganizationID, *sharedWithUserID) {
			return nil, ErrSecureSharingUnavailable
		}
	}

	if strings.TrimSpace(req.EncryptedKey) == "" {
		return nil, repository.ErrInvalidInput
	}

	canView := true
	canEdit := false
	canShare := false
	if req.CanView != nil {
		canView = *req.CanView
	}
	if req.CanEdit != nil {
		canEdit = *req.CanEdit
	}
	if req.CanShare != nil {
		canShare = *req.CanShare
	}
	if !canView && (canEdit || canShare) {
		canView = true
	}
	if !canView {
		return nil, repository.ErrInvalidInput
	}

	share := &domain.ItemShare{
		UUID:             uuid.New(),
		ItemUUID:         item.UUID,
		OrganizationID:   item.OrganizationID,
		OwnerID:          ownerID,
		SharedWithUserID: sharedWithUserID,
		CanView:          canView,
		CanEdit:          canEdit,
		CanShare:         canShare,
		EncryptedKey:     req.EncryptedKey,
		ExpiresAt:        req.ExpiresAt,
	}

	if err := s.shareRepo.Create(ctx, share); err != nil {
		s.logger.Error("failed to create item share", "error", err)
		return nil, fmt.Errorf("failed to create item share: %w", err)
	}

	if sharedWithUser != nil && s.emailSender != nil && s.emailBuilder != nil {
		go s.sendShareNotificationEmail(ownerID, sharedWithUser, item)
	}

	return &ItemShareWithItem{Share: share, Item: item}, nil
}

func (s *itemShareService) sendShareNotificationEmail(ownerID uint, recipient *domain.User, item *domain.OrganizationItem) {
	if recipient == nil || recipient.Email == "" {
		return
	}

	owner, err := s.userRepo.GetByID(context.Background(), ownerID)
	if err != nil || owner == nil {
		s.logger.Warn("failed to load share owner for notification email", "error", err)
		return
	}

	itemName := item.Metadata.Name
	if strings.TrimSpace(itemName) == "" {
		itemName = "Shared item"
	}

	message, buildErr := s.emailBuilder.BuildShareNotificationEmail(
		recipient.Email,
		owner.Name,
		itemName,
	)
	if buildErr != nil {
		s.logger.Error("failed to build share notification email", "error", buildErr)
		return
	}

	if sendErr := s.emailSender.Send(context.Background(), message); sendErr != nil {
		s.logger.Error("failed to send share notification email", "error", sendErr)
	}
}

func (s *itemShareService) ListOwned(ctx context.Context, ownerID uint) ([]*ItemShareWithItem, error) {
	shares, err := s.shareRepo.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	results := make([]*ItemShareWithItem, 0, len(shares))
	for _, share := range shares {
		item, err := s.orgItemRepo.GetByUUID(ctx, share.ItemUUID.String())
		if err != nil {
			if err == repository.ErrNotFound {
				continue
			}
			return nil, err
		}
		results = append(results, &ItemShareWithItem{Share: share, Item: item})
	}

	return results, nil
}

func (s *itemShareService) ListReceived(ctx context.Context, userID uint) ([]*ItemShareWithItem, error) {
	shares, err := s.shareRepo.ListSharedWithUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	results := make([]*ItemShareWithItem, 0, len(shares))
	for _, share := range shares {
		if share.IsExpired() {
			continue
		}
		item, err := s.orgItemRepo.GetByUUID(ctx, share.ItemUUID.String())
		if err != nil {
			if err == repository.ErrNotFound {
				continue
			}
			return nil, err
		}
		if !s.shareStillAuthorized(ctx, share, item) {
			continue
		}
		results = append(results, &ItemShareWithItem{Share: share, Item: item})
	}

	return results, nil
}

func (s *itemShareService) GetByUUID(ctx context.Context, userID uint, shareUUID string) (*ItemShareWithItem, error) {
	share, err := s.shareRepo.GetByUUID(ctx, shareUUID)
	if err != nil {
		return nil, err
	}
	if share.IsExpired() {
		return nil, repository.ErrNotFound
	}
	if share.OwnerID != userID {
		if share.SharedWithUserID == nil || *share.SharedWithUserID != userID {
			return nil, repository.ErrForbidden
		}
	}

	item, err := s.orgItemRepo.GetByUUID(ctx, share.ItemUUID.String())
	if err != nil {
		return nil, err
	}
	if share.OwnerID != userID && !s.shareStillAuthorized(ctx, share, item) {
		return nil, repository.ErrForbidden
	}

	return &ItemShareWithItem{Share: share, Item: item}, nil
}

func (s *itemShareService) Revoke(ctx context.Context, ownerID uint, shareID uint) error {
	share, err := s.shareRepo.GetByID(ctx, shareID)
	if err != nil {
		return err
	}
	if share.OwnerID != ownerID {
		return repository.ErrForbidden
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, share.OrganizationID, domain.CapabilityAccessRevoke); err != nil {
			return err
		}
	}

	return s.shareRepo.Delete(ctx, shareID)
}

func (s *itemShareService) UpdateSharedItem(
	ctx context.Context,
	userID uint,
	shareUUID string,
	req *UpdateSharedItemRequest,
) (*domain.OrganizationItem, error) {
	if strings.TrimSpace(shareUUID) == "" {
		return nil, repository.ErrInvalidInput
	}
	if strings.TrimSpace(req.Data) == "" {
		return nil, repository.ErrInvalidInput
	}
	if req.Metadata.Name == "" {
		return nil, repository.ErrInvalidInput
	}

	share, err := s.shareRepo.GetByUUID(ctx, shareUUID)
	if err != nil {
		return nil, err
	}
	if share.IsExpired() {
		return nil, repository.ErrNotFound
	}
	if share.SharedWithUserID == nil || *share.SharedWithUserID != userID {
		return nil, repository.ErrForbidden
	}
	if !share.CanEdit {
		return nil, repository.ErrForbidden
	}

	item, err := s.orgItemRepo.GetByUUID(ctx, share.ItemUUID.String())
	if err != nil {
		return nil, err
	}
	if !s.shareStillAuthorized(ctx, share, item) {
		return nil, repository.ErrForbidden
	}
	// Editing through a share writes the organization item, so the recipient
	// needs the same collection write access as any other edit.
	if err := s.authorizeItemShare(ctx, userID, item); err != nil {
		return nil, err
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, item.OrganizationID, domain.CapabilityItemUpdate); err != nil {
			return nil, err
		}
	}

	item.Data = req.Data
	item.Metadata = req.Metadata

	if err := s.orgItemRepo.Update(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *itemShareService) ReShare(
	ctx context.Context,
	userID uint,
	shareUUID string,
	req *CreateItemShareRequest,
) (*ItemShareWithItem, error) {
	if strings.TrimSpace(shareUUID) == "" {
		return nil, repository.ErrInvalidInput
	}

	share, err := s.shareRepo.GetByUUID(ctx, shareUUID)
	if err != nil {
		return nil, err
	}
	if share.IsExpired() {
		return nil, repository.ErrNotFound
	}
	if share.SharedWithUserID == nil || *share.SharedWithUserID != userID {
		return nil, repository.ErrForbidden
	}
	if !share.CanShare {
		return nil, repository.ErrForbidden
	}

	item, err := s.orgItemRepo.GetByUUID(ctx, share.ItemUUID.String())
	if err != nil {
		return nil, err
	}
	if !s.shareStillAuthorized(ctx, share, item) {
		return nil, repository.ErrForbidden
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, item.OrganizationID, domain.CapabilitySharingCreate); err != nil {
			return nil, err
		}
	}
	// A re-share cannot grant more than the re-sharer received.
	if req.CanEdit != nil && *req.CanEdit && !share.CanEdit {
		return nil, repository.ErrForbidden
	}

	return s.createShareInternal(ctx, share.OwnerID, item, req)
}

func (s *itemShareService) UpdatePermissions(
	ctx context.Context,
	ownerID uint,
	shareUUID string,
	req *UpdateItemSharePermissionsRequest,
) (*ItemShareWithItem, error) {
	if strings.TrimSpace(shareUUID) == "" {
		return nil, repository.ErrInvalidInput
	}

	share, err := s.shareRepo.GetByUUID(ctx, shareUUID)
	if err != nil {
		return nil, err
	}
	if share.OwnerID != ownerID {
		return nil, repository.ErrForbidden
	}
	if s.entitlements != nil {
		if err := s.entitlements.Authorize(ctx, share.OrganizationID, domain.CapabilityItemUpdate); err != nil {
			return nil, err
		}
	}

	if req != nil {
		if req.CanEdit != nil {
			share.CanEdit = *req.CanEdit
		}
		if req.CanShare != nil {
			share.CanShare = *req.CanShare
		}
		if req.ClearExpiresAt {
			share.ExpiresAt = nil
		} else if req.ExpiresAt != nil {
			if req.ExpiresAt.Before(time.Now()) {
				return nil, repository.ErrInvalidInput
			}
			share.ExpiresAt = req.ExpiresAt
		}
	}

	// Enforce invariant: shares must always be viewable.
	share.CanView = true

	if err := s.shareRepo.Update(ctx, share); err != nil {
		return nil, err
	}

	item, err := s.orgItemRepo.GetByUUID(ctx, share.ItemUUID.String())
	if err != nil {
		return nil, err
	}

	return &ItemShareWithItem{Share: share, Item: item}, nil
}
