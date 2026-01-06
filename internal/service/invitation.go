package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
)

type InvitationService interface {
	CreateInvitation(ctx context.Context, req *domain.CreateInvitationRequest, createdBy uint, inviterName string) (*domain.Invitation, error)
}

type invitationService struct {
	repo        repository.InvitationRepository
	userRepo    repository.UserRepository
	emailSender email.Sender
	logger      Logger
}

// NewInvitationService creates a new invitation service
func NewInvitationService(
	repo repository.InvitationRepository,
	userRepo repository.UserRepository,
	emailSender email.Sender,
	logger Logger,
) InvitationService {
	return &invitationService{
		repo:        repo,
		userRepo:    userRepo,
		emailSender: emailSender,
		logger:      logger,
	}
}

func (s *invitationService) CreateInvitation(ctx context.Context, req *domain.CreateInvitationRequest, createdBy uint, inviterName string) (*domain.Invitation, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, repository.ErrAlreadyExists
	}

	// Check if there's already an active invitation
	existingInvite, err := s.repo.GetByEmail(ctx, req.Email)
	if err == nil && existingInvite != nil {
		return nil, fmt.Errorf("active invitation already exists for this email")
	}

	// Generate invitation code (32 chars, URL-safe)
	code, err := generateInvitationCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate invitation code: %w", err)
	}

	// Create invitation
	invitation := &domain.Invitation{
		Email:     req.Email,
		Code:      code,
		RoleID:    req.RoleID,
		CreatedBy: createdBy,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days
	}

	if err := s.repo.Create(ctx, invitation); err != nil {
		s.logger.Error("failed to create invitation", "email", req.Email, "error", err)
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Send invitation email (async)
	go func() {
		emailCtx := context.Background()
		roleName := getRoleName(req.RoleID)
		if err := s.emailSender.SendInvitationEmail(emailCtx, req.Email, inviterName, code, roleName); err != nil {
			s.logger.Error("failed to send invitation email", "email", req.Email, "error", err)
		}
	}()

	s.logger.Info("invitation created",
		"email", req.Email,
		"created_by", createdBy,
		"role_id", req.RoleID)

	return invitation, nil
}

// generateInvitationCode generates a secure random invitation code
func generateInvitationCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const length = 32
	code := make([]byte, length)

	for i := range code {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		code[i] = charset[int(b[0])%len(charset)]
	}

	return string(code), nil
}

func getRoleName(roleID uint) string {
	if roleID == 1 {
		return "Admin"
	}
	return "Member"
}
