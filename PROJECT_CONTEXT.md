# PassWall Server - Project Context

**Version:** 2.0  
**Last Updated:** January 2026  
**Status:** Production Ready  
**Go Version:** 1.24+

---

## 📋 Table of Contents

1. [Project Overview](#-project-overview)
2. [Technology Stack](#-technology-stack)
3. [Project Structure](#-project-structure)
4. [Architecture Overview](#-architecture-overview)
5. [Domain Models](#-domain-models)
6. [Database Strategy](#-database-strategy)
7. [Security & Encryption](#-security--encryption)
8. [SaaS Architecture](#-saas-architecture)
9. [API Structure](#-api-structure)
10. [Development Workflow](#-development-workflow)
11. [Key Files & Locations](#-key-files--locations)
12. [Best Practices](#-best-practices)
13. [Common Tasks](#-common-tasks)

---

## 🎯 Project Overview

**PassWall Server** is the core backend for the open-source password manager PassWall platform. It provides a production-grade, zero-knowledge password management solution with enterprise SaaS features.

### Key Features

- 🔐 **Zero-Knowledge Encryption** - AES-256-CBC + HMAC-SHA256
- 🏢 **Multi-Tenant SaaS** - Organization-based subscriptions with RBAC
- 🔄 **Multi-Device Sync** - Revision-based delta sync
- 📦 **Flexible Storage** - Passwords, credit cards, bank accounts, notes, emails
- 💳 **Stripe Integration** - Subscription management with webhooks
- 🛡️ **Security-First** - Modern encryption, KDF configuration, soft deletes
- 🐳 **Docker Support** - Easy deployment with Docker Compose

### Core Principles

1. **Zero Data Loss** - Soft deletes with retention periods
2. **Security First** - Zero-knowledge architecture, never store plaintext
3. **Churn Prevention** - Grace periods, read-only fallback instead of lockout
4. **Enterprise Ready** - Org-scoped billing, RBAC, audit logging
5. **Developer Friendly** - Clean architecture, type safety, comprehensive docs

---

## 🛠 Technology Stack

### Backend Core

- **Language:** Go 1.24+
- **Framework:** Gin (Web framework)
- **ORM:** GORM v1.31+
- **Database:** PostgreSQL 13+ (Primary), SQLite (Development)

### Key Dependencies

```go
github.com/gin-gonic/gin v1.10.0              // Web framework
gorm.io/gorm v1.31.1                          // ORM
gorm.io/driver/postgres v1.6.0                // PostgreSQL driver
github.com/golang-jwt/jwt/v4 v4.5.2           // JWT authentication
golang.org/x/crypto v0.46.0                   // Cryptography
github.com/spf13/viper v1.21.0                // Configuration
github.com/sirupsen/logrus v1.9.3             // Logging
github.com/aws/aws-sdk-go-v2/service/sesv2   // Email (SES)
github.com/stripe/stripe-go/v81              // Payment (Stripe)
```

### Infrastructure

- **Container:** Docker + Docker Compose
- **Database:** PostgreSQL with automatic migrations
- **Email:** Gmail API, AWS SES, SMTP
- **Payment:** Stripe subscriptions + webhooks
- **Backup:** Automated backup rotation

---

## 📁 Project Structure

```
passwall-server/
├── cmd/
│   └── passwall-server/
│       └── main.go                    # Application entry point
├── internal/
│   ├── core/
│   │   ├── app.go                     # Application initialization
│   │   ├── database.go                # Database setup & AutoMigrate
│   │   ├── router.go                  # Route definitions
│   │   └── seeding.go                 # Database seeding (idempotent)
│   ├── domain/                        # Domain models (entities)
│   │   ├── user.go                    # User entity
│   │   ├── organization.go            # Organization entity
│   │   ├── subscription.go            # Subscription state machine
│   │   ├── plan.go                    # Subscription plans
│   │   ├── item.go                    # Password vault items
│   │   ├── collection.go              # Item collections
│   │   ├── folder.go                  # Folder organization
│   │   └── ...
│   ├── handler/http/                  # HTTP request handlers
│   │   ├── auth.go                    # Authentication endpoints
│   │   ├── user.go                    # User management
│   │   ├── organization.go            # Organization CRUD
│   │   ├── item.go                    # Item (password) CRUD
│   │   ├── payment.go                 # Stripe payment handling
│   │   ├── webhook.go                 # Stripe webhooks
│   │   └── middleware.go              # Middleware (auth, RBAC)
│   ├── service/                       # Business logic layer
│   │   ├── auth.go                    # Authentication logic
│   │   ├── user.go                    # User business logic
│   │   ├── organization_service.go    # Organization logic
│   │   ├── subscription_service.go    # Subscription lifecycle
│   │   ├── permission_service.go      # RBAC permission checks
│   │   ├── feature_service.go         # Feature gating
│   │   └── ...
│   ├── repository/gormrepo/           # Data access layer
│   │   ├── user.go                    # User repository
│   │   ├── organization.go            # Organization repository
│   │   ├── subscription.go            # Subscription repository
│   │   ├── item.go                    # Item repository
│   │   ├── seed.go                    # Role/permission seeding
│   │   └── seed_plans.go              # Plan seeding
│   └── cleanup/                       # Background workers
│       ├── subscription_worker.go     # Expire subscriptions
│       ├── organization_deletion_worker.go
│       └── token_cleanup.go
├── pkg/                               # Reusable packages
│   ├── logger/                        # Logging utilities
│   ├── crypto/                        # Encryption/decryption
│   ├── token/                         # JWT token management
│   ├── database/                      # Database interfaces
│   └── constants/                     # App constants
├── migrations/                        # SQL migrations (production only)
│   └── 010_saas_refactor.sql
├── docs/                              # Architecture documentation
│   ├── ARCHITECTURE_INDEX.md
│   ├── MODERN_ENCRYPTION_ARCHITECTURE.md
│   ├── architecture/
│   │   └── passwall_saas_core_spec.md
│   └── ...
├── build/docker/                      # Docker configuration
│   ├── Dockerfile
│   └── docker-compose.yml
├── config.yml                         # Configuration file
├── Makefile                           # Build/dev commands
└── go.mod                             # Go dependencies
```

---

## 🏗 Architecture Overview

### Clean Architecture Pattern

```
┌─────────────────────────────────────────────────────────────┐
│                     HTTP/API Layer                          │
│  (internal/handler/http) - Gin handlers, middleware         │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Service Layer                            │
│  (internal/service) - Business logic, orchestration         │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                   Repository Layer                          │
│  (internal/repository/gormrepo) - Data access, GORM         │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Database Layer                           │
│  PostgreSQL - Tables, indexes, constraints                  │
└─────────────────────────────────────────────────────────────┘
```

### Request Flow

1. **Client** → HTTP Request
2. **Middleware** → Auth check, RBAC validation
3. **Handler** → Parse request, validate input
4. **Service** → Business logic, feature gates
5. **Repository** → Database queries
6. **Response** → JSON response

### Key Architectural Patterns

- **Domain-Driven Design:** Rich domain models in `internal/domain/`
- **Dependency Injection:** Services receive dependencies via constructors
- **Interface-Based:** Repositories use interfaces for testability
- **Middleware Chain:** Authentication → RBAC → Rate limiting
- **State Machine:** Subscription lifecycle management
- **Background Workers:** Cleanup jobs run in separate goroutines

---

## 🎨 Domain Models

### Core Entities

#### User

```go
type User struct {
    ID        uint
    UUID      uuid.UUID
    Email     string         // Unique email
    Name      string
    
    // Modern Zero-Knowledge Encryption
    MasterPasswordHash string  // bcrypt(HKDF(masterKey, "auth"))
    ProtectedUserKey   string  // EncString: "2.iv|ct|mac"
    
    // KDF Configuration (per user)
    KdfType        KdfType    // PBKDF2 or Argon2id
    KdfIterations  int        // Default: 600K
    KdfSalt        string     // Random per user
    
    // RSA Keys for Organization Sharing
    RSAPublicKey     *string
    RSAPrivateKeyEnc *string
    
    RoleID       uint
    IsVerified   bool
    IsSystemUser bool
}
```

#### Organization

```go
type Organization struct {
    ID          uint
    Name        string
    Status      OrgStatus  // active, suspended, deleted
    PlanType    string     // Deprecated - use Subscription
    CreatedBy   uint
    
    // Soft Delete
    DeletedAt         *time.Time
    ScheduledDeletion *time.Time  // Retention period
    
    // Relationships
    Subscription   *Subscription
    Members        []OrganizationUser
    Collections    []Collection
}
```

#### Subscription (State Machine)

```go
type Subscription struct {
    ID             uint
    OrganizationID uint
    PlanID         uint
    
    // State Machine
    State          SubState  // draft, trialing, active, past_due, canceled, expired
    
    // Timing
    StartedAt      *time.Time
    TrialEndsAt    *time.Time
    CurrentPeriodStart *time.Time
    CurrentPeriodEnd   *time.Time
    CancelAt       *time.Time
    CanceledAt     *time.Time
    EndedAt        *time.Time
    
    // Stripe Integration
    StripeCustomerID      *string
    StripeSubscriptionID  *string
    
    // Relationships
    Plan         *Plan
    Organization *Organization
}

// State Transitions:
// DRAFT → TRIALING → ACTIVE ⟷ PAST_DUE → EXPIRED
//           ↓           ↓
//       CANCELED → EXPIRED
```

#### Plan

```go
type Plan struct {
    ID           uint
    Code         string  // free-monthly, premium-yearly, etc.
    Name         string
    BillingCycle string  // monthly, yearly
    PriceCents   int
    Currency     string
    TrialDays    int
    
    // Limits
    MaxUsers       *int  // nil = unlimited
    MaxCollections *int
    MaxItems       *int
    
    // Features (JSON)
    Features PlanFeatures
    
    // Stripe Integration
    StripePriceID   string
    StripeProductID string
}

type PlanFeatures struct {
    Sharing         bool
    Teams           bool
    Audit           bool
    SSO             bool
    APIAccess       bool
    PrioritySupport bool
}
```

#### Item (Vault Entry)

```go
type Item struct {
    ID        uint
    UUID      uuid.UUID
    
    // Ownership
    OwnerType string  // "user" or "organization"
    OwnerID   uint
    
    // Organization
    CollectionID   *uint
    FolderID       *uint
    
    // Encryption
    EncryptedData  string  // AES-256-CBC + HMAC
    
    // Metadata (searchable, not sensitive)
    Metadata       ItemMetadata  // JSON
    
    // Lifecycle
    CreatedBy uint
    UpdatedBy uint
    DeletedAt *time.Time
}
```

---

## 💾 Database Strategy

### Philosophy

> **Development:** GORM AutoMigrate creates all tables automatically  
> **Production:** SQL migration files for updating existing databases

### How It Works

#### Fresh Database (Development)

```bash
# Just start the server
make run
```

**What happens:**
1. GORM AutoMigrate creates ALL tables in final structure
2. Seeding runs automatically (idempotent):
   - Roles & permissions
   - 9 subscription plans
   - Default free subscriptions

#### Existing Database (Production)

```bash
# Backup first!
pg_dump passwall > backup.sql

# Run migration
psql passwall < migrations/010_saas_refactor.sql

# Start server
./passwall-server
```

### Key Features

- ✅ **Idempotent Seeding** - Safe to run multiple times
- ✅ **Transaction Safety** - All seeding wrapped in transactions
- ✅ **Non-Critical Failures** - Seeding errors don't stop startup
- ✅ **AutoMigrate** - Schema changes applied automatically in dev

### Migration Files

Located in `/migrations/` - **ONLY for production updates**

- Never run on fresh databases
- Historical record of schema changes
- Reviewed before running on production

---

## 🔐 Security & Encryption

### Zero-Knowledge Architecture

**Core Principle:** Server never has access to plaintext passwords

```
User Password (client-side only)
    ↓ PBKDF2(600K iterations) + Random Salt
Master Key (256-bit, client-side)
    ↓ HKDF(info="auth")
Auth Key → bcrypt → Server (authentication)
    ↓ HKDF(info="enc" + "mac")
Stretched Key → Wraps User Key
    ↓
User Key (512-bit)
    ↓ AES-256-CBC + HMAC-SHA256
Encrypted Items → Server Storage
```

### Encryption Flow

#### Signup

1. Client generates random salt (32 bytes)
2. Derive Master Key: `PBKDF2(password, salt, 600000)`
3. Derive Auth Key: `HKDF(masterKey, info="auth")`
4. Send `bcrypt(authKey)` to server
5. Generate User Key (512-bit random)
6. Encrypt User Key with Master Key
7. Send encrypted User Key to server

#### Signin

1. Client retrieves KDF config from server
2. Derive Master Key locally
3. Derive Auth Key and send to server
4. Server validates with bcrypt
5. Client receives encrypted User Key
6. Client decrypts User Key with Master Key
7. Use User Key to decrypt vault items

#### Encryption Format

All encrypted data uses **EncString** format:

```
"2.iv|ciphertext|mac"
```

- **2** = Version (AES-256-CBC + HMAC-SHA256)
- **iv** = Base64-encoded IV (16 bytes)
- **ciphertext** = Base64-encoded encrypted data
- **mac** = Base64-encoded HMAC (32 bytes)

### Security Best Practices

- ✅ Master Key never leaves client
- ✅ Server never stores plaintext passwords
- ✅ Each user has unique random KDF salt
- ✅ Encrypt-then-MAC pattern
- ✅ HTTPS only in production
- ✅ JWT with short expiry (30min access + 7d refresh)
- ✅ Rate limiting on authentication endpoints
- ✅ SQL injection prevention (GORM parameterized queries)

---

## 🏢 SaaS Architecture

### Subscription State Machine

```
DRAFT → TRIALING → ACTIVE ⟷ PAST_DUE → EXPIRED
           ↓           ↓
       CANCELED → EXPIRED
```

#### State Definitions

| State | Description | Access Level |
|-------|-------------|--------------|
| **DRAFT** | Created but not paid | None |
| **TRIALING** | In trial period | Full |
| **ACTIVE** | Paid and active | Full |
| **PAST_DUE** | Payment failed, grace period | Full |
| **CANCELED** | User canceled, active until period end | Full |
| **EXPIRED** | No valid payment | Read-only |

### RBAC Permission Matrix

#### Roles (per Organization)

- **OWNER** - Full control including billing & deletion
- **ADMIN** - Manage members, collections, items (no billing)
- **MANAGER** - Manage collections and items
- **MEMBER** - CRUD own items only
- **BILLING** - View/manage billing only
- **READ_ONLY** - View only (runtime override for expired subs)

#### Permission Groups

```go
// Organization
org:view, org:update, org:delete, org:transfer_ownership

// Members
member:view, member:invite, member:remove, member:update_role

// Billing
billing:view, billing:update, billing:cancel

// Collections
collection:create, collection:view, collection:update, collection:delete

// Items
item:create, item:view, item:update, item:delete, item:share, item:export

// Security
audit:view, security:rotate_keys, security:revoke_sessions
```

### Runtime Permission Override

**Critical:** When subscription expires, effective role becomes READ_ONLY

```go
func GetEffectiveRole(membership Membership, subscription Subscription) Role {
    if subscription.State != ACTIVE {
        return READ_ONLY
    }
    return membership.Role
}
```

- ✅ **Never mutate stored roles** on subscription changes
- ✅ **Compute at runtime** for each request
- ✅ **Preserves data** for recovery after renewal

### Subscription Plans

9 plans seeded automatically:

| Code | Name | Cycle | Price | Users | Features |
|------|------|-------|-------|-------|----------|
| `free-monthly` | Free | monthly | $0 | 1 | Basic |
| `premium-monthly` | Premium | monthly | $2.99 | 1 | API |
| `premium-yearly` | Premium | yearly | $29.90 | 1 | API |
| `family-monthly` | Family | monthly | $5.99 | 6 | Sharing, API |
| `family-yearly` | Family | yearly | $59.90 | 6 | Sharing, API |
| `team-monthly` | Team | monthly | $9.99 | 10 | Teams, Priority |
| `team-yearly` | Team | yearly | $99.90 | 10 | Teams, Priority |
| `business-monthly` | Business | monthly | $19.99 | ∞ | All features |
| `business-yearly` | Business | yearly | $199.90 | ∞ | All features |

### Stripe Integration

#### Webhooks Handled

- `customer.subscription.created` - New subscription
- `customer.subscription.updated` - Plan change, renewal
- `customer.subscription.deleted` - Cancellation
- `invoice.payment_succeeded` - Successful payment
- `invoice.payment_failed` - Failed payment (→ PAST_DUE)
- `invoice.finalized` - Invoice ready

#### Webhook Security

- ✅ Signature verification with webhook secret
- ✅ Idempotency protection (webhook_events table)
- ✅ Duplicate event detection
- ✅ Transaction-safe processing

### Feature Gating

```go
// Check before write operations
can, err := featureService.CanInviteUser(ctx, orgID)
can, err := featureService.CanCreateCollection(ctx, orgID)
can, err := featureService.CanCreateItem(ctx, orgID)
can, err := featureService.CanUseTeams(ctx, orgID)
```

**Enforcement points:**
- Plan limits (max users, collections, items)
- Feature availability (teams, audit, SSO)
- Subscription state (expired = no writes)

---

## 🌐 API Structure

### Base URL

```
http://localhost:3625/api
```

### Authentication

**JWT Bearer Token** in Authorization header:

```
Authorization: Bearer <access_token>
```

### Key Endpoint Groups

#### Authentication

```
POST   /auth/signup          # Create new user
POST   /auth/signin          # Login
POST   /auth/refresh         # Refresh access token
POST   /auth/signout         # Logout
GET    /auth/check           # Verify token
```

#### Users

```
GET    /users                # List users (admin)
GET    /users/:id            # Get user
PUT    /users/:id            # Update user
DELETE /users/:id            # Delete user (soft)
```

#### Organizations

```
GET    /organizations                    # List user's orgs
POST   /organizations                    # Create org
GET    /organizations/:id                # Get org details
PUT    /organizations/:id                # Update org
DELETE /organizations/:id                # Delete org (soft)

# Members
GET    /organizations/:id/members        # List members
POST   /organizations/:id/members/invite # Invite user
DELETE /organizations/:id/members/:uid   # Remove member
PUT    /organizations/:id/members/:uid   # Update role
```

#### Subscriptions

```
GET    /organizations/:id/subscription            # Get subscription
POST   /organizations/:id/subscription            # Create subscription
PUT    /organizations/:id/subscription/upgrade    # Upgrade plan
PUT    /organizations/:id/subscription/downgrade  # Downgrade plan
POST   /organizations/:id/subscription/cancel     # Cancel subscription
POST   /organizations/:id/subscription/resume     # Resume canceled
```

#### Plans

```
GET    /plans          # List available plans
GET    /plans/:code    # Get plan details
```

#### Items (Vault)

```
GET    /items                    # List items
POST   /items                    # Create item
GET    /items/:id                # Get item
PUT    /items/:id                # Update item
DELETE /items/:id                # Delete item (soft)
POST   /items/:id/share          # Share item
```

#### Collections

```
GET    /collections              # List collections
POST   /collections              # Create collection
GET    /collections/:id          # Get collection
PUT    /collections/:id          # Update collection
DELETE /collections/:id          # Delete collection
```

#### Webhooks

```
POST   /webhooks/stripe          # Stripe webhook endpoint (no auth)
```

### Response Format

**Success:**

```json
{
  "data": { ... },
  "message": "Success"
}
```

**Error:**

```json
{
  "error": "Error message",
  "code": "ERROR_CODE"
}
```

---

## 💻 Development Workflow

### Initial Setup

```bash
# 1. Clone repository
git clone https://github.com/passwall/passwall-server.git
cd passwall-server

# 2. Install dependencies
go mod download

# 3. Install dev tools
make install-tools

# 4. Start PostgreSQL
make db-up

# 5. Run server
make run
```

### Development with Hot Reload

```bash
# Uses Air for automatic reloading
make dev
```

### Docker Compose (Recommended)

```bash
# Start all services (PostgreSQL + Server)
make up

# View logs
make logs

# Stop services
make down

# Restart services
make restart
```

### Testing

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Open coverage report
open coverage.html
```

### Linting

```bash
# Run linter
make lint
```

### Building

```bash
# Build for current platform
make build

# Build for Linux
make build-linux

# Build for macOS (Intel + ARM)
make build-darwin

# Build for all platforms
make build-all
```

### Database Operations

```bash
# Start PostgreSQL only
make db-up

# Stop PostgreSQL
make db-down

# View PostgreSQL logs
make db-logs

# Reset database (deletes all data!)
make db-reset
```

### Docker Image

```bash
# Build Docker image
make image-build

# Build and publish to Docker Hub
make image-publish
```

---

## 📌 Key Files & Locations

### Configuration

| File | Purpose |
|------|---------|
| `config.yml` | Main configuration file |
| `env.example` | Environment variable template |
| `go.mod` | Go dependencies |
| `Makefile` | Build and dev commands |

### Entry Points

| File | Purpose |
|------|---------|
| `cmd/passwall-server/main.go` | Application entry point |
| `internal/core/app.go` | App initialization |
| `internal/core/router.go` | Route definitions |

### Core Logic

| Directory | Purpose |
|-----------|---------|
| `internal/domain/` | Domain models (entities) |
| `internal/service/` | Business logic |
| `internal/repository/gormrepo/` | Data access layer |
| `internal/handler/http/` | HTTP handlers |
| `internal/cleanup/` | Background workers |

### Infrastructure

| Directory | Purpose |
|-----------|---------|
| `pkg/crypto/` | Encryption utilities |
| `pkg/token/` | JWT management |
| `pkg/logger/` | Logging |
| `pkg/database/` | Database interfaces |
| `pkg/stripe/` | Stripe integration |

### Documentation

| File | Purpose |
|------|---------|
| `README.md` | Project overview |
| `DATABASE_STRATEGY.md` | Database migration strategy |
| `SAAS_REFACTOR_IMPLEMENTATION_SUMMARY.md` | SaaS architecture details |
| `docs/ARCHITECTURE_INDEX.md` | Architecture documentation index |
| `docs/architecture/passwall_saas_core_spec.md` | Core SaaS specification |

---

## ✅ Best Practices

### Code Style

1. **Follow Go conventions** - Use `gofmt`, follow Go style guide
2. **Meaningful names** - Clear, descriptive variable/function names
3. **Error handling** - Always check and handle errors properly
4. **Comments** - Document complex logic, use JSDoc for functions
5. **DRY principle** - Don't Repeat Yourself

### Security

1. **Never log sensitive data** - Passwords, tokens, keys
2. **Always validate input** - Use binding tags, validate in service layer
3. **Use prepared statements** - GORM handles this automatically
4. **Encrypt at rest** - All sensitive data encrypted before storage
5. **Rate limiting** - Apply to authentication endpoints
6. **HTTPS only** - In production, redirect HTTP to HTTPS

### Database

1. **Use soft deletes** - Preserve data with `DeletedAt` field
2. **Use transactions** - Wrap multi-step operations
3. **Index strategically** - Add indexes for frequently queried fields
4. **Avoid N+1 queries** - Use `Preload()` for associations
5. **Paginate results** - Don't load thousands of records at once

### API Design

1. **RESTful conventions** - Use proper HTTP methods and status codes
2. **Consistent responses** - Uniform JSON structure
3. **Version your APIs** - Plan for breaking changes
4. **Document endpoints** - Keep API docs up-to-date
5. **Handle errors gracefully** - Return meaningful error messages

### Testing

1. **Test business logic** - Focus on service layer
2. **Mock dependencies** - Use interfaces for testability
3. **Integration tests** - Test critical flows end-to-end
4. **Coverage goals** - Aim for >80% coverage
5. **Test edge cases** - Null values, empty arrays, concurrent access

### RBAC Implementation

1. **Check permissions in handlers** - Before business logic
2. **Compute effective role at runtime** - Never mutate stored roles
3. **Use middleware** - For common permission checks
4. **Feature gate before writes** - Enforce plan limits
5. **Audit permission changes** - Log role updates

---

## 🎯 Common Tasks

### Add a New Entity

1. **Create domain model** in `internal/domain/`
2. **Add to AutoMigrate** in `internal/core/database.go`
3. **Create repository** in `internal/repository/gormrepo/`
4. **Create service** in `internal/service/`
5. **Create handler** in `internal/handler/http/`
6. **Register routes** in `internal/core/router.go`
7. **Write tests** for service and handler

### Add a New API Endpoint

1. **Define route** in `internal/core/router.go`
2. **Create handler function** in appropriate handler file
3. **Add business logic** to service layer
4. **Add repository method** if needed
5. **Add middleware** if auth/RBAC required
6. **Update API documentation**
7. **Write integration test**

### Add Feature Gating

1. **Add check in handler** before write operation:
   ```go
   can, err := h.featureService.CanDoAction(ctx, orgID)
   if err != nil {
       c.JSON(403, gin.H{"error": err.Error()})
       return
   }
   ```
2. **Implement check in FeatureService**
3. **Check plan limits and subscription state**
4. **Return clear error messages**

### Add Background Worker

1. **Create worker file** in `internal/cleanup/`
2. **Implement worker struct** with service dependencies
3. **Create `Run()` method** with ticker loop
4. **Add context cancellation** for graceful shutdown
5. **Start in `main.go`** as goroutine
6. **Add logging** for monitoring

### Update Subscription State

1. **Never update directly** - Use state machine methods:
   ```go
   subscription.Activate()
   subscription.MarkPastDue()
   subscription.Cancel(immediate bool)
   subscription.Expire()
   ```
2. **Validate transitions** - State machine prevents invalid transitions
3. **Update in transaction** - Ensure atomicity
4. **Log state changes** - For audit trail

### Handle Stripe Webhook

1. **Verify signature** - Always validate webhook signature
2. **Check idempotency** - Query `webhook_events` table
3. **Parse event** - Extract subscription/invoice data
4. **Update local state** - Sync with Stripe state
5. **Save webhook event** - For audit and debugging
6. **Return 200 quickly** - Process async if needed

---

## 🚀 Next Steps

When starting a new task, always:

1. **Read this context document** first
2. **Check relevant architecture docs** in `/docs/`
3. **Review existing similar code** for patterns
4. **Follow the established structure** (domain → repo → service → handler)
5. **Write tests** alongside implementation
6. **Update documentation** if adding new features

---

## 📞 Support & Resources

### Documentation

- **This File:** Project context and overview
- **README.md:** Quick start guide
- **docs/:** Detailed architecture documents
- **API Docs:** [Postman Collection](https://documenter.getpostman.com/view/3658426/SzYbyHXj)

### Key Concepts to Understand

1. **Zero-Knowledge Encryption** - Read `MODERN_ENCRYPTION_ARCHITECTURE.md`
2. **SaaS Architecture** - Read `passwall_saas_core_spec.md`
3. **Database Strategy** - Read `DATABASE_STRATEGY.md`
4. **Subscription Lifecycle** - Read `SAAS_REFACTOR_IMPLEMENTATION_SUMMARY.md`

### Commands Reference

```bash
make help              # Show all available commands
make run               # Run server locally
make dev               # Run with hot reload
make test              # Run tests
make lint              # Run linter
make up                # Start with Docker Compose
make logs              # View logs
```

---

**Last Updated:** January 2026  
**Maintained By:** PassWall Team  
**License:** MIT

---

**Remember:** This is a production-grade, zero-knowledge password manager. Security and data integrity are paramount. When in doubt, choose the option that:

1. ✅ Preserves user data
2. ✅ Maintains security
3. ✅ Reduces churn
4. ✅ Enables future recovery

**Happy coding! 🚀**

