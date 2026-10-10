package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

// IssueSSOSession signs in a user after IdP authentication. SSO proves who
// the user is to the organization, but it does not replace the user's own
// Passwall 2FA: users with 2FA enabled receive a two-factor token and finish
// through the regular /auth/2fa/verify step.
func (s *authService) IssueSSOSession(ctx context.Context, userID uint, app string, deviceID string) (*domain.AuthResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if !user.IsVerified {
		return nil, errors.New("email not verified")
	}

	if user.TwoFactorEnabled && user.TwoFactorSecret != nil {
		sessionUUID := uuid.New()
		deviceUUID := parseUUIDOrNil(deviceID)
		if deviceID != "" && deviceUUID != uuid.Nil {
			sessionUUID = deviceUUID
		}
		tfToken, err := s.createTwoFactorToken(user, sessionUUID, deviceUUID, app, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create 2FA token: %w", err)
		}
		return &domain.AuthResponse{TwoFactorRequired: true, TwoFactorToken: tfToken}, nil
	}

	resp, err := s.IssueTokenForUser(ctx, userID, app, deviceID)
	if err != nil {
		return nil, err
	}
	resp.PolicyRequirements = s.collectPolicyRequirements(ctx, user)
	return resp, nil
}
