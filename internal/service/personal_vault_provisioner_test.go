package service

import (
	"context"
	"errors"
	"testing"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type provisionState struct {
	users         []*domain.User
	orgs          []*domain.Organization
	memberships   []*domain.OrganizationUser
	collections   []*domain.Collection
	folders       []*domain.OrganizationFolder
	subscriptions []*domain.Subscription
}

type provisionStateKey struct{}

type fakeProvisionTxManager struct {
	committed *provisionState
}

func (m *fakeProvisionTxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	next := &provisionState{}
	if err := fn(context.WithValue(ctx, provisionStateKey{}, next)); err != nil {
		return err
	}
	m.committed = next
	return nil
}

type fakeProvisionRepositories struct {
	failAt string
}

func (r *fakeProvisionRepositories) state(ctx context.Context) *provisionState {
	return ctx.Value(provisionStateKey{}).(*provisionState)
}

func (r *fakeProvisionRepositories) fail(stage string) error {
	if r.failAt == stage {
		return errors.New("injected " + stage + " failure")
	}
	return nil
}

func (r *fakeProvisionRepositories) Create(ctx context.Context, value any) error {
	panic("typed Create method required")
}

type fakeUserCreator struct{ *fakeProvisionRepositories }

func (r fakeUserCreator) Create(ctx context.Context, user *domain.User) error {
	if err := r.fail("user"); err != nil {
		return err
	}
	user.ID = 1
	r.state(ctx).users = append(r.state(ctx).users, user)
	return nil
}

type fakeOrgWriter struct{ *fakeProvisionRepositories }

func (r fakeOrgWriter) Create(ctx context.Context, org *domain.Organization) error {
	if err := r.fail("organization"); err != nil {
		return err
	}
	org.ID = 1
	r.state(ctx).orgs = append(r.state(ctx).orgs, org)
	return nil
}
func (r fakeOrgWriter) Update(ctx context.Context, org *domain.Organization) error {
	return r.fail("organization-update")
}

type fakeMembershipCreator struct{ *fakeProvisionRepositories }

func (r fakeMembershipCreator) Create(ctx context.Context, membership *domain.OrganizationUser) error {
	if err := r.fail("membership"); err != nil {
		return err
	}
	r.state(ctx).memberships = append(r.state(ctx).memberships, membership)
	return nil
}

type fakeCollectionCreator struct{ *fakeProvisionRepositories }

func (r fakeCollectionCreator) Create(ctx context.Context, collection *domain.Collection) error {
	if err := r.fail("collection"); err != nil {
		return err
	}
	r.state(ctx).collections = append(r.state(ctx).collections, collection)
	return nil
}

type fakeFolderCreator struct{ *fakeProvisionRepositories }

func (r fakeFolderCreator) Create(ctx context.Context, folder *domain.OrganizationFolder) error {
	if err := r.fail("folder"); err != nil {
		return err
	}
	r.state(ctx).folders = append(r.state(ctx).folders, folder)
	return nil
}

type fakePlanRepository struct{ *fakeProvisionRepositories }

func (r fakePlanRepository) GetByCode(context.Context, string) (*domain.Plan, error) {
	if err := r.fail("plan"); err != nil {
		return nil, err
	}
	return &domain.Plan{ID: 7, Code: freeMonthlyPlanCode}, nil
}

type fakeSubscriptionCreator struct{ *fakeProvisionRepositories }

func (r fakeSubscriptionCreator) Create(ctx context.Context, subscription *domain.Subscription) error {
	if err := r.fail("subscription"); err != nil {
		return err
	}
	r.state(ctx).subscriptions = append(r.state(ctx).subscriptions, subscription)
	return nil
}

func newTestProvisioner(tx *fakeProvisionTxManager, repos *fakeProvisionRepositories) PersonalVaultProvisioner {
	return NewPersonalVaultProvisioner(
		tx,
		fakeUserCreator{repos},
		fakeOrgWriter{repos},
		fakeMembershipCreator{repos},
		fakeCollectionCreator{repos},
		fakeFolderCreator{repos},
		fakeSubscriptionCreator{repos},
		fakePlanRepository{repos},
	)
}

func TestPersonalVaultProvisionerRollsBackEveryStep(t *testing.T) {
	t.Parallel()
	stages := []string{"organization", "user", "membership", "organization-update", "collection", "folder", "plan", "subscription"}
	for _, stage := range stages {
		stage := stage
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			tx := &fakeProvisionTxManager{}
			err := newTestProvisioner(tx, &fakeProvisionRepositories{failAt: stage}).Provision(
				context.Background(),
				&domain.User{Name: "Ada", Email: "ada@example.com"},
				"encrypted-org-key",
			)
			require.Error(t, err)
			assert.Nil(t, tx.committed)
		})
	}
}

func TestPersonalVaultProvisionerCreatesCompleteAggregate(t *testing.T) {
	t.Parallel()
	tx := &fakeProvisionTxManager{}
	user := &domain.User{Name: "Ada", Email: "ada@example.com"}
	require.NoError(t, newTestProvisioner(tx, &fakeProvisionRepositories{}).Provision(
		context.Background(),
		user,
		"encrypted-org-key",
	))

	require.NotNil(t, tx.committed)
	assert.Len(t, tx.committed.users, 1)
	assert.Len(t, tx.committed.orgs, 1)
	assert.Len(t, tx.committed.memberships, 1)
	require.Len(t, tx.committed.collections, 1)
	assert.Equal(t, "General", tx.committed.collections[0].Name)
	assert.True(t, tx.committed.collections[0].IsDefault)
	assert.Len(t, tx.committed.folders, len(constants.DefaultPersonalVaultFolders))
	require.Len(t, tx.committed.subscriptions, 1)
	assert.Equal(t, domain.SubStateActive, tx.committed.subscriptions[0].State)
	assert.Equal(t, uint(1), user.PersonalOrganizationID)
	assert.Equal(t, uint(1), user.DefaultOrganizationID)
}
