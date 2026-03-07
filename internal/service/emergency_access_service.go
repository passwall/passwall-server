package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
)

// EmergencyAccessService defines the business logic for emergency access
type EmergencyAccessService interface {
	Invite(ctx context.Context, grantorID uint, granteeEmail string) (*domain.EmergencyAccess, error)
	ListGranted(ctx context.Context, grantorID uint) ([]*domain.EmergencyAccess, error)
	ListTrusted(ctx context.Context, granteeID uint, granteeEmail string) ([]*domain.EmergencyAccess, error)
	Accept(ctx context.Context, granteeID uint, eaUUID string) (*domain.EmergencyAccess, error)
	Confirm(ctx context.Context, grantorID uint, eaUUID string, keyEncrypted string) (*domain.EmergencyAccess, error)
	RequestRecovery(ctx context.Context, granteeID uint, eaUUID string) (*domain.EmergencyAccess, error)
	ApproveRecovery(ctx context.Context, grantorID uint, eaUUID string) (*domain.EmergencyAccess, error)
	RejectRecovery(ctx context.Context, grantorID uint, eaUUID string) (*domain.EmergencyAccess, error)
	Revoke(ctx context.Context, grantorID uint, eaUUID string) error
	GetVaultForRecovery(ctx context.Context, granteeID uint, eaUUID string) (*EmergencyVaultResponse, error)
}

// EmergencyVaultResponse contains the grantor's encrypted vault for recovery view
type EmergencyVaultResponse struct {
	KeyEncrypted string                     `json:"key_encrypted"`
	Items        []*domain.OrganizationItem `json:"items"`
}

type emergencyAccessService struct {
	eaRepo       repository.EmergencyAccessRepository
	userRepo     repository.UserRepository
	orgItemRepo  repository.OrganizationItemRepository
	emailSender  email.Sender
	emailBuilder *email.EmailBuilder
	logger       Logger
}

func NewEmergencyAccessService(
	eaRepo repository.EmergencyAccessRepository,
	userRepo repository.UserRepository,
	orgItemRepo repository.OrganizationItemRepository,
	emailSender email.Sender,
	emailBuilder *email.EmailBuilder,
	logger Logger,
) EmergencyAccessService {
	return &emergencyAccessService{
		eaRepo:       eaRepo,
		userRepo:     userRepo,
		orgItemRepo:  orgItemRepo,
		emailSender:  emailSender,
		emailBuilder: emailBuilder,
		logger:       logger,
	}
}

func (s *emergencyAccessService) Invite(ctx context.Context, grantorID uint, granteeEmail string) (*domain.EmergencyAccess, error) {
	granteeEmail = strings.TrimSpace(strings.ToLower(granteeEmail))
	if granteeEmail == "" {
		return nil, repository.ErrInvalidInput
	}

	grantor, err := s.userRepo.GetByID(ctx, grantorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get grantor: %w", err)
	}

	if strings.EqualFold(grantor.Email, granteeEmail) {
		return nil, repository.ErrInvalidInput
	}

	ea := &domain.EmergencyAccess{
		UUID:         uuid.New(),
		GrantorID:    grantorID,
		GranteeEmail: granteeEmail,
		Status:       domain.EAStatusInvited,
	}

	if err := s.eaRepo.Create(ctx, ea); err != nil {
		return nil, fmt.Errorf("failed to create emergency access: %w", err)
	}

	go s.sendEmergencyInviteEmail(grantor, granteeEmail)

	ea.Grantor = grantor
	return ea, nil
}

func (s *emergencyAccessService) ListGranted(ctx context.Context, grantorID uint) ([]*domain.EmergencyAccess, error) {
	return s.eaRepo.ListByGrantor(ctx, grantorID)
}

func (s *emergencyAccessService) ListTrusted(ctx context.Context, granteeID uint, granteeEmail string) ([]*domain.EmergencyAccess, error) {
	accepted, err := s.eaRepo.ListByGrantee(ctx, granteeID)
	if err != nil {
		return nil, err
	}

	pending, err := s.eaRepo.ListByGranteeEmail(ctx, granteeEmail)
	if err != nil {
		return nil, err
	}

	return append(accepted, pending...), nil
}

func (s *emergencyAccessService) Accept(ctx context.Context, granteeID uint, eaUUID string) (*domain.EmergencyAccess, error) {
	ea, err := s.eaRepo.GetByUUID(ctx, eaUUID)
	if err != nil {
		return nil, err
	}

	if ea.Status != domain.EAStatusInvited {
		return nil, repository.ErrInvalidInput
	}

	grantee, err := s.userRepo.GetByID(ctx, granteeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get grantee: %w", err)
	}

	if !strings.EqualFold(grantee.Email, ea.GranteeEmail) {
		return nil, repository.ErrForbidden
	}

	ea.GranteeID = &granteeID
	ea.Status = domain.EAStatusAccepted

	if err := s.eaRepo.Update(ctx, ea); err != nil {
		return nil, fmt.Errorf("failed to update emergency access: %w", err)
	}

	go s.sendEmergencyAcceptedEmail(ea.GrantorID, grantee)

	return ea, nil
}

