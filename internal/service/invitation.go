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
	GetPendingInvitations(ctx context.Context, email string) ([]*domain.Invitation, error)
	GetSentInvitations(ctx context.Context, userID uint) ([]*domain.Invitation, error)
	AcceptInvitation(ctx context.Context, invitationID uint, userID uint) error
	DeclineInvitation(ctx context.Context, invitationID uint, userID uint) error
}

type invitationService struct {
	repo         repository.InvitationRepository
	userRepo     repository.UserRepository
	orgRepo      repository.OrganizationRepository
	emailSender  email.Sender
	emailBuilder *email.EmailBuilder
	logger       Logger
}

// NewInvitationService creates a new invitation service
func NewInvitationService(
	repo repository.InvitationRepository,
	userRepo repository.UserRepository,
	orgRepo repository.OrganizationRepository,
	emailSender email.Sender,
	emailBuilder *email.EmailBuilder,
	logger Logger,
) InvitationService {
	return &invitationService{
		repo:         repo,
		userRepo:     userRepo,
		orgRepo:      orgRepo,
		emailSender:  emailSender,
		emailBuilder: emailBuilder,
		logger:       logger,
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
		Email:           req.Email,
		Code:            code,
		RoleID:          req.RoleID,
		CreatedBy:       createdBy,
		ExpiresAt:       time.Now().Add(7 * 24 * time.Hour), // 7 days
		OrganizationID:  req.OrganizationID,
		OrgRole:         req.OrgRole,
		EncryptedOrgKey: req.EncryptedOrgKey,
		AccessAll:       req.AccessAll != nil && *req.AccessAll,
	}

	if err := s.repo.Create(ctx, invitation); err != nil {
		s.logger.Error("failed to create invitation", "email", req.Email, "error", err)
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Send invitation email (async)
	go func() {
		emailCtx := context.Background()
		roleName := getRoleName(req.RoleID)
		
		// Get organization name if this is an org invitation
		orgName := ""
		if req.OrganizationID != nil {
			if org, err := s.orgRepo.GetByID(emailCtx, *req.OrganizationID); err == nil {
				orgName = org.Name
			}
		}
		
		// Build invitation email message
		var message *email.EmailMessage
		var err error
		if orgName != "" {
			message, err = s.emailBuilder.BuildInvitationWithOrgEmail(req.Email, inviterName, code, roleName, orgName)
		} else {
			message, err = s.emailBuilder.BuildInvitationEmail(req.Email, inviterName, code, roleName)
		}
		
		if err != nil {
			s.logger.Error("failed to build invitation email", "email", req.Email, "error", err)
			return
		}
		
		// Send email
		if err := s.emailSender.Send(emailCtx, message); err != nil {
			s.logger.Error("failed to send invitation email", "email", req.Email, "error", err)
		}
	}()

	s.logger.Info("invitation created",
		"email", req.Email,
		"created_by", createdBy,
		"role_id", req.RoleID)

	return invitation, nil
}

func (s *invitationService) GetPendingInvitations(ctx context.Context, email string) ([]*domain.Invitation, error) {
	invitations, err := s.repo.GetAllByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending invitations: %w", err)
	}
	return invitations, nil
}

func (s *invitationService) GetSentInvitations(ctx context.Context, userID uint) ([]*domain.Invitation, error) {
	invitations, err := s.repo.GetByCreator(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sent invitations: %w", err)
	}
	return invitations, nil
}

func (s *invitationService) AcceptInvitation(ctx context.Context, invitationID uint, userID uint) error {
	// Get invitation
	invitation, err := s.repo.GetByID(ctx, invitationID)
	if err != nil {
		return fmt.Errorf("invitation not found: %w", err)
	}

	// Verify invitation belongs to this user's email
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if invitation.Email != user.Email {
		return repository.ErrForbidden
	}

	// Check if already used
	if invitation.IsUsed() {
		return fmt.Errorf("invitation already used")
	}

	// Check if expired
	if invitation.IsExpired() {
		return fmt.Errorf("invitation expired")
	}

	// If this is an organization invitation, add user to org
	if invitation.OrganizationID != nil && invitation.OrgRole != nil && invitation.EncryptedOrgKey != nil {
		// This will be handled by organization service
		// We'll return the invitation data and let the caller handle org join
		s.logger.Info("invitation accepted - requires org join",
			"invitation_id", invitationID,
			"user_id", userID,
			"org_id", *invitation.OrganizationID)
	}

	// Mark invitation as used
	now := time.Now()
	invitation.UsedAt = &now
	if err := s.repo.Update(ctx, invitation); err != nil {
		return fmt.Errorf("failed to mark invitation as used: %w", err)
	}

	s.logger.Info("invitation accepted",
		"invitation_id", invitationID,
		"user_id", userID)

	return nil
}

func (s *invitationService) DeclineInvitation(ctx context.Context, invitationID uint, userID uint) error {
	// Get invitation
	invitation, err := s.repo.GetByID(ctx, invitationID)
	if err != nil {
		return fmt.Errorf("invitation not found: %w", err)
	}

	// Verify invitation belongs to this user's email
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if invitation.Email != user.Email {
		return repository.ErrForbidden
	}

	// Delete invitation (declined)
	if err := s.repo.Delete(ctx, invitationID); err != nil {
		return fmt.Errorf("failed to decline invitation: %w", err)
	}

	s.logger.Info("invitation declined",
		"invitation_id", invitationID,
		"user_id", userID)

	return nil
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
