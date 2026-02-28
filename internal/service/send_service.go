package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

// SendService defines the business logic for Secure Send
type SendService interface {
	Create(ctx context.Context, creatorID uint, req *domain.CreateSendRequest) (*domain.Send, error)
	GetByAccessID(ctx context.Context, accessID string, requesterUserID uint) (*domain.SendAccessDTO, error)
	VerifySendPassword(ctx context.Context, accessID string, password string) (*domain.SendAccessDTO, error)
	GetByUUID(ctx context.Context, creatorID uint, sendUUID string) (*domain.Send, error)
	List(ctx context.Context, creatorID uint) ([]*domain.Send, error)
	Update(ctx context.Context, creatorID uint, sendUUID string, req *domain.UpdateSendRequest) (*domain.Send, error)
	Delete(ctx context.Context, creatorID uint, sendUUID string) error
	CleanupExpired(ctx context.Context) (int64, error)
}

type sendService struct {
	sendRepo    repository.SendRepository
	userRepo    repository.UserRepository
	orgUserRepo repository.OrganizationUserRepository
	policyRepo  repository.OrganizationPolicyRepository
	logger      Logger
}

func NewSendService(
	sendRepo repository.SendRepository,
	userRepo repository.UserRepository,
	orgUserRepo repository.OrganizationUserRepository,
	policyRepo repository.OrganizationPolicyRepository,
	logger Logger,
) SendService {
	return &sendService{
		sendRepo:    sendRepo,
		userRepo:    userRepo,
		orgUserRepo: orgUserRepo,
		policyRepo:  policyRepo,
		logger:      logger,
	}
}

func generateAccessID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:12], nil
}

func (s *sendService) Create(ctx context.Context, creatorID uint, req *domain.CreateSendRequest) (*domain.Send, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, repository.ErrInvalidInput
	}
	if strings.TrimSpace(req.Data) == "" {
		return nil, repository.ErrInvalidInput
	}

	// Check RemoveSend policy: if enabled, non-admin members cannot create sends
	if req.OrganizationID > 0 {
		if err := s.checkRemoveSendPolicy(ctx, req.OrganizationID, creatorID); err != nil {
			return nil, err
		}
	}

	accessID, err := generateAccessID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate access ID: %w", err)
	}

	// Default deletion date: 7 days
	deletionDate := time.Now().Add(7 * 24 * time.Hour)
	if req.DeletionDate != nil {
		deletionDate = *req.DeletionDate
	}

	send := &domain.Send{
		UUID:           uuid.NewV4(),
		AccessID:       accessID,
		CreatorID:      creatorID,
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Type:           req.Type,
		Data:           req.Data,
		Notes:          req.Notes,
		MaxAccessCount: req.MaxAccessCount,
		ExpirationDate: req.ExpirationDate,
		DeletionDate:   deletionDate,
		HideEmail:      req.HideEmail,
	}

	if req.Password != nil && *req.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		hashedStr := string(hashed)
		send.Password = &hashedStr
	}

	if send.Type == "" {
		send.Type = domain.SendTypeText
	}

	if err := s.sendRepo.Create(ctx, send); err != nil {
		return nil, fmt.Errorf("failed to create send: %w", err)
	}

	return send, nil
}

func (s *sendService) GetByAccessID(ctx context.Context, accessID string, requesterUserID uint) (*domain.SendAccessDTO, error) {
	send, err := s.sendRepo.GetByAccessID(ctx, accessID)
	if err != nil {
		return nil, err
	}

	if send.Disabled {
		return nil, repository.ErrNotFound
	}
	if send.IsExpired() {
		return nil, repository.ErrNotFound
	}
	if send.IsAccessLimitReached() {
		return nil, repository.ErrNotFound
	}

	dto := &domain.SendAccessDTO{
		AccessID:       send.AccessID,
		Name:           send.Name,
		Type:           send.Type,
		HasPassword:    send.HasPassword(),
		ExpirationDate: send.ExpirationDate,
	}

	if !send.HideEmail && send.Creator != nil {
		dto.CreatorEmail = send.Creator.Email
	}

	// Withhold encrypted payload until password is verified
	if !send.HasPassword() {
		dto.Data = send.Data
		dto.Notes = send.Notes
		if err := s.sendRepo.IncrementAccessCount(ctx, send.ID); err != nil {
			s.logger.Error("failed to increment send access count", "send_id", send.ID, "error", err)
		}
	}

	return dto, nil
}

