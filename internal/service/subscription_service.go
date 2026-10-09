package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/stripe"
	"gorm.io/gorm"
)

const (
	ManualSubscriptionCodePlanNotAllowed    = "PLAN_NOT_ALLOWED_FOR_ORG"
	ManualSubscriptionCodeExternalActive    = "EXTERNAL_SUBSCRIPTION_ACTIVE"
	ManualSubscriptionCodeSeatsBelowMembers = "SEATS_BELOW_MEMBERS"
	ManualSubscriptionCodeInvalidEndDate    = "INVALID_END_DATE"
	ManualSubscriptionCodeUsageExceedsPlan  = "USAGE_EXCEEDS_PLAN"
	ManualSubscriptionCodeNotActiveManual   = "MANUAL_SUBSCRIPTION_NOT_ACTIVE"
	ManualSubscriptionCodeNoteRequired      = "NOTE_REQUIRED"
)

type ManualSubscriptionError struct {
	Code    string
	Message string
}

func (e *ManualSubscriptionError) Error() string { return e.Message }

type ManualSubscriptionInput struct {
	PlanCode string
	EndsAt   time.Time
	Seats    *int
	Note     string
}

type ManualSubscriptionChange struct {
	Subscription *domain.Subscription
	Organization *domain.Organization
	OldPlan      string
	NewPlan      string
	Provider     domain.PaymentProvider
	EndsAt       *time.Time
	Seats        *int
}

// SubscriptionService handles subscription operations
type SubscriptionService interface {
	Create(ctx context.Context, orgID uint, planCode string, stripeSubscriptionID string, seatsPurchased *int) (*domain.Subscription, error)
	CreateFromProvider(ctx context.Context, orgID uint, planCode string, providerSubscriptionID string, seatsPurchased *int, trialEndsAt *time.Time) (*domain.Subscription, error)
	GetByID(ctx context.Context, id uint) (*domain.Subscription, error)
	GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	Update(ctx context.Context, sub *domain.Subscription) error
	UpdateSeatsPurchasedByStripeSubscriptionID(ctx context.Context, stripeSubID string, seatsPurchased *int) error
	SyncProviderPeriod(ctx context.Context, stripeSubID string, periodEnd time.Time, trialEnd *time.Time) error
	Upgrade(ctx context.Context, orgID uint, planCode string) error
	Downgrade(ctx context.Context, orgID uint, planCode string) error
	Cancel(ctx context.Context, orgID uint) error
	Resume(ctx context.Context, orgID uint) error
	Renew(ctx context.Context, subID uint) error
	HandlePaymentSuccess(ctx context.Context, stripeSubID string) error
	HandlePaymentFailed(ctx context.Context, stripeSubID string) error
	HandleProviderCanceled(ctx context.Context, subID uint, endedAt *time.Time) error
	GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error)
	ExpireSubscription(ctx context.Context, subID uint) error
	CheckExpiredSubscriptions(ctx context.Context) error
	GrantManual(ctx context.Context, orgID uint, input ManualSubscriptionInput) (*ManualSubscriptionChange, error)
	ExtendManual(ctx context.Context, orgID uint, endsAt time.Time, note string) (*ManualSubscriptionChange, error)
	EndManual(ctx context.Context, orgID uint, note string) (*ManualSubscriptionChange, error)
	SendManualExpiryReminders(ctx context.Context) error
}

type subscriptionService struct {
	subRepo interface {
		ExpireActiveByOrganizationID(ctx context.Context, orgID uint, endedAt time.Time) error
		Create(ctx context.Context, sub *domain.Subscription) error
		GetByID(ctx context.Context, id uint) (*domain.Subscription, error)
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
		GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error)
		Update(ctx context.Context, sub *domain.Subscription) error
		ListPastDueExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListCanceledExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListTrialEnding(ctx context.Context, before time.Time) ([]*domain.Subscription, error)
		ListManualExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListManualEndingSoon(ctx context.Context, before time.Time) ([]*domain.Subscription, error)
		LockOrganization(ctx context.Context, orgID uint) error
	}
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	}
	orgRepo interface {
		GetByID(ctx context.Context, id uint) (*domain.Organization, error)
		GetMemberCount(ctx context.Context, orgID uint) (int, error)
		GetCollectionCount(ctx context.Context, orgID uint) (int, error)
		GetItemCount(ctx context.Context, orgID uint) (int, error)
	}
	orgService OrganizationService
	// emailService interface for sending emails (optional - can be nil)
	emailService interface {
		SendPaymentFailedEmail(ctx context.Context, sub *domain.Subscription) error
		SendSubscriptionExpiredEmail(ctx context.Context, sub *domain.Subscription) error
	}
	// stripe client for cancel/reactivate operations
	stripe any
	// logger for structured logging
	logger    Logger
	txManager interface {
		WithinTx(ctx context.Context, fn func(context.Context) error) error
	}
	manualEmailSender  email.Sender
	manualEmailBuilder *email.EmailBuilder
	orgUserRepo        repository.OrganizationUserRepository
	frontendURL        string
}