func (s *emergencyAccessService) Confirm(ctx context.Context, grantorID uint, eaUUID string, keyEncrypted string) (*domain.EmergencyAccess, error) {
	if strings.TrimSpace(keyEncrypted) == "" {
		return nil, repository.ErrInvalidInput
	}

	ea, err := s.eaRepo.GetByUUID(ctx, eaUUID)
	if err != nil {
		return nil, err
	}

	if ea.GrantorID != grantorID {
		return nil, repository.ErrForbidden
	}

	if ea.Status != domain.EAStatusAccepted {
		return nil, repository.ErrInvalidInput
	}

	ea.KeyEncrypted = &keyEncrypted
	ea.Status = domain.EAStatusConfirmed

	if err := s.eaRepo.Update(ctx, ea); err != nil {
		return nil, fmt.Errorf("failed to confirm emergency access: %w", err)
	}

	return ea, nil
}

func (s *emergencyAccessService) RequestRecovery(ctx context.Context, granteeID uint, eaUUID string) (*domain.EmergencyAccess, error) {
	ea, err := s.eaRepo.GetByUUID(ctx, eaUUID)
	if err != nil {
		return nil, err
	}

	if ea.GranteeID == nil || *ea.GranteeID != granteeID {
		return nil, repository.ErrForbidden
	}

	if ea.Status != domain.EAStatusConfirmed {
		return nil, repository.ErrInvalidInput
	}

	now := time.Now()
	ea.Status = domain.EAStatusRecoveryRequested
	ea.RecoveryInitAt = &now

	if err := s.eaRepo.Update(ctx, ea); err != nil {
		return nil, fmt.Errorf("failed to request recovery: %w", err)
	}

	grantee, _ := s.userRepo.GetByID(ctx, granteeID)
	go s.sendRecoveryRequestEmail(ea.GrantorID, grantee)

	return ea, nil
}

func (s *emergencyAccessService) ApproveRecovery(ctx context.Context, grantorID uint, eaUUID string) (*domain.EmergencyAccess, error) {
	ea, err := s.eaRepo.GetByUUID(ctx, eaUUID)
	if err != nil {
		return nil, err
	}

	if ea.GrantorID != grantorID {
		return nil, repository.ErrForbidden
	}

	if ea.Status != domain.EAStatusRecoveryRequested {
		return nil, repository.ErrInvalidInput
	}

	now := time.Now()
	ea.Status = domain.EAStatusRecoveryApproved
	ea.RecoveryApproveAt = &now

	if err := s.eaRepo.Update(ctx, ea); err != nil {
		return nil, fmt.Errorf("failed to approve recovery: %w", err)
	}

	if ea.GranteeID != nil {
		go s.sendRecoveryApprovedEmail(*ea.GranteeID)
	}

	return ea, nil
}

func (s *emergencyAccessService) RejectRecovery(ctx context.Context, grantorID uint, eaUUID string) (*domain.EmergencyAccess, error) {
	ea, err := s.eaRepo.GetByUUID(ctx, eaUUID)
	if err != nil {
		return nil, err
	}

	if ea.GrantorID != grantorID {
		return nil, repository.ErrForbidden
	}

	if ea.Status != domain.EAStatusRecoveryRequested {
		return nil, repository.ErrInvalidInput
	}

	ea.Status = domain.EAStatusRecoveryRejected

	if err := s.eaRepo.Update(ctx, ea); err != nil {
		return nil, fmt.Errorf("failed to reject recovery: %w", err)
	}

	return ea, nil
}

func (s *emergencyAccessService) Revoke(ctx context.Context, grantorID uint, eaUUID string) error {
	ea, err := s.eaRepo.GetByUUID(ctx, eaUUID)
	if err != nil {
		return err
	}

	if ea.GrantorID != grantorID {
		return repository.ErrForbidden
	}

	return s.eaRepo.Delete(ctx, ea.ID)
}