func (s *sendService) VerifySendPassword(ctx context.Context, accessID string, password string) (*domain.SendAccessDTO, error) {
	send, err := s.sendRepo.GetByAccessID(ctx, accessID)
	if err != nil {
		return nil, err
	}

	if send.Disabled || send.IsExpired() || send.IsAccessLimitReached() {
		return nil, repository.ErrNotFound
	}

	if send.HasPassword() {
		if err := bcrypt.CompareHashAndPassword([]byte(*send.Password), []byte(password)); err != nil {
			return nil, repository.ErrForbidden
		}
	}

	if err := s.sendRepo.IncrementAccessCount(ctx, send.ID); err != nil {
		s.logger.Error("failed to increment send access count", "send_id", send.ID, "error", err)
	}

	dto := &domain.SendAccessDTO{
		AccessID:       send.AccessID,
		Name:           send.Name,
		Type:           send.Type,
		Data:           send.Data,
		Notes:          send.Notes,
		HasPassword:    send.HasPassword(),
		ExpirationDate: send.ExpirationDate,
	}

	if !send.HideEmail && send.Creator != nil {
		dto.CreatorEmail = send.Creator.Email
	}

	return dto, nil
}

func (s *sendService) GetByUUID(ctx context.Context, creatorID uint, sendUUID string) (*domain.Send, error) {
	send, err := s.sendRepo.GetByUUID(ctx, sendUUID)
	if err != nil {
		return nil, err
	}

	if send.CreatorID != creatorID {
		return nil, repository.ErrForbidden
	}

	return send, nil
}

func (s *sendService) List(ctx context.Context, creatorID uint) ([]*domain.Send, error) {
	return s.sendRepo.ListByCreator(ctx, creatorID)
}

func (s *sendService) Update(ctx context.Context, creatorID uint, sendUUID string, req *domain.UpdateSendRequest) (*domain.Send, error) {
	send, err := s.sendRepo.GetByUUID(ctx, sendUUID)
	if err != nil {
		return nil, err
	}

	if send.CreatorID != creatorID {
		return nil, repository.ErrForbidden
	}

	if req.Name != nil {
		send.Name = *req.Name
	}
	if req.Data != nil {
		send.Data = *req.Data
	}
	if req.Notes != nil {
		send.Notes = req.Notes
	}
	if req.MaxAccessCount != nil {
		send.MaxAccessCount = req.MaxAccessCount
	}
	if req.ExpirationDate != nil {
		send.ExpirationDate = req.ExpirationDate
	}
	if req.DeletionDate != nil {
		send.DeletionDate = *req.DeletionDate
	}
	if req.Disabled != nil {
		send.Disabled = *req.Disabled
	}
	if req.HideEmail != nil {
		send.HideEmail = *req.HideEmail
	}
	if req.Password != nil {
		if *req.Password == "" {
			send.Password = nil
		} else {
			hashed, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
			if err != nil {
				return nil, fmt.Errorf("failed to hash password: %w", err)
			}
			hashedStr := string(hashed)
			send.Password = &hashedStr
		}
	}

	if err := s.sendRepo.Update(ctx, send); err != nil {
		return nil, fmt.Errorf("failed to update send: %w", err)
	}

	return send, nil
}

func (s *sendService) Delete(ctx context.Context, creatorID uint, sendUUID string) error {
	send, err := s.sendRepo.GetByUUID(ctx, sendUUID)
	if err != nil {
		return err
	}

	if send.CreatorID != creatorID {
		return repository.ErrForbidden
	}

	return s.sendRepo.SoftDelete(ctx, send.ID)
}

func (s *sendService) CleanupExpired(ctx context.Context) (int64, error) {
	return s.sendRepo.DeleteExpired(ctx)
}

// checkRemoveSendPolicy checks if the RemoveSend policy blocks this user
func (s *sendService) checkRemoveSendPolicy(ctx context.Context, orgID, userID uint) error {
	policy, err := s.policyRepo.GetByOrgAndType(ctx, orgID, domain.PolicyRemoveSend)
	if err != nil {
		return nil // policy not found = not enforced
	}
	if !policy.Enabled {
		return nil
	}

	// Check if user is admin/owner in org (admins are exempt)
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return repository.ErrForbidden
	}

	if orgUser.Role == domain.OrgRoleOwner || orgUser.Role == domain.OrgRoleAdmin {
		return nil
	}

	return repository.ErrForbidden
}