// NewSubscriptionService creates a new subscription service
func NewSubscriptionService(
	subRepo interface {
		ExpireActiveByOrganizationID(ctx context.Context, orgID uint, endedAt time.Time) error
		Create(ctx context.Context, sub *domain.Subscription) error
		GetByID(ctx context.Context, id uint) (*domain.Subscription, error)
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
		GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error)
		Update(ctx context.Context, sub *domain.Subscription) error
		ListPastDueExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListCanceledExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListTrialEnding(ctx context.Context, before time.Time) ([]*domain.Subscription, error)
		ListManualExpired(ctx context.Context) ([]*domain.Subscription, error)
		ListManualEndingSoon(ctx context.Context, before time.Time) ([]*domain.Subscription, error)
		LockOrganization(ctx context.Context, orgID uint) error
	},
	planRepo interface {
		GetByCode(ctx context.Context, code string) (*domain.Plan, error)
		GetByID(ctx context.Context, id uint) (*domain.Plan, error)
	},
	orgRepo interface {
		GetByID(ctx context.Context, id uint) (*domain.Organization, error)
		GetMemberCount(ctx context.Context, orgID uint) (int, error)
		GetCollectionCount(ctx context.Context, orgID uint) (int, error)
		GetItemCount(ctx context.Context, orgID uint) (int, error)
	},
	orgService OrganizationService,
	emailService interface {
		SendPaymentFailedEmail(ctx context.Context, sub *domain.Subscription) error
		SendSubscriptionExpiredEmail(ctx context.Context, sub *domain.Subscription) error
	},
	stripe any,
	logger Logger,
	txManager interface {
		WithinTx(ctx context.Context, fn func(context.Context) error) error
	},
	options ...SubscriptionServiceOption,
) SubscriptionService {
	service := &subscriptionService{
		subRepo:      subRepo,
		planRepo:     planRepo,
		orgRepo:      orgRepo,
		orgService:   orgService,
		emailService: emailService,
		stripe:       stripe,
		logger:       logger,
		txManager:    txManager,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type SubscriptionServiceOption func(*subscriptionService)

func WithManualSubscriptionEmails(sender email.Sender, builder *email.EmailBuilder, orgUsers repository.OrganizationUserRepository, frontendURL string) SubscriptionServiceOption {
	return func(service *subscriptionService) {
		service.manualEmailSender = sender
		service.manualEmailBuilder = builder
		service.orgUserRepo = orgUsers
		service.frontendURL = strings.TrimRight(frontendURL, "/")
	}
}

// Create creates a new subscription. A trial is derived from the plan catalog.
func (s *subscriptionService) Create(ctx context.Context, orgID uint, planCode string, stripeSubscriptionID string, seatsPurchased *int) (*domain.Subscription, error) {
	return s.create(ctx, orgID, planCode, stripeSubscriptionID, seatsPurchased, nil, true)
}

// CreateFromProvider creates a subscription whose trial is decided by the
// billing provider: trialEndsAt is the provider trial end, or nil when the
// provider started the subscription without a trial.
func (s *subscriptionService) CreateFromProvider(ctx context.Context, orgID uint, planCode string, providerSubscriptionID string, seatsPurchased *int, trialEndsAt *time.Time) (*domain.Subscription, error) {
	return s.create(ctx, orgID, planCode, providerSubscriptionID, seatsPurchased, trialEndsAt, false)
}

func (s *subscriptionService) create(
	ctx context.Context,
	orgID uint,
	planCode string,
	stripeSubscriptionID string,
	seatsPurchased *int,
	providerTrialEnd *time.Time,
	usePlanTrial bool,
) (*domain.Subscription, error) {
	s.logger.Info("subscription.create called",
		"org_id", orgID,
		"plan_code", planCode,
		"stripe_subscription_id_set", stripeSubscriptionID != "",
	)
	// Idempotency for Stripe retries: if we already have this Stripe subscription, return it.
	if stripeSubscriptionID != "" {
		if existing, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubscriptionID); err == nil && existing != nil {
			s.logger.Info("subscription.create idempotent hit",
				"org_id", orgID,
				"stripe_subscription_id", stripeSubscriptionID,
			)
			return existing, nil
		}
	}

	// Enforce plan–organization type invariant:
	// personal vaults can only have free/pro plans; multi-user plans require a non-personal org.
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		s.logger.Error("subscription.create failed to get organization", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("organization not found: %w", err)
	}
	if org.IsPersonal && domain.IsMultiUserPlan(planCode) {
		s.logger.Warn("subscription.create rejected: multi-user plan on personal vault",
			"org_id", orgID, "plan_code", planCode)
		return nil, fmt.Errorf("personal vaults can only use free or pro plans; create a separate organization for %s", planCode)
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(planCode) && planCode != string(domain.PlanFree) {
		s.logger.Warn("subscription.create rejected: personal plan on organization",
			"org_id", orgID, "plan_code", planCode)
		return nil, fmt.Errorf("pro plan is only available for personal vaults")
	}

	plan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil {
		s.logger.Error("subscription.create failed to get plan", "org_id", orgID, "plan_code", planCode, "error", err)
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	if !plan.IsActive {
		s.logger.Warn("subscription.create inactive plan", "org_id", orgID, "plan_code", planCode)
		return nil, fmt.Errorf("plan is not active")
	}

	now := time.Now()

	sub := &domain.Subscription{
		UUID:                 uuid.New(),
		OrganizationID:       orgID,
		PlanID:               plan.ID,
		State:                domain.SubStateActive,
		StripeSubscriptionID: &stripeSubscriptionID,
		SeatsPurchased:       seatsPurchased,
		StartedAt:            &now,
	}

	// Handle trial period
	var trialEnd *time.Time
	switch {
	case !usePlanTrial:
		if providerTrialEnd != nil && providerTrialEnd.After(now) {
			trialEnd = providerTrialEnd
		}
	case plan.HasTrial():
		end := now.AddDate(0, 0, plan.TrialDays)
		trialEnd = &end
	}
	if trialEnd != nil {
		sub.State = domain.SubStateTrialing
		sub.TrialEndsAt = trialEnd
		sub.RenewAt = trialEnd
	} else {
		// Set renew date based on billing cycle
		var renewAt time.Time
		if plan.BillingCycle == domain.BillingCycleMonthly {
			renewAt = now.AddDate(0, 1, 0)
		} else {
			renewAt = now.AddDate(1, 0, 0)
		}
		sub.RenewAt = &renewAt
	}

	if err := s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.subRepo.LockOrganization(txCtx, orgID); err != nil {
			return fmt.Errorf("lock organization subscription: %w", err)
		}
		// A new provider subscription is a real purchase (existing provider IDs
		// returned above), so it supersedes an administrator grant: the owner
		// is paying and must get what they pay for.
		if stripeSubscriptionID != "" {
			current, currentErr := s.subRepo.GetByOrganizationID(txCtx, orgID)
			if currentErr == nil && isOpenManualGrant(current) {
				s.logger.Info("subscription.create supersedes manual grant",
					"org_id", orgID,
					"manual_subscription_id", current.ID,
				)
			}
		}
		// Keep the effective-subscription invariant atomic: a failed insert
		// must not leave the organization without its previous entitlement.
		if err := s.subRepo.ExpireActiveByOrganizationID(txCtx, orgID, now); err != nil {
			return fmt.Errorf("expire existing active subscriptions: %w", err)
		}
		if err := s.subRepo.Create(txCtx, sub); err != nil {
			return fmt.Errorf("create subscription: %w", err)
		}
		return nil
	}); err != nil {
		s.logger.Error("subscription.create failed to persist", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	s.logger.Info("subscription.create ok",
		"org_id", orgID,
		"subscription_id", sub.ID,
		"state", sub.State,
	)
	return sub, nil
}

func (s *subscriptionService) GrantManual(ctx context.Context, orgID uint, input ManualSubscriptionInput) (*ManualSubscriptionChange, error) {
	if err := validateManualNote(input.Note); err != nil {
		return nil, err
	}
	if err := validateManualEndDate(input.EndsAt, time.Now()); err != nil {
		return nil, err
	}

	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}
	plan, err := s.planRepo.GetByCode(ctx, strings.TrimSpace(input.PlanCode))
	if err != nil || plan == nil {
		return nil, &ManualSubscriptionError{Code: ManualSubscriptionCodePlanNotAllowed, Message: "plan is not available"}
	}
	if !plan.IsActive || !manualPlanAllowed(org.IsPersonal, plan.Code) {
		return nil, &ManualSubscriptionError{Code: ManualSubscriptionCodePlanNotAllowed, Message: "plan is not allowed for this organization"}
	}
	if input.Seats != nil && *input.Seats <= 0 {
		return nil, &ManualSubscriptionError{Code: ManualSubscriptionCodeSeatsBelowMembers, Message: "seats must be greater than zero"}
	}
	if input.Seats != nil && plan.MaxUsers != nil && *input.Seats > *plan.MaxUsers {
		return nil, &ManualSubscriptionError{Code: ManualSubscriptionCodeUsageExceedsPlan, Message: "seats exceed the plan limit"}
	}
	var change *ManualSubscriptionChange
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.subRepo.LockOrganization(txCtx, orgID); err != nil {
			return fmt.Errorf("lock organization subscription: %w", err)
		}
		// Usage is read under the organization lock so a concurrent change
		// cannot slip in between the check and the grant.
		if !plan.IsFree() {
			if err := s.validateManualPaidUsage(txCtx, orgID, plan, input.Seats); err != nil {
				return err
			}
		}

		current, currentErr := s.subRepo.GetByOrganizationID(txCtx, orgID)
		if currentErr != nil && !isNotFound(currentErr) {
			return fmt.Errorf("load current subscription: %w", currentErr)
		}
		if currentErr != nil {
			current = nil
		}
		if blocksManualGrant(current, time.Now()) {
			return &ManualSubscriptionError{
				Code:    ManualSubscriptionCodeExternalActive,
				Message: "an external subscription still has access",
			}
		}

		oldPlan := ""
		var noticeSentAt *time.Time
		if current != nil {
			if current.Plan != nil {
				oldPlan = current.Plan.Code
			}
			// A plan change that keeps the end date must not resend the
			// reminder the owner already received for that date.
			if isOpenManualGrant(current) && current.RenewAt.Equal(input.EndsAt) {
				noticeSentAt = current.ManualEndNoticeSentAt
			}
		}

		now := time.Now()
		if err := s.subRepo.ExpireActiveByOrganizationID(txCtx, orgID, now); err != nil {
			return fmt.Errorf("expire current subscription: %w", err)
		}

		manual := &domain.Subscription{
			UUID:                  uuid.New(),
			OrganizationID:        orgID,
			PlanID:                plan.ID,
			Plan:                  plan,
			State:                 domain.SubStateActive,
			StartedAt:             &now,
			RenewAt:               &input.EndsAt,
			StripeSubscriptionID:  nil,
			SeatsPurchased:        cloneOptionalInt(input.Seats),
			ManualEndNoticeSentAt: noticeSentAt,
		}
		if err := s.subRepo.Create(txCtx, manual); err != nil {
			return fmt.Errorf("create manual subscription: %w", err)
		}
		change = &ManualSubscriptionChange{
			Subscription: manual,
			Organization: org,
			OldPlan:      oldPlan,
			NewPlan:      plan.Code,
			Provider:     domain.PaymentProviderManual,
			EndsAt:       manual.RenewAt,
			Seats:        manual.SeatsPurchased,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

func (s *subscriptionService) ExtendManual(ctx context.Context, orgID uint, endsAt time.Time, note string) (*ManualSubscriptionChange, error) {
	if err := validateManualNote(note); err != nil {
		return nil, err
	}
	if err := validateManualEndDate(endsAt, time.Now()); err != nil {
		return nil, err
	}

	var change *ManualSubscriptionChange
	err := s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.subRepo.LockOrganization(txCtx, orgID); err != nil {
			return fmt.Errorf("lock organization subscription: %w", err)
		}
		org, err := s.orgRepo.GetByID(txCtx, orgID)
		if err != nil {
			return fmt.Errorf("organization not found: %w", err)
		}
		sub, err := s.subRepo.GetByOrganizationID(txCtx, orgID)
		if err != nil {
			if isNotFound(err) {
				return errManualNotActive()
			}
			return fmt.Errorf("load current subscription: %w", err)
		}
		// A grant whose end date just passed is still extendable until the
		// worker finalizes it; the caller computes the new end from now.
		if !isOpenManualGrant(sub) {
			return errManualNotActive()
		}
		planCode := ""
		if sub.Plan != nil {
			planCode = sub.Plan.Code
		}
		sub.RenewAt = &endsAt
		sub.ManualEndNoticeSentAt = nil
		if err := s.subRepo.Update(txCtx, sub); err != nil {
			return fmt.Errorf("extend manual subscription: %w", err)
		}
		change = &ManualSubscriptionChange{
			Subscription: sub,
			Organization: org,
			OldPlan:      planCode,
			NewPlan:      planCode,
			Provider:     domain.PaymentProviderManual,
			EndsAt:       sub.RenewAt,
			Seats:        cloneOptionalInt(sub.SeatsPurchased),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

func (s *subscriptionService) EndManual(ctx context.Context, orgID uint, note string) (*ManualSubscriptionChange, error) {
	if err := validateManualNote(note); err != nil {
		return nil, err
	}

	var change *ManualSubscriptionChange
	err := s.withinTx(ctx, func(txCtx context.Context) error {
		if err := s.subRepo.LockOrganization(txCtx, orgID); err != nil {
			return fmt.Errorf("lock organization subscription: %w", err)
		}
		org, err := s.orgRepo.GetByID(txCtx, orgID)
		if err != nil {
			return fmt.Errorf("organization not found: %w", err)
		}
		sub, err := s.subRepo.GetByOrganizationID(txCtx, orgID)
		if err != nil {
			if isNotFound(err) {
				return errManualNotActive()
			}
			return fmt.Errorf("load current subscription: %w", err)
		}
		if !domain.IsManualSubscription(sub) {
			if blocksManualGrant(sub, time.Now()) {
				return &ManualSubscriptionError{Code: ManualSubscriptionCodeExternalActive, Message: "an external subscription still has access"}
			}
			return errManualNotActive()
		}
		if !isOpenManualGrant(sub) {
			return errManualNotActive()
		}
		if sub.Plan == nil {
			sub.Plan, err = s.planRepo.GetByID(txCtx, sub.PlanID)
			if err != nil {
				return fmt.Errorf("load subscription plan: %w", err)
			}
		}

		now := time.Now()
		oldPlan := sub.Plan.Code
		seats := cloneOptionalInt(sub.SeatsPurchased)
		sub.State = domain.SubStateExpired
		sub.EndedAt = &now
		sub.RenewAt = nil
		sub.SeatsPurchased = nil
		if err := s.subRepo.Update(txCtx, sub); err != nil {
			return fmt.Errorf("end manual subscription: %w", err)
		}

		newPlan := oldPlan
		resultSub := sub
		if org.IsPersonal && !sub.Plan.IsFree() {
			freePlan, err := s.planRepo.GetByCode(txCtx, "free-monthly")
			if err != nil {
				return fmt.Errorf("load free plan: %w", err)
			}
			freeSub := &domain.Subscription{
				UUID:           uuid.New(),
				OrganizationID: orgID,
				PlanID:         freePlan.ID,
				Plan:           freePlan,
				State:          domain.SubStateActive,
				StartedAt:      &now,
			}
			if err := s.subRepo.Create(txCtx, freeSub); err != nil {
				return fmt.Errorf("create free subscription: %w", err)
			}
			newPlan = freePlan.Code
			resultSub = freeSub
		}
		change = &ManualSubscriptionChange{
			Subscription: resultSub,
			Organization: org,
			OldPlan:      oldPlan,
			NewPlan:      newPlan,
			Provider:     domain.PaymentProviderManual,
			EndsAt:       &now,
			Seats:        seats,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

func (s *subscriptionService) validateManualPaidUsage(ctx context.Context, orgID uint, plan *domain.Plan, seats *int) error {
	maxUsers := plan.MaxUsers
	if seats != nil {
		maxUsers = seats
	}
	if maxUsers != nil {
		members, err := s.orgRepo.GetMemberCount(ctx, orgID)
		if err != nil {
			return fmt.Errorf("load member usage: %w", err)
		}
		if members > *maxUsers {
			return &ManualSubscriptionError{Code: ManualSubscriptionCodeSeatsBelowMembers, Message: "seats cannot be below current members"}
		}
	}
	if plan.MaxCollections != nil {
		collections, err := s.orgRepo.GetCollectionCount(ctx, orgID)
		if err != nil {
			return fmt.Errorf("load collection usage: %w", err)
		}
		if collections > *plan.MaxCollections {
			return &ManualSubscriptionError{Code: ManualSubscriptionCodeUsageExceedsPlan, Message: "collections exceed the target plan limit"}
		}
	}
	if plan.MaxItems != nil {
		items, err := s.orgRepo.GetItemCount(ctx, orgID)
		if err != nil {
			return fmt.Errorf("load item usage: %w", err)
		}
		if items > *plan.MaxItems {
			return &ManualSubscriptionError{Code: ManualSubscriptionCodeUsageExceedsPlan, Message: "items exceed the target plan limit"}
		}
	}
	return nil
}

func validateManualNote(note string) error {
	if strings.TrimSpace(note) == "" {
		return &ManualSubscriptionError{Code: ManualSubscriptionCodeNoteRequired, Message: "note is required"}
	}
	return nil
}

func validateManualEndDate(endsAt, now time.Time) error {
	if endsAt.IsZero() || !endsAt.After(now) || endsAt.After(now.AddDate(5, 0, 0)) {
		return &ManualSubscriptionError{Code: ManualSubscriptionCodeInvalidEndDate, Message: "ends_at must be in the future and no more than 5 years away"}
	}
	return nil
}

func manualPlanAllowed(personal bool, planCode string) bool {
	if personal {
		return domain.IsPersonalVaultPlan(planCode)
	}
	base := strings.TrimSuffix(strings.TrimSuffix(planCode, "-monthly"), "-yearly")
	return base == string(domain.PlanFamily) || base == string(domain.PlanTeam) || base == string(domain.PlanBusiness)
}

func blocksManualGrant(sub *domain.Subscription, now time.Time) bool {
	if sub == nil || domain.IsManualSubscription(sub) {
		return false
	}
	switch sub.State {
	case domain.SubStateActive, domain.SubStateTrialing, domain.SubStatePastDue:
		return true
	case domain.SubStateCanceled:
		return sub.RenewAt != nil && sub.RenewAt.After(now)
	default:
		return false
	}
}

// isOpenManualGrant reports a time-limited manual grant that has not been
// finalized yet, including one whose end date passed before the worker ran.
// Catalog Free rows (no end date) are not grants.
func isOpenManualGrant(sub *domain.Subscription) bool {
	return sub != nil &&
		domain.IsManualSubscription(sub) &&
		sub.State == domain.SubStateActive &&
		sub.RenewAt != nil &&
		(sub.Plan == nil || !sub.Plan.IsFree())
}

func errManualNotActive() error {
	return &ManualSubscriptionError{Code: ManualSubscriptionCodeNotActiveManual, Message: "active manual subscription not found"}
}

// isNotFound covers both the repository sentinel and GORM's, since
// subscription lookups surface the latter.
func isNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound)
}

func isActiveManual(sub *domain.Subscription, now time.Time) bool {
	return sub != nil &&
		domain.IsManualSubscription(sub) &&
		sub.State == domain.SubStateActive &&
		sub.RenewAt != nil &&
		sub.RenewAt.After(now)
}

func (s *subscriptionService) activeManualGrantSupersedes(ctx context.Context, providerSub *domain.Subscription) bool {
	if providerSub == nil {
		return false
	}
	current, err := s.subRepo.GetByOrganizationID(ctx, providerSub.OrganizationID)
	return err == nil && current != nil && current.ID != providerSub.ID && isActiveManual(current, time.Now())
}

func cloneOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (s *subscriptionService) UpdateSeatsPurchasedByStripeSubscriptionID(ctx context.Context, stripeSubID string, seatsPurchased *int) error {
	if stripeSubID == "" {
		return fmt.Errorf("stripe subscription id required")
	}
	s.logger.Info("subscription.seats sync",
		"stripe_subscription_id", stripeSubID,
		"seats_set", seatsPurchased != nil,
	)
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		s.logger.Error("subscription.seats sync failed to load", "stripe_subscription_id", stripeSubID, "error", err)
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	if s.activeManualGrantSupersedes(ctx, sub) {
		return nil
	}
	sub.SeatsPurchased = seatsPurchased
	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("subscription.seats sync failed to update", "stripe_subscription_id", stripeSubID, "error", err)
		return fmt.Errorf("failed to update subscription seats: %w", err)
	}
	return nil
}

func (s *subscriptionService) SyncProviderPeriod(
	ctx context.Context,
	stripeSubID string,
	periodEnd time.Time,
	trialEnd *time.Time,
) error {
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	if s.activeManualGrantSupersedes(ctx, sub) {
		return nil
	}
	if !periodEnd.IsZero() {
		sub.RenewAt = &periodEnd
	}
	if trialEnd != nil {
		sub.TrialEndsAt = trialEnd
	}
	return s.subRepo.Update(ctx, sub)
}

// GetByID retrieves a subscription by ID
func (s *subscriptionService) GetByID(ctx context.Context, id uint) (*domain.Subscription, error) {
	return s.subRepo.GetByID(ctx, id)
}

// GetByOrganizationID retrieves the active subscription for an organization
func (s *subscriptionService) GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error) {
	return s.subRepo.GetByOrganizationID(ctx, orgID)
}

// GetByStripeSubscriptionID retrieves a subscription by its Stripe subscription ID
func (s *subscriptionService) GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.Subscription, error) {
	return s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
}

// Update updates a subscription
func (s *subscriptionService) Update(ctx context.Context, sub *domain.Subscription) error {
	if sub != nil {
		current, err := s.subRepo.GetByID(ctx, sub.ID)
		if err == nil && isActiveManual(current, time.Now()) {
			return nil
		}
	}
	return s.subRepo.Update(ctx, sub)
}

// Upgrade upgrades a subscription to a higher plan
func (s *subscriptionService) Upgrade(ctx context.Context, orgID uint, planCode string) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization not found: %w", err)
	}
	if org.IsPersonal && domain.IsMultiUserPlan(planCode) {
		return fmt.Errorf("personal vaults can only use free or pro plans")
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(planCode) && planCode != string(domain.PlanFree) {
		return fmt.Errorf("pro plan is only available for personal vaults")
	}

	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	newPlan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Validate upgrade
	currentPlan, err := s.planRepo.GetByID(ctx, sub.PlanID)
	if err != nil {
		return fmt.Errorf("failed to get current plan: %w", err)
	}

	if newPlan.PriceCents <= currentPlan.PriceCents {
		return fmt.Errorf("new plan must be higher tier than current plan")
	}

	// Update subscription
	sub.PlanID = newPlan.ID

	return s.subRepo.Update(ctx, sub)
}

// Downgrade downgrades a subscription to a lower plan
func (s *subscriptionService) Downgrade(ctx context.Context, orgID uint, planCode string) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("organization not found: %w", err)
	}
	if org.IsPersonal && domain.IsMultiUserPlan(planCode) {
		return fmt.Errorf("personal vaults can only use free or pro plans")
	}
	if !org.IsPersonal && domain.IsPersonalVaultPlan(planCode) && planCode != string(domain.PlanFree) {
		return fmt.Errorf("pro plan is only available for personal vaults")
	}

	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	newPlan, err := s.planRepo.GetByCode(ctx, planCode)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
	}

	// Check if organization usage exceeds new plan limits
	if newPlan.MaxUsers != nil {
		memberCount, err := s.orgService.GetMemberCount(ctx, orgID)
		if err != nil {
			return fmt.Errorf("failed to get member count: %w", err)
		}
		if memberCount > *newPlan.MaxUsers {
			return fmt.Errorf("cannot downgrade: current users (%d) exceed plan limit (%d)", memberCount, *newPlan.MaxUsers)
		}
	}

	if newPlan.MaxCollections != nil {
		collectionCount, err := s.orgService.GetCollectionCount(ctx, orgID)
		if err != nil {
			return fmt.Errorf("failed to get collection count: %w", err)
		}
		if collectionCount > *newPlan.MaxCollections {
			return fmt.Errorf("cannot downgrade: current collections (%d) exceed plan limit (%d)", collectionCount, *newPlan.MaxCollections)
		}
	}

	// Schedule downgrade at end of billing period
	sub.PlanID = newPlan.ID

	return s.subRepo.Update(ctx, sub)
}

// Cancel cancels a subscription at the end of the billing period.
// The cancellation is routed to the correct payment provider (Stripe, RevenueCat, or manual).
func (s *subscriptionService) Cancel(ctx context.Context, orgID uint) error {
	// 1. Get subscription from database
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.State == domain.SubStateExpired || sub.State == domain.SubStateCanceled {
		return fmt.Errorf("subscription is already canceled or expired")
	}

	// 2. Detect payment provider and route accordingly
	provider := domain.DetectPaymentProvider(sub.StripeSubscriptionID)

	switch provider {
	case domain.PaymentProviderRevenueCat:
		// RevenueCat subscriptions are managed by App Store / Play Store.
		// They cannot be canceled from our API; the user must cancel from the store.
		store := domain.DetectStoreFromSubscriptionID(sub.StripeSubscriptionID)
		storeName := domain.StoreDisplayName(store)
		s.logger.Infof("Cancel rejected for RevenueCat subscription (org_id=%d, store=%s, subscription_id=%s)",
			orgID, storeName, *sub.StripeSubscriptionID)
		return fmt.Errorf("this subscription is managed by %s. Please cancel it from the %s directly", storeName, storeName)

	case domain.PaymentProviderStripe:
		// Cancel via Stripe API
		stripeClient, ok := s.stripe.(*stripe.Client)
		if !ok {
			return fmt.Errorf("stripe client not available")
		}

		s.logger.Infof("Canceling Stripe subscription at period end: %s (org_id=%d)",
			*sub.StripeSubscriptionID, orgID)

		stripeSub, err := stripeClient.CancelSubscription(*sub.StripeSubscriptionID, true) // true = cancel at period end
		if err != nil {
			s.logger.Error("Failed to cancel Stripe subscription",
				"stripe_subscription_id", *sub.StripeSubscriptionID,
				"org_id", orgID,
				"error", err)
			return fmt.Errorf("failed to cancel Stripe subscription: %w", err)
		}

		s.logger.Infof("Stripe subscription canceled successfully: %s (status=%s, cancel_at_period_end=true)",
			stripeSub.ID, stripeSub.Status)

	case domain.PaymentProviderManual:
		// Manual/admin-granted subscriptions have no external provider to notify.
		s.logger.Infof("Canceling manual subscription (org_id=%d)", orgID)

	default:
		return fmt.Errorf("unknown payment provider for subscription")
	}

	// 3. Update database (after external provider success, if applicable)
	now := time.Now()
	sub.CancelAt = &now
	sub.State = domain.SubStateCanceled

	s.logger.Infof("Marking subscription as CANCELED at period end (org_id=%d, provider=%s, will_expire_at=%v)",
		orgID, provider, sub.RenewAt)

	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("Failed to update subscription in database after cancel",
			"org_id", orgID,
			"provider", provider,
			"error", err)
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	s.logger.Infof("Subscription canceled successfully in database (org_id=%d, provider=%s)", orgID, provider)
	return nil
}

// Resume resumes a canceled subscription.
// The reactivation is routed to the correct payment provider (Stripe, RevenueCat, or manual).
func (s *subscriptionService) Resume(ctx context.Context, orgID uint) error {
	// 1. Get subscription from database
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	if sub.State != domain.SubStateCanceled {
		return fmt.Errorf("can only resume canceled subscriptions (current state: %s)", sub.State)
	}

	// 2. Detect payment provider and route accordingly
	provider := domain.DetectPaymentProvider(sub.StripeSubscriptionID)

	switch provider {
	case domain.PaymentProviderRevenueCat:
		// RevenueCat subscriptions are managed by App Store / Play Store.
		// They cannot be reactivated from our API; the user must re-subscribe from the store.
		store := domain.DetectStoreFromSubscriptionID(sub.StripeSubscriptionID)
		storeName := domain.StoreDisplayName(store)
		s.logger.Infof("Resume rejected for RevenueCat subscription (org_id=%d, store=%s, subscription_id=%s)",
			orgID, storeName, *sub.StripeSubscriptionID)
		return fmt.Errorf("this subscription is managed by %s. Please resubscribe from the %s directly", storeName, storeName)

	case domain.PaymentProviderStripe:
		// Reactivate via Stripe API
		stripeClient, ok := s.stripe.(*stripe.Client)
		if !ok {
			return fmt.Errorf("stripe client not available")
		}

		s.logger.Infof("Reactivating Stripe subscription: %s (org_id=%d)",
			*sub.StripeSubscriptionID, orgID)

		stripeSub, err := stripeClient.ReactivateSubscription(*sub.StripeSubscriptionID)
		if err != nil {
			s.logger.Error("Failed to reactivate Stripe subscription",
				"stripe_subscription_id", *sub.StripeSubscriptionID,
				"org_id", orgID,
				"error", err)
			return fmt.Errorf("failed to reactivate Stripe subscription: %w", err)
		}

		s.logger.Infof("Stripe subscription reactivated successfully: %s (status=%s)",
			stripeSub.ID, stripeSub.Status)

	case domain.PaymentProviderManual:
		// Manual/admin-granted subscriptions have no external provider to notify.
		s.logger.Infof("Reactivating manual subscription (org_id=%d)", orgID)

	default:
		return fmt.Errorf("unknown payment provider for subscription")
	}

	// 3. Update database (after external provider success, if applicable)
	sub.State = domain.SubStateActive
	sub.CancelAt = nil

	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.logger.Error("Failed to update subscription in database after reactivation",
			"org_id", orgID,
			"provider", provider,
			"error", err)
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	s.logger.Infof("Subscription reactivated successfully in database (org_id=%d, provider=%s)", orgID, provider)
	return nil
}

// Renew renews a subscription
func (s *subscriptionService) Renew(ctx context.Context, subID uint) error {
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Calculate next renewal date
	nextRenew := s.calculateNextRenewal(sub)
	sub.RenewAt = &nextRenew

	return s.subRepo.Update(ctx, sub)
}

// HandlePaymentSuccess handles successful payment webhook
func (s *subscriptionService) HandlePaymentSuccess(ctx context.Context, stripeSubID string) error {
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	if s.activeManualGrantSupersedes(ctx, sub) {
		return nil
	}

	// Transition state based on current state
	now := time.Now()

	switch sub.State {
	case domain.SubStateTrialing:
		// The $0 invoice that opens a provider trial must not end the trial early.
		if sub.TrialEndsAt != nil && sub.TrialEndsAt.After(now) {
			return nil
		}
		sub.State = domain.SubStateActive
		sub.StartedAt = &now

	case domain.SubStateDraft:
		// First payment succeeded - activate subscription
		sub.State = domain.SubStateActive
		sub.StartedAt = &now

	case domain.SubStatePastDue:
		// Payment retry succeeded - restore to active
		sub.State = domain.SubStateActive
		sub.GracePeriodEndsAt = nil

	case domain.SubStateExpired:
		// Reactivation payment succeeded
		sub.State = domain.SubStateActive
		sub.StartedAt = &now
		sub.EndedAt = nil
	}

	// Preserve the provider-synchronized paid-through date when it is still
	// valid. Invoice-only success events may not carry a period end, so fall
	// back to the catalog billing cycle only when no future date is known.
	if sub.RenewAt == nil || !sub.RenewAt.After(now) {
		nextRenew := s.calculateNextRenewal(sub)
		sub.RenewAt = &nextRenew
	}

	return s.subRepo.Update(ctx, sub)
}

// HandlePaymentFailed handles failed payment webhook
func (s *subscriptionService) HandlePaymentFailed(ctx context.Context, stripeSubID string) error {
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	if s.activeManualGrantSupersedes(ctx, sub) {
		return nil
	}

	// Move to past_due state with grace period
	sub.State = domain.SubStatePastDue
	graceDays := 14
	if sub.Plan != nil {
		graceDays = sub.Plan.GraceDays
	}
	gracePeriod := time.Now().AddDate(0, 0, graceDays)
	sub.GracePeriodEndsAt = &gracePeriod

	// Send notification email
	if s.emailService != nil {
		go func() {
			_ = s.emailService.SendPaymentFailedEmail(context.Background(), sub)
		}()
	}

	return s.subRepo.Update(ctx, sub)
}

// ExpireSubscription expires a subscription
func (s *subscriptionService) ExpireSubscription(ctx context.Context, subID uint) error {
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	if sub.State == domain.SubStateExpired {
		return nil
	}
	if sub.Plan == nil {
		sub.Plan, err = s.planRepo.GetByID(ctx, sub.PlanID)
		if err != nil {
			return fmt.Errorf("failed to get subscription plan: %w", err)
		}
	}

	now := time.Now()
	if err := s.withinTx(ctx, func(txCtx context.Context) error {
		sub.State = domain.SubStateExpired
		sub.EndedAt = &now
		if err := s.subRepo.Update(txCtx, sub); err != nil {
			return fmt.Errorf("expire paid subscription: %w", err)
		}

		if sub.Plan.ExpiryBehavior != domain.ExpiryBehaviorDowngradeToFree || sub.Plan.IsFree() {
			return nil
		}
		freePlan, err := s.planRepo.GetByCode(txCtx, "free-monthly")
		if err != nil {
			return fmt.Errorf("load free downgrade plan: %w", err)
		}
		freeSubscription := &domain.Subscription{
			UUID:           uuid.New(),
			OrganizationID: sub.OrganizationID,
			PlanID:         freePlan.ID,
			State:          domain.SubStateActive,
			StartedAt:      &now,
		}
		if err := s.subRepo.Create(txCtx, freeSubscription); err != nil {
			return fmt.Errorf("create free downgrade subscription: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	if s.emailService != nil {
		go func() {
			_ = s.emailService.SendSubscriptionExpiredEmail(context.Background(), sub)
		}()
	}
	return nil
}

// HandleProviderCanceled applies a provider cancellation. endedAt is the
// provider's termination time; when it has passed (immediate cancel, refund,
// dispute) access ends now. Without it, access runs until the paid-through date.
func (s *subscriptionService) HandleProviderCanceled(ctx context.Context, subID uint, endedAt *time.Time) error {
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}
	now := time.Now()
	if endedAt != nil && !endedAt.After(now) {
		return s.ExpireSubscription(ctx, subID)
	}
	accessUntil := sub.RenewAt
	if endedAt != nil {
		accessUntil = endedAt
	}
	if accessUntil != nil && accessUntil.After(now) {
		until := *accessUntil
		sub.State = domain.SubStateCanceled
		sub.RenewAt = &until
		sub.CancelAt = &until
		return s.subRepo.Update(ctx, sub)
	}
	return s.ExpireSubscription(ctx, subID)
}

func (s *subscriptionService) withinTx(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	if s.txManager == nil {
		return fn(ctx)
	}
	return s.txManager.WithinTx(ctx, fn)
}

// CheckExpiredSubscriptions checks and expires subscriptions that should be expired
func (s *subscriptionService) CheckExpiredSubscriptions(ctx context.Context) error {
	now := time.Now()
	batches := []struct {
		name string
		list func(context.Context) ([]*domain.Subscription, error)
	}{
		// past_due subscriptions whose grace period ended
		{"past_due", s.subRepo.ListPastDueExpired},
		// canceled subscriptions whose paid-through date passed
		{"canceled", s.subRepo.ListCanceledExpired},
		// trials follow the plan expiry behavior (Personal → Free, shared → Frozen)
		{"trial", func(ctx context.Context) ([]*domain.Subscription, error) {
			return s.subRepo.ListTrialEnding(ctx, now)
		}},
		// manual grants whose end date passed
		{"manual", s.subRepo.ListManualExpired},
	}

	var failed int
	for _, batch := range batches {
		subs, err := batch.list(ctx)
		if err != nil {
			return fmt.Errorf("failed to list %s subscriptions to expire: %w", batch.name, err)
		}
		for _, sub := range subs {
			if err := s.ExpireSubscription(ctx, sub.ID); err != nil {
				failed++
				if s.logger != nil {
					s.logger.Error("failed to expire subscription",
						"batch", batch.name,
						"subscription_id", sub.ID,
						"organization_id", sub.OrganizationID,
						"error", err,
					)
				}
			}
		}
	}
	if failed > 0 {
		return fmt.Errorf("failed to expire %d subscription(s)", failed)
	}
	return nil
}

func (s *subscriptionService) SendManualExpiryReminders(ctx context.Context) error {
	if s.manualEmailSender == nil || s.manualEmailBuilder == nil {
		return nil
	}
	subscriptions, err := s.subRepo.ListManualEndingSoon(ctx, time.Now().Add(7*24*time.Hour))
	if err != nil {
		return fmt.Errorf("list manual subscription reminders: %w", err)
	}

	var failed int
	for _, sub := range subscriptions {
		if sub == nil || sub.RenewAt == nil || sub.ManualEndNoticeSentAt != nil {
			continue
		}
		// StartedAt is available for manual grants, so suppress a seven-day
		// reminder when the original grant itself was shorter than seven days.
		if sub.StartedAt != nil && sub.RenewAt.Sub(*sub.StartedAt) < 7*24*time.Hour {
			continue
		}
		org := sub.Organization
		if org == nil {
			org, err = s.orgRepo.GetByID(ctx, sub.OrganizationID)
			if err != nil {
				failed++
				continue
			}
		}
		recipient := strings.TrimSpace(org.BillingEmail)
		if recipient == "" {
			recipient = s.organizationOwnerEmail(ctx, org.ID)
		}
		if recipient == "" {
			continue
		}
		planName := "paid"
		if sub.Plan != nil {
			planName = strings.TrimSpace(sub.Plan.Name)
			if planName == "" {
				planName = sub.Plan.Code
			}
		}
		billingURL := fmt.Sprintf("%s/organizations/%s/billing", s.frontendURL, org.PublicID)
		message, err := s.manualEmailBuilder.BuildManualSubscriptionEndingEmail(
			recipient,
			org.Name,
			planName,
			*sub.RenewAt,
			org.IsPersonal,
			billingURL,
		)
		if err != nil {
			failed++
			continue
		}
		if err := s.manualEmailSender.Send(ctx, message); err != nil {
			failed++
			continue
		}
		sentAt := time.Now()
		sub.ManualEndNoticeSentAt = &sentAt
		if err := s.subRepo.Update(ctx, sub); err != nil {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("failed to process %d manual subscription reminder(s)", failed)
	}
	return nil
}

func (s *subscriptionService) organizationOwnerEmail(ctx context.Context, orgID uint) string {
	if s.orgUserRepo == nil {
		return ""
	}
	members, err := s.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		return ""
	}
	for _, member := range members {
		if member != nil && member.Role == domain.OrgRoleOwner && member.User != nil {
			return strings.TrimSpace(member.User.Email)
		}
	}
	return ""
}

// calculateNextRenewal calculates the next renewal date based on billing cycle
func (s *subscriptionService) calculateNextRenewal(sub *domain.Subscription) time.Time {
	if sub.Plan == nil {
		return time.Now().AddDate(0, 1, 0) // Default to 1 month
	}

	now := time.Now()

	switch sub.Plan.BillingCycle {
	case domain.BillingCycleMonthly:
		return now.AddDate(0, 1, 0)
	case domain.BillingCycleYearly:
		return now.AddDate(1, 0, 0)
	default:
		return now.AddDate(0, 1, 0)
	}
}