func (s *emergencyAccessService) GetVaultForRecovery(ctx context.Context, granteeID uint, eaUUID string) (*EmergencyVaultResponse, error) {
	ea, err := s.eaRepo.GetByUUID(ctx, eaUUID)
	if err != nil {
		return nil, err
	}

	if ea.GranteeID == nil || *ea.GranteeID != granteeID {
		return nil, repository.ErrForbidden
	}

	if ea.Status != domain.EAStatusRecoveryApproved {
		return nil, repository.ErrForbidden
	}

	if ea.KeyEncrypted == nil {
		return nil, fmt.Errorf("key exchange not completed")
	}

	grantor, err := s.userRepo.GetByID(ctx, ea.GrantorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get grantor: %w", err)
	}

	// Only fetch items from grantor's personal vault organization
	items, _, err := s.orgItemRepo.ListByOrganization(ctx, repository.OrganizationItemFilter{
		OrganizationID: grantor.PersonalOrganizationID,
		PerPage:        10000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list personal vault items: %w", err)
	}
	allItems := items

	// Reset status back to confirmed after vault is accessed
	ea.Status = domain.EAStatusConfirmed
	ea.RecoveryInitAt = nil
	ea.RecoveryApproveAt = nil
	if updateErr := s.eaRepo.Update(ctx, ea); updateErr != nil {
		s.logger.Error("failed to reset emergency access status", "error", updateErr)
	}

	return &EmergencyVaultResponse{
		KeyEncrypted: *ea.KeyEncrypted,
		Items:        allItems,
	}, nil
}

// Re-encrypt emergency access keys when user changes master password
func (s *emergencyAccessService) ReKeyForUser(ctx context.Context, grantorID uint, reKeyFn func(oldKeyEncrypted string) (string, error)) error {
	grants, err := s.eaRepo.ListConfirmedByGrantor(ctx, grantorID)
	if err != nil {
		return fmt.Errorf("failed to list confirmed grants: %w", err)
	}

	for _, ea := range grants {
		if ea.KeyEncrypted == nil {
			continue
		}

		newKey, err := reKeyFn(*ea.KeyEncrypted)
		if err != nil {
			s.logger.Error("failed to re-key emergency access", "ea_id", ea.ID, "error", err)
			continue
		}

		ea.KeyEncrypted = &newKey
		if err := s.eaRepo.Update(ctx, ea); err != nil {
			s.logger.Error("failed to update re-keyed emergency access", "ea_id", ea.ID, "error", err)
		}
	}

	return nil
}

// Email helpers

func (s *emergencyAccessService) sendEmergencyInviteEmail(grantor *domain.User, granteeEmail string) {
	if s.emailSender == nil || s.emailBuilder == nil {
		return
	}

	grantorName := grantor.Name
	if grantorName == "" {
		grantorName = grantor.Email
	}

	msg, err := s.emailBuilder.BuildEmergencyInviteEmail(granteeEmail, grantorName)
	if err != nil {
		s.logger.Error("failed to build emergency invite email", "error", err)
		return
	}

	if err := s.emailSender.Send(context.Background(), msg); err != nil {
		s.logger.Error("failed to send emergency invite email", "error", err)
	}
}

func (s *emergencyAccessService) sendEmergencyAcceptedEmail(grantorID uint, grantee *domain.User) {
	if s.emailSender == nil || s.emailBuilder == nil {
		return
	}

	grantor, err := s.userRepo.GetByID(context.Background(), grantorID)
	if err != nil {
		return
	}

	granteeName := grantee.Name
	if granteeName == "" {
		granteeName = grantee.Email
	}

	msg, buildErr := s.emailBuilder.BuildEmergencyAcceptedEmail(grantor.Email, granteeName)
	if buildErr != nil {
		s.logger.Error("failed to build emergency accepted email", "error", buildErr)
		return
	}

	if sendErr := s.emailSender.Send(context.Background(), msg); sendErr != nil {
		s.logger.Error("failed to send emergency accepted email", "error", sendErr)
	}
}

func (s *emergencyAccessService) sendRecoveryRequestEmail(grantorID uint, grantee *domain.User) {
	if s.emailSender == nil || s.emailBuilder == nil {
		return
	}

	grantor, err := s.userRepo.GetByID(context.Background(), grantorID)
	if err != nil {
		return
	}

	granteeName := ""
	if grantee != nil {
		granteeName = grantee.Name
		if granteeName == "" {
			granteeName = grantee.Email
		}
	}

	msg, buildErr := s.emailBuilder.BuildEmergencyRecoveryRequestEmail(grantor.Email, granteeName)
	if buildErr != nil {
		s.logger.Error("failed to build recovery request email", "error", buildErr)
		return
	}

	if sendErr := s.emailSender.Send(context.Background(), msg); sendErr != nil {
		s.logger.Error("failed to send recovery request email", "error", sendErr)
	}
}

func (s *emergencyAccessService) sendRecoveryApprovedEmail(granteeID uint) {
	if s.emailSender == nil || s.emailBuilder == nil {
		return
	}

	grantee, err := s.userRepo.GetByID(context.Background(), granteeID)
	if err != nil {
		return
	}

	msg, buildErr := s.emailBuilder.BuildEmergencyRecoveryApprovedEmail(grantee.Email)
	if buildErr != nil {
		s.logger.Error("failed to build recovery approved email", "error", buildErr)
		return
	}

	if sendErr := s.emailSender.Send(context.Background(), msg); sendErr != nil {
		s.logger.Error("failed to send recovery approved email", "error", sendErr)
	}
}
