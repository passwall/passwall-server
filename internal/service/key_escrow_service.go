package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

var (
	ErrEscrowNotConfigured = errors.New("key escrow is not configured on this server")
	ErrEscrowNotEnabled    = errors.New("key escrow is not enabled for this organization's SSO")
	ErrEscrowAlreadyExists = errors.New("key escrow already exists for this user and organization")
	ErrEscrowNotFound      = errors.New("key escrow not found")
	ErrInvalidEscrowKey    = errors.New("invalid escrow master key configuration")
)

// KeyEscrowService handles SSO key escrow operations.
// Only the organization key is escrowed — the user's personal vault key is never stored.
// On SSO login the server returns the org key so the client can decrypt org items;
// personal vault items remain locked until the user enters their master password.
type KeyEscrowService interface {
	// EnableForOrg generates an org escrow key (if not exists) when admin enables key escrow on SSO connection
	EnableForOrg(ctx context.Context, orgID uint) error

	// EnrollUser wraps the organization's symmetric key with the org escrow key and stores it
	EnrollUser(ctx context.Context, userID, orgID uint, rawOrgKeyB64 string) error

	// GetOrgKey retrieves and decrypts the escrowed org key (used during SSO login)
	GetOrgKey(ctx context.Context, userID, orgID uint) (string, error)

	// RevokeUser removes a user's escrowed key (offboarding)
	RevokeUser(ctx context.Context, userID, orgID uint) error

	// GetStatus returns key escrow enrollment status for a user in an org
	GetStatus(ctx context.Context, userID, orgID uint) (*domain.KeyEscrowStatusResponse, error)

	// IsConfigured returns true if the server has an escrow master key configured
	IsConfigured() bool
}

type keyEscrowService struct {
	escrowRepo    repository.KeyEscrowRepository
	orgKeyRepo    repository.OrgEscrowKeyRepository
	ssoConnRepo   repository.SSOConnectionRepository
	masterKey     []byte // 256-bit server escrow master key
	logger        Logger
}

func NewKeyEscrowService(
	escrowRepo repository.KeyEscrowRepository,
	orgKeyRepo repository.OrgEscrowKeyRepository,
	ssoConnRepo repository.SSOConnectionRepository,
	escrowMasterKeyHex string,
	logger Logger,
) KeyEscrowService {
	var masterKey []byte
	if escrowMasterKeyHex != "" {
		decoded, err := hex.DecodeString(escrowMasterKeyHex)
		if err == nil && len(decoded) == 32 {
			masterKey = decoded
		} else {
			logger.Error("invalid escrow_master_key in config: must be 64 hex chars (256-bit)")
		}
	}

	return &keyEscrowService{
		escrowRepo:  escrowRepo,
		orgKeyRepo:  orgKeyRepo,
		ssoConnRepo: ssoConnRepo,
		masterKey:   masterKey,
		logger:      logger,
	}
}

func (s *keyEscrowService) IsConfigured() bool {
	return len(s.masterKey) == 32
}

func (s *keyEscrowService) EnableForOrg(ctx context.Context, orgID uint) error {
	if !s.IsConfigured() {
		return ErrEscrowNotConfigured
	}

	// Check if org already has an escrow key
	existing, err := s.orgKeyRepo.GetByOrganizationID(ctx, orgID)
	if err == nil && existing != nil {
		s.logger.Info("org escrow key already exists", "org_id", orgID)
		return nil
	}

	// Generate random 256-bit org escrow key
	orgKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, orgKey); err != nil {
		return fmt.Errorf("failed to generate org escrow key: %w", err)
	}

	// Encrypt org key with server master key
	encryptedOrgKey, err := encryptAESGCM(s.masterKey, orgKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt org escrow key: %w", err)
	}

	orgEscrowKey := &domain.OrgEscrowKey{
		OrganizationID: orgID,
		EncryptedKey:   base64.StdEncoding.EncodeToString(encryptedOrgKey),
		KeyVersion:     1,
		Status:         domain.KeyEscrowStatusActive,
	}

	if err := s.orgKeyRepo.Create(ctx, orgEscrowKey); err != nil {
		return fmt.Errorf("failed to store org escrow key: %w", err)
	}

	s.logger.Info("org escrow key created", "org_id", orgID)
	return nil
}

