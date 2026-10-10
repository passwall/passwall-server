# passwall-server

Passwall backend: REST API, auth, zero-knowledge storage, organizations,
subscriptions and entitlements. Every client (Vault, Extension, Mobile, Desktop)
depends on it, so API changes must stay backward compatible.

## Stack

Go 1.26 (`go.mod`, toolchain go1.26.9), Gin, GORM (PostgreSQL; SQLite only in tests), goose v3
migrations, stripe-go v81, golang-jwt, viper. Module `github.com/passwall/passwall-server`.

## Commands

```bash
make build                                  # build/passwall-server (CGO off, ldflags into pkg/buildvars)
go test ./...                               # quick; `make test` = go test -v -race -cover ./... (race needs CGO)
go vet ./...
PATH=$HOME/go/bin:$PATH golangci-lint run --timeout 15m ./...   # CI pins golangci-lint v2.4.0
go mod tidy && git diff --exit-code go.mod go.sum                 # CI fails on an untidy module
```

Before calling work done: `go vet ./...`, `go test -race ./...`, golangci-lint
with 0 issues, gofmt clean. Run locally against the shared stack with the
`passwall-local-dev` skill (the harness builds and runs this repo).

Release: `make pre-release` (bumps MINOR in `VERSION`, commits, pushes) then
`make release` (tags `v$(cat VERSION)`); the tag CI publishes
`passwall/passwall-server:X.Y.Z`.

## Architecture

- `cmd/passwall-server/main.go`: `passwall-server` runs the API;
  `passwall-server migrate` (or `migrate-and-seed`) runs migrations and exits.
- `internal/core/app.go`: manual constructor DI (repositories → services →
  handlers → `SetupRouter` in `router.go`) and background workers.
- Layers: `internal/domain` (models, DTOs) → `internal/repository` (interfaces,
  `ErrNotFound`) + `gormrepo/` (GORM impls, `TxManager`, `dbFromContext` for
  transactions) → `internal/service` (business logic) → `internal/handler/http`
  (Gin handlers, middleware). Also `internal/authz`, `internal/cleanup`
  (workers), `internal/email`, `internal/config`, `pkg/*`.
- Routing: `/auth/*` public; `/api` behind `AuthMiddleware` (JWT); admin under
  `RequireAdminMiddleware`; manual subscription mutations additionally under
  `RequireSystemAdminMiddleware` (re-reads `is_admin && is_system_user` from the
  DB). Org routes resolve the 12-char `public_id` via `OrgPublicIDResolverMiddleware`.
- Entitlements are not middleware: services call
  `OrganizationEntitlementService.Authorize`; handlers map errors with
  `respondEntitlementError`. Access states `paid`, `free`, `free_over_quota`,
  `frozen`; `ENTITLEMENT_ENFORCEMENT_MODES` env (`cap=mode,*=mode`) controls
  off/log/enforce per capability.

## Domain essentials

- Organization-only vaults: every user has a personal org
  (`users.personal_organization_id`, `organizations.is_personal`). Personal orgs
  take Free/Pro (`IsPersonalVaultPlan`), shared orgs Family/Team/Business.
- Zero-knowledge: the server stores `bcrypt(HKDF(masterKey,"auth"))`, the
  protected user key, KDF config, RSA public key and encrypted private key,
  per-member encrypted org keys, and org-key-encrypted item data. Never add code
  that needs plaintext, and never log keys, tokens or item data.
- Payment provider is inferred from `stripe_subscription_id`: NULL/blank =
  manual admin grant, `rc_` prefix = RevenueCat, otherwise Stripe.

## Conventions

- Errors: wrap with `fmt.Errorf("...: %w", err)`; handlers return
  `gin.H{"error": msg}` plus a stable `"code"` for anything clients branch on
  (entitlement 403 codes, manual subscription codes in `subscription_service.go`).
- Audit: `service.ActivityLogger` with `ActivityField*` keys → `user_activities`;
  new activity types go in `domain/user_activity.go`.
- Logging through the `service.Logger` interface (key/value pairs).
- Tests: stubs embed the repository interface and override only what they use
  (an unexpected call panics, which catches N+1 regressions); gormrepo tests use
  in-memory SQLite; testify `require`/`assert`.
- Migrations: add `internal/core/migrations/0000N_name.sql` (goose, embedded) and
  bump the expected version in `versioned_migrations_integration_test.go`
  (testcontainers Postgres; skipped without Docker). Production runs migrations
  as a separate step before the new server starts; the server never auto-migrates
  there.
- Avoid N+1 in list endpoints: batch with `IN ?` queries (see
  `GetEffectiveByOrganizationIDs`, `GetCountsByIDs`).

## Gotchas

- `subscriptionRepository.GetByOrganizationID` returns `gorm.ErrRecordNotFound`,
  not `repository.ErrNotFound`; check both.
- `Plan.IsFree()` is `PriceCents == 0`; test plans need a price to count as paid.
- Effective subscription = newest active/trialing/past_due row (skipping lapsed
  manual grants), else a canceled row still paid through, else the newest row.
- GORM drops a `clause.OrderBy{Expression}` when a later `.Order(string)` is
  merged; put all ordering in one expression.
- Config: `config.yml` in the working dir plus `PW_*` env vars; all `config.yml`
  files are gitignored. Plans are seeded from `stripe.plans` in config.
- Stale docs: `README.md`, `CHANGELOG.md`, and the migration section of
  `PROJECT_CONTEXT.md` (the rest of it is a useful API/encryption reference).
  `pkg/database/migrations/` and root `migrations/` are unused legacy.
