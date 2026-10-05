package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
)

const freeMonthlyPlanCode = "free-monthly"

type subscriptionCreator interface {
	Create(ctx context.Context, sub *domain.Subscription) error
}

type planByCodeRepository interface {
	GetByCode(ctx context.Context, code string) (*domain.Plan, error)
}

type userCreator interface {
	Create(ctx context.Context, user *domain.User) error
}

type organizationWriter interface {
	Create(ctx context.Context, org *domain.Organization) error
	Update(ctx context.Context, org *domain.Organization) error
}

type organizationUserCreator interface {
	Create(ctx context.Context, orgUser *domain.OrganizationUser) error
}

type collectionCreator interface {
	Create(ctx context.Context, collection *domain.Collection) error
}

type organizationFolderCreator interface {
	Create(ctx context.Context, folder *domain.OrganizationFolder) error
}

// PersonalVaultProvisioner atomically creates the complete account aggregate.
type PersonalVaultProvisioner interface {
	Provision(ctx context.Context, user *domain.User, encryptedOrgKey string) error
}

type personalVaultProvisioner struct {
	txManager      repository.TxManager
	userRepo       userCreator
	orgRepo        organizationWriter
	orgUserRepo    organizationUserCreator
	collectionRepo collectionCreator
	folderRepo     organizationFolderCreator
	subRepo        subscriptionCreator
	planRepo       planByCodeRepository
}

func NewPersonalVaultProvisioner(
	txManager repository.TxManager,
	userRepo userCreator,
	orgRepo organizationWriter,
	orgUserRepo organizationUserCreator,
	collectionRepo collectionCreator,
	folderRepo organizationFolderCreator,
	subRepo subscriptionCreator,
	planRepo planByCodeRepository,
) PersonalVaultProvisioner {
	return &personalVaultProvisioner{
		txManager:      txManager,
		userRepo:       userRepo,
		orgRepo:        orgRepo,
		orgUserRepo:    orgUserRepo,
		collectionRepo: collectionRepo,
		folderRepo:     folderRepo,
		subRepo:        subRepo,
		planRepo:       planRepo,
	}
}

func (p *personalVaultProvisioner) Provision(ctx context.Context, user *domain.User, encryptedOrgKey string) error {
	if user == nil {
		return repository.ErrInvalidInput
	}

	return p.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		creatorEmail, creatorName := user.Email, user.Name
		org := &domain.Organization{
			Name:               "Personal Vault",
			BillingEmail:       user.Email,
			EncryptedOrgKey:    encryptedOrgKey,
			IsActive:           true,
			Status:             domain.OrgStatusActive,
			IsPersonal:         true,
			CreatedByUserEmail: &creatorEmail,
			CreatedByUserName:  &creatorName,
		}
		if err := p.orgRepo.Create(txCtx, org); err != nil {
			return fmt.Errorf("create personal organization: %w", err)
		}

		user.PersonalOrganizationID = org.ID
		user.DefaultOrganizationID = org.ID
		if err := p.userRepo.Create(txCtx, user); err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		now := time.Now().UTC()
		membership := &domain.OrganizationUser{
			OrganizationID:  org.ID,
			UserID:          user.ID,
			Role:            domain.OrgRoleOwner,
			EncryptedOrgKey: encryptedOrgKey,
			AccessAll:       true,
			Status:          domain.OrgUserStatusConfirmed,
			InvitedAt:       &now,
			AcceptedAt:      &now,
		}
		if err := p.orgUserRepo.Create(txCtx, membership); err != nil {
			return fmt.Errorf("create personal owner membership: %w", err)
		}

		ownerID := user.ID
		org.CreatedByUserID = &ownerID
		org.PersonalOwnerUserID = &ownerID
		if err := p.orgRepo.Update(txCtx, org); err != nil {
			return fmt.Errorf("finalize personal organization: %w", err)
		}

		if err := p.collectionRepo.Create(txCtx, &domain.Collection{
			UUID:           uuid.New(),
			OrganizationID: org.ID,
			Name:           "General",
			Description:    "System default collection",
			IsDefault:      true,
			IsPrivate:      false,
		}); err != nil {
			return fmt.Errorf("create default collection: %w", err)
		}

		for _, name := range constants.DefaultPersonalVaultFolders {
			if err := p.folderRepo.Create(txCtx, &domain.OrganizationFolder{
				UUID:            uuid.New(),
				OrganizationID:  org.ID,
				CreatedByUserID: user.ID,
				Name:            name,
			}); err != nil {
				return fmt.Errorf("create default folder %q: %w", name, err)
			}
		}

		plan, err := p.planRepo.GetByCode(txCtx, freeMonthlyPlanCode)
		if err != nil {
			return fmt.Errorf("get free plan: %w", err)
		}
		if plan == nil {
			return fmt.Errorf("get free plan: %w", repository.ErrNotFound)
		}
		if err := p.subRepo.Create(txCtx, &domain.Subscription{
			UUID:           uuid.New(),
			OrganizationID: org.ID,
			PlanID:         plan.ID,
			State:          domain.SubStateActive,
			StartedAt:      &now,
		}); err != nil {
			return fmt.Errorf("create free subscription: %w", err)
		}

		return nil
	})
}
