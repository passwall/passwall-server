package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

type stepUpUserRepoStub struct {
	repository.UserRepository
	user *domain.User
}

func (r stepUpUserRepoStub) GetByID(context.Context, uint) (*domain.User, error) { return r.user, nil }

func newStepUpFixture(t *testing.T, twoFactor bool) (*authService, *domain.User, string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("auth-hash"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Passwall", AccountName: "admin@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	secret := key.Secret()
	user := &domain.User{ID: 1, RoleID: constants.RoleIDAdmin, IsSystemUser: true, MasterPasswordHash: string(hash)}
	if twoFactor {
		user.TwoFactorEnabled = true
		user.TwoFactorSecret = &secret
	}
	svc := &authService{config: &AuthConfig{JWTSecret: "test-secret-0123456789"}, userRepo: stepUpUserRepoStub{user: user}}
	return svc, user, secret
}

func TestIssueAdminStepUp(t *testing.T) {
	session := uuid.New()

	t.Run("requires two-factor", func(t *testing.T) {
		svc, user, _ := newStepUpFixture(t, false)
		_, _, err := svc.IssueAdminStepUp(context.Background(), user.ID, session, "auth-hash", "000000")
		if !errors.Is(err, ErrTwoFactorRequired) {
			t.Fatalf("err = %v, want ErrTwoFactorRequired", err)
		}
	})

	svc, user, secret := newStepUpFixture(t, true)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct{ hash, code string }{
		"wrong master password": {"wrong", code},
		"wrong code":            {"auth-hash", "000000"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := svc.IssueAdminStepUp(context.Background(), user.ID, session, tc.hash, tc.code); !errors.Is(err, ErrStepUpFailed) {
				t.Fatalf("err = %v, want ErrStepUpFailed", err)
			}
		})
	}

	t.Run("issues a token bound to user and session", func(t *testing.T) {
		token, expiresAt, err := svc.IssueAdminStepUp(context.Background(), user.ID, session, "auth-hash", code)
		if err != nil {
			t.Fatalf("IssueAdminStepUp() error = %v", err)
		}
		if time.Until(expiresAt) > adminStepUpDuration || time.Until(expiresAt) < adminStepUpDuration-time.Minute {
			t.Fatalf("expiresAt = %v", expiresAt)
		}
		if err := svc.VerifyAdminStepUp(token, user.ID, session); err != nil {
			t.Fatalf("VerifyAdminStepUp() error = %v", err)
		}
		for name, check := range map[string]func() error{
			"other user":    func() error { return svc.VerifyAdminStepUp(token, user.ID+1, session) },
			"other session": func() error { return svc.VerifyAdminStepUp(token, user.ID, uuid.New()) },
			"empty token":   func() error { return svc.VerifyAdminStepUp("", user.ID, session) },
			"garbage":       func() error { return svc.VerifyAdminStepUp("not-a-jwt", user.ID, session) },
		} {
			if err := check(); !errors.Is(err, ErrStepUpInvalid) {
				t.Fatalf("%s: err = %v, want ErrStepUpInvalid", name, err)
			}
		}
	})

	t.Run("rejects expired and foreign-subject tokens", func(t *testing.T) {
		sign := func(subject string, exp time.Time) string {
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, adminStepUpClaims{
				UserID: user.ID, SessionUUID: session,
				RegisteredClaims: jwt.RegisteredClaims{Subject: subject, ExpiresAt: jwt.NewNumericDate(exp)},
			}).SignedString([]byte(svc.config.JWTSecret))
			if err != nil {
				t.Fatal(err)
			}
			return token
		}
		if err := svc.VerifyAdminStepUp(sign(adminStepUpSubject, time.Now().Add(-time.Second)), user.ID, session); !errors.Is(err, ErrStepUpInvalid) {
			t.Fatalf("expired: err = %v", err)
		}
		// A 2FA sign-in token signed with the same secret must not pass as step-up.
		if err := svc.VerifyAdminStepUp(sign("2fa", time.Now().Add(time.Minute)), user.ID, session); !errors.Is(err, ErrStepUpInvalid) {
			t.Fatalf("2fa subject: err = %v", err)
		}
	})
}