func (s *keyEscrowService) EnrollUser(ctx context.Context, userID, orgID uint, rawOrgKeyB64 string) error {
	if !s.IsConfigured() {
		return ErrEscrowNotConfigured
	}

	// Check if already enrolled
	existing, err := s.escrowRepo.GetByUserAndOrg(ctx, userID, orgID)
	if err == nil && existing != nil {
		return ErrEscrowAlreadyExists
	}

	// Decode the raw org key sent by the client
	rawOrgKey, err := base64.StdEncoding.DecodeString(rawOrgKeyB64)
	if err != nil {
		return fmt.Errorf("invalid org key encoding: %w", err)
	}
	if len(rawOrgKey) != 64 {
		return fmt.Errorf("invalid org key: expected 64 bytes (SymmetricKey), got %d", len(rawOrgKey))
	}

	// Get org escrow key (server-side wrapping key)
	escrowKey, err := s.getDecryptedOrgKey(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get org escrow key: %w", err)
	}

	// Encrypt org key with org escrow key
	wrappedKey, err := encryptAESGCM(escrowKey, rawOrgKey)
	if err != nil {
		return fmt.Errorf("failed to wrap org key: %w", err)
	}

	escrow := &domain.KeyEscrow{
		UserID:         userID,
		OrganizationID: orgID,
		WrappedOrgKey:  base64.StdEncoding.EncodeToString(wrappedKey),
		KeyVersion:     1,
		Status:         domain.KeyEscrowStatusActive,
	}

	if err := s.escrowRepo.Create(ctx, escrow); err != nil {
		return fmt.Errorf("failed to store escrowed org key: %w", err)
	}

	s.logger.Info("org key escrowed", "user_id", userID, "org_id", orgID)
	return nil
}

func (s *keyEscrowService) GetOrgKey(ctx context.Context, userID, orgID uint) (string, error) {
	if !s.IsConfigured() {
		return "", ErrEscrowNotConfigured
	}

	escrow, err := s.escrowRepo.GetByUserAndOrg(ctx, userID, orgID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrEscrowNotFound
		}
		return "", fmt.Errorf("failed to get escrowed key: %w", err)
	}
	if !escrow.IsActive() {
		return "", ErrEscrowNotFound
	}

	// Get org escrow key (server-side wrapping key)
	escrowKey, err := s.getDecryptedOrgKey(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to get org escrow key: %w", err)
	}

	// Decrypt the wrapped org key
	wrappedKeyBytes, err := base64.StdEncoding.DecodeString(escrow.WrappedOrgKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode wrapped key: %w", err)
	}

	rawOrgKey, err := decryptAESGCM(escrowKey, wrappedKeyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to unwrap org key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(rawOrgKey), nil
}

func (s *keyEscrowService) RevokeUser(ctx context.Context, userID, orgID uint) error {
	if err := s.escrowRepo.DeleteByUserAndOrg(ctx, userID, orgID); err != nil {
		return fmt.Errorf("failed to revoke escrowed key: %w", err)
	}
	s.logger.Info("org key escrow revoked", "user_id", userID, "org_id", orgID)
	return nil
}

func (s *keyEscrowService) GetStatus(ctx context.Context, userID, orgID uint) (*domain.KeyEscrowStatusResponse, error) {
	// Check if SSO connection has escrow enabled
	conn, err := s.ssoConnRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return &domain.KeyEscrowStatusResponse{Enabled: false, Enrolled: false}, nil
	}

	enabled := conn.KeyEscrowEnabled

	// Check if user is enrolled
	escrow, err := s.escrowRepo.GetByUserAndOrg(ctx, userID, orgID)
	if err != nil || escrow == nil {
		return &domain.KeyEscrowStatusResponse{Enabled: enabled, Enrolled: false}, nil
	}

	return &domain.KeyEscrowStatusResponse{
		Enabled:    enabled,
		Enrolled:   escrow.IsActive(),
		KeyVersion: escrow.KeyVersion,
	}, nil
}

// getDecryptedOrgKey retrieves and decrypts the org escrow key
func (s *keyEscrowService) getDecryptedOrgKey(ctx context.Context, orgID uint) ([]byte, error) {
	orgKeyRecord, err := s.orgKeyRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("org escrow key not found; enable key escrow for this organization first")
		}
		return nil, err
	}

	encKeyBytes, err := base64.StdEncoding.DecodeString(orgKeyRecord.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode org escrow key: %w", err)
	}

	orgKey, err := decryptAESGCM(s.masterKey, encKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt org escrow key: %w", err)
	}

	return orgKey, nil
}

// --- AES-256-GCM helpers ---
// Format: nonce (12 bytes) || ciphertext || GCM tag (16 bytes)

func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func decryptAESGCM(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
