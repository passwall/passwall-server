package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
)

// ErrShareInviteSent indicates a signup email was sent for non-registered recipient.
var ErrShareInviteSent = errors.New("share invite email sent")

// CreateItemShareRequest represents a request to share a personal item.
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

// ItemShareWithItem bundles share + item data for responses.
type ItemShareWithItem struct {
	Share *domain.ItemShare
	Item  *domain.Item
}

type itemShareService struct {
	shareRepo    repository.ItemShareRepository
	itemRepo     repository.ItemRepository
	userRepo     repository.UserRepository
	emailSender  email.Sender
	emailBuilder *email.EmailBuilder
	logger       Logger
}

func NewItemShareService(
	shareRepo repository.ItemShareRepository,
	itemRepo repository.ItemRepository,
	userRepo repository.UserRepository,
	emailSender email.Sender,
	emailBuilder *email.EmailBuilder,
	logger Logger,
) ItemShareService {
	return &itemShareService{
		shareRepo:    shareRepo,
		itemRepo:     itemRepo,
		userRepo:     userRepo,
		emailSender:  emailSender,
		emailBuilder: emailBuilder,
		logger:       logger,
	}
}

func (s *itemShareService) Create(ctx context.Context, ownerID uint, ownerSchema string, req *CreateItemShareRequest) (*ItemShareWithItem, error) {
	if strings.TrimSpace(req.ItemUUID) == "" {
		return nil, repository.ErrInvalidInput
	}
	if req.SharedWithUserID == nil && strings.TrimSpace(req.SharedWithEmail) == "" {
		return nil, repository.ErrInvalidInput
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		return nil, repository.ErrInvalidInput
	}

	item, err := s.itemRepo.FindByUUID(ctx, ownerSchema, req.ItemUUID)
	if err != nil {
		return nil, err
	}

	return s.createShareInternal(ctx, ownerID, ownerSchema, item, req)
}

func (s *itemShareService) createShareInternal(
	ctx context.Context,
	ownerID uint,
	ownerSchema string,
	item *domain.Item,
	req *CreateItemShareRequest,
) (*ItemShareWithItem, error) {
	sharedWithUserID := req.SharedWithUserID
	var sharedWithUser *domain.User
	if sharedWithUserID == nil && req.SharedWithEmail != "" {
		user, err := s.userRepo.GetByEmail(ctx, req.SharedWithEmail)
		if err != nil || user == nil {
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
		UUID:             uuid.NewV4(),
		ItemUUID:         item.UUID,
		UserSchema:       ownerSchema,
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

func (s *itemShareService) sendShareNotificationEmail(ownerID uint, recipient *domain.User, item *domain.Item) {
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
		item, err := s.itemRepo.FindByUUID(ctx, share.UserSchema, share.ItemUUID.String())
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
		item, err := s.itemRepo.FindByUUID(ctx, share.UserSchema, share.ItemUUID.String())
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

	item, err := s.itemRepo.FindByUUID(ctx, share.UserSchema, share.ItemUUID.String())
	if err != nil {
		return nil, err
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

	return s.shareRepo.Delete(ctx, shareID)
}

func (s *itemShareService) UpdateSharedItem(
	ctx context.Context,
	userID uint,
	shareUUID string,
	req *UpdateSharedItemRequest,
) (*domain.Item, error) {
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

	item, err := s.itemRepo.FindByUUID(ctx, share.UserSchema, share.ItemUUID.String())
	if err != nil {
		return nil, err
	}

	item.Data = req.Data
	item.Metadata = req.Metadata

	if err := s.itemRepo.Update(ctx, share.UserSchema, item); err != nil {
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

	item, err := s.itemRepo.FindByUUID(ctx, share.UserSchema, share.ItemUUID.String())
	if err != nil {
		return nil, err
	}

	return s.createShareInternal(ctx, share.OwnerID, share.UserSchema, item, req)
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

	item, err := s.itemRepo.FindByUUID(ctx, share.UserSchema, share.ItemUUID.String())
	if err != nil {
		return nil, err
	}

	return &ItemShareWithItem{Share: share, Item: item}, nil
}
