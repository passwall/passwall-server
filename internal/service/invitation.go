package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
)

// Referral ("invite a friend to Passwall") invitations. They create no
// membership; organization invitations are handled by OrganizationService.

const (
	referralExpiry       = 7 * 24 * time.Hour
	maxReferralsPerDay   = 20
	referralEmailTimeout = 15 * time.Second
)

const (
	ReferralCodeAlreadyRegistered = "ALREADY_REGISTERED"
	ReferralCodeExists            = "REFERRAL_EXISTS"
	ReferralCodeLimit             = "REFERRAL_LIMIT_REACHED"
	ReferralCodeInvalidEmail      = "INVALID_EMAIL"
)

type InvitationService interface {
	CreateReferral(ctx context.Context, email string, createdBy uint, inviterName string) (*domain.Invitation, error)
	ListSentReferrals(ctx context.Context, userID uint) ([]*domain.Invitation, error)
}

type invitationService struct {
	repo         repository.InvitationRepository
	userRepo     repository.UserRepository
	emailSender  email.Sender
	emailBuilder *email.EmailBuilder
	logger       Logger
}

// NewInvitationService creates the referral invitation service.
func NewInvitationService(
	repo repository.InvitationRepository,
	userRepo repository.UserRepository,
	emailSender email.Sender,
	emailBuilder *email.EmailBuilder,
	logger Logger,
) InvitationService {
	return &invitationService{
		repo:         repo,
		userRepo:     userRepo,
		emailSender:  emailSender,
		emailBuilder: emailBuilder,
		logger:       logger,
	}
}

func (s *invitationService) CreateReferral(ctx context.Context, emailAddr string, createdBy uint, inviterName string) (*domain.Invitation, error) {
	emailAddr = domain.NormalizeInvitationEmail(emailAddr)
	if emailAddr == "" || !strings.Contains(emailAddr, "@") {
		return nil, invitationErr(400, ReferralCodeInvalidEmail, "a valid email address is required")
	}
	if existing, err := s.userRepo.GetByEmail(ctx, emailAddr); err == nil && existing != nil {
		return nil, invitationErr(409, ReferralCodeAlreadyRegistered, "this person already uses Passwall")
	}
	if _, err := s.repo.GetActiveByEmail(ctx, emailAddr); err == nil {
		return nil, invitationErr(409, ReferralCodeExists, "an invitation was already sent to this email")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check referral: %w", err)
	}
	sent, err := s.repo.CountByCreatorSince(ctx, createdBy, time.Now().Add(-24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("count referrals: %w", err)
	}
	if sent >= maxReferralsPerDay {
		return nil, invitationErr(429, ReferralCodeLimit, "daily invitation limit reached; try again tomorrow")
	}

	code, err := generateInvitationCode()
	if err != nil {
		return nil, fmt.Errorf("generate invitation code: %w", err)
	}
	invitation := &domain.Invitation{
		Email:     emailAddr,
		Code:      code,
		RoleID:    2,
		CreatedBy: createdBy,
		ExpiresAt: time.Now().Add(referralExpiry),
	}
	if err := s.repo.Create(ctx, invitation); err != nil {
		return nil, fmt.Errorf("create referral: %w", err)
	}

	if s.emailSender != nil && s.emailBuilder != nil {
		message, err := s.emailBuilder.BuildInvitationEmail(emailAddr, inviterName, code, "Member")
		if err == nil {
			sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), referralEmailTimeout)
			defer cancel()
			err = s.emailSender.Send(sendCtx, message)
		}
		if err != nil {
			s.logger.Error("failed to send referral email", "invitation_id", invitation.ID, "error", err)
		}
	}
	return invitation, nil
}

func (s *invitationService) ListSentReferrals(ctx context.Context, userID uint) ([]*domain.Invitation, error) {
	return s.repo.ListByCreator(ctx, userID)
}

// generateInvitationCode returns an unbiased 32-character alphanumeric code.
func generateInvitationCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	code := make([]byte, 32)
	limit := big.NewInt(int64(len(charset)))
	for i := range code {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		code[i] = charset[n.Int64()]
	}
	return string(code), nil
}
