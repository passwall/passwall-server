package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/hash"
)

type tokenKindRepoStub struct {
	repository.TokenRepository
	tokens map[string]*domain.Token
}

func (r *tokenKindRepoStub) GetByUUID(_ context.Context, id string) (*domain.Token, error) {
	if token, ok := r.tokens[id]; ok {
		return token, nil
	}
	return nil, repository.ErrNotFound
}

func (r *tokenKindRepoStub) DeleteByUUID(_ context.Context, id string) error {
	delete(r.tokens, id)
	return nil
}

type tokenKindUserRepoStub struct {
	repository.UserRepository
	user *domain.User
}

func (r tokenKindUserRepoStub) GetByUUID(context.Context, string) (*domain.User, error) {
	return r.user, nil
}

func TestValidateTokenRejectsRefreshTokensAsBearer(t *testing.T) {
	user := &domain.User{ID: 7, UUID: uuid.New(), Email: "admin@example.com"}
	svc := &authService{
		config:   &AuthConfig{JWTSecret: "test-secret-0123456789", AccessTokenDuration: "30m", RefreshTokenDuration: "15d"},
		userRepo: tokenKindUserRepoStub{user: user},
	}
	details, err := svc.createToken(user, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	stored := func(kind, token string, id uuid.UUID) *domain.Token {
		return &domain.Token{Kind: kind, Token: hash.SHA256(token), ExpiryTime: time.Now().Add(time.Hour), UUID: id}
	}

	for _, tt := range []struct {
		name    string
		kind    string
		token   string
		id      uuid.UUID
		wantErr error
	}{
		{name: "access token", kind: tokenKindAccess, token: details.AccessToken, id: details.AtUUID},
		{name: "legacy row without kind", kind: "", token: details.AccessToken, id: details.AtUUID},
		{name: "refresh token", kind: tokenKindRefresh, token: details.RefreshToken, id: details.RtUUID, wantErr: ErrUnauthorized},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc.tokenRepo = &tokenKindRepoStub{tokens: map[string]*domain.Token{tt.id.String(): stored(tt.kind, tt.token, tt.id)}}
			claims, err := svc.ValidateToken(context.Background(), tt.token)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ValidateToken() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || claims.UserID != user.ID {
				t.Fatalf("ValidateToken() = %+v, %v", claims, err)
			}
		})
	}
}
