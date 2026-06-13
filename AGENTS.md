# Architecture Guidelines for AI Agents (Clean Architecture)

This document defines the strict architectural rules and constraints that all AI agents must follow when adding new features, modifying existing code, or performing refactoring.

---

## 1. Layer Dependency Rules

Dependencies must always flow in a single direction:

```text
HTTP / CLI -> Service -> Repository -> PostgreSQL
```

* Outer layers are allowed to know about inner layers.
* Inner layers must **never** know implementation details of outer layers.
* Communication between layers must occur exclusively through interfaces defined in the **Domain** layer.

---

## 2. Layer Responsibilities

### Domain Layer (`internal/domain/`)

The Domain layer contains only business entities and interfaces (contracts). It is split into four focused packages — each owns exactly one concept:

| Package | Contents |
|---|---|
| `internal/domain/user` | `User` entity · `UserRepository` · `UserService` |
| `internal/domain/role` | `Role` entity · `RoleRepository` |
| `internal/domain/permission` | `Permission` entity · `PermissionRepository` · `PermissionService` |
| `internal/domain/token` | `RefreshToken` entity · `RefreshTokenRepository` · `Manager` · `ManagerExtended` |

Cross-domain references use plain scalar types (`int64`, `string`) instead of importing sibling packages. This keeps the dependency graph acyclic — no domain package imports another domain package.

**STRICTLY FORBIDDEN** to import in any domain package:

* Any database libraries (`pgx`, `sql`, `gorm`, etc.)
* Routers and HTTP packages (`chi`, `http`, `gin`)
* Logging frameworks (`logrus`, `zap`)
* Infrastructure authentication packages (`jwt`, `oauth`)

The Domain layer must depend only on the Go standard library (e.g. `time`, `context`).

---

### Service Layer (`internal/service/`)

Contains the application's business logic.

Requirements:

* Must interact with repositories and authentication managers exclusively through Domain interfaces.
* Must remain independent of JWT implementation details.
* Must remain independent of PostgreSQL implementation details.
* Passwords must only be stored and processed as hashes using:

```go
golang.org/x/crypto/bcrypt
```

* Plain-text password storage is strictly prohibited.

---

### Repository Layer (`internal/repository/`)

Responsible for data persistence.

Requirements:

* Must use:

```go
github.com/jackc/pgx/v5/pgxpool
```

* **ORMs are STRICTLY PROHIBITED**:

  * GORM
  * Ent
  * Bun
  * Any other ORM

Only raw SQL with pgx is allowed.

Additional requirements:

* Repository implementations must translate database-specific errors into domain-level errors.
* Example: PostgreSQL unique constraint violation (`23505`) should be converted into an appropriate application error from:

```text
internal/errs
```

* Repository layer must not contain business logic.

---

### HTTP / Transport Layer (`internal/transport/http/`)

Responsible for:

* Receiving requests
* Request validation
* Calling services
* Returning responses

Requirements:

* Routing must be implemented using:

```go
github.com/go-chi/chi/v5
```

* Direct usage of:

```go
http.Error(...)
```

is prohibited.

All responses must be returned through the centralized response package and follow a unified JSON format.

Success response:

```json
{
  "data": {}
}
```

Error response:

```json
{
  "error": {
    "code": "ERR_CODE",
    "message": "readable message"
  }
}
```

### Response Types
All handler responses must use typed DTO structs. Anonymous maps (`map[string]string`)
are strictly prohibited in handler responses.

Response structs must be declared in:

```text
internal/transport/http/dto/
```

Simple message responses must use:

```go
dto.MessageResponse{Message: "..."}
```

Business logic inside handlers is strictly prohibited.

---

### CLI / CMD Layer (`cmd/`)

Application entry point and bootstrap layer based on:

```go
github.com/spf13/cobra
```

Responsibilities:

* Configuration loading
* Dependency injection
* Application startup
* Graceful shutdown

### Dependency Injection

All dependencies must be assembled manually in a single composition root.

Initialization order:

```text
Config
    ↓
Logger
    ↓
Database
    ↓
Repositories
    ↓
TokenManager
    ↓
Services
    ↓
Handlers
    ↓
Router
    ↓
HTTP Server
```

Global variables for business logic are prohibited.

---

### Graceful Shutdown

Every HTTP server must support graceful shutdown.

Requirements:

* Handle:

  * `SIGINT`
  * `SIGTERM`

* Shutdown timeout:

```go
5 * time.Second
```

Shutdown sequence:

1. Stop accepting new requests
2. Complete active requests
3. Close PostgreSQL connection pool
4. Write shutdown logs

---

## 3. Authentication

Authentication must be implemented through the `token.Manager` abstraction defined in `internal/domain/token`.

### Domain Contract

`internal/domain/token/token.go` defines two interfaces:

```go
// Manager is the primary contract — used by services and middleware.
type Manager interface {
Generate(userID int64) (string, error)
Parse(token string) (int64, error)
}

// ManagerExtended embeds Manager and adds role-aware token support.
// Used by AuthService and JWTAuth middleware.
type ManagerExtended interface {
Manager
GenerateWithRole(userID int64, role string) (string, error)
ParseWithRole(token string) (int64, string, error)
}
```

The same file also defines `RefreshToken` and `RefreshTokenRepository` — keeping all token-related domain contracts in one place.

### Infrastructure Implementation

JWT implementation belongs exclusively to the infrastructure layer.

Location:

```text
internal/auth/jwt.go
```

`JWTManager` satisfies both `token.Manager` and `token.ManagerExtended`. Compile-time checks are enforced with `var _ token.Manager = (*JWTManager)(nil)`.

Requirements:

* Services must depend only on `token.Manager` or `token.ManagerExtended`.
* Services must never import JWT packages directly.
* JWT implementation must be replaceable without changing business logic.

Possible future replacements:

* Redis Sessions
* OAuth2
* Keycloak
* OpenID Connect

## Password Reset Flow

The application includes a secure password reset flow implemented according to the same Clean Architecture rules.

- Persistence: a new table `password_reset_tokens` stores one-time reset tokens with fields: `id`, `user_id`, `token`, `expires_at`, `used_at`, `created_at`.
- Domain: new domain package `internal/domain/passwordreset` defines `Token` and the `Repository` interface (Create, GetByToken, MarkUsed, DeleteByUserID).
- Repository: implementations live in `internal/repository/postgres` and perform raw SQL against the `password_reset_tokens` table. Repositories translate DB errors into `internal/errs` values.
- Service: `PasswordResetService` (in `internal/service`) is responsible for:
  - creating a single active reset token per user (old tokens are deleted),
  - generating cryptographically secure tokens,
  - rendering and sending the reset email via the `domain/mailer` contract,
  - validating tokens (expiry and used-state),
  - updating the user's hashed password, marking the token used, and revoking all refresh tokens for the user.
- Transport: HTTP handlers expose two endpoints:
  - `POST /api/v1/auth/forgot-password` — accepts `{"email": "..."}` and always responds 200 with a generic message (prevents account enumeration).
  - `POST /api/v1/auth/reset-password` — accepts `{"token": "...", "new_password": "..."}` and performs the reset.

Security notes:
- Reset tokens are short-lived and single-use; services must check `expires_at` and `used_at`.
- On successful password reset, all refresh tokens are revoked using the `RefreshTokenRepository` contract to force re-authentication.
- Email sending failures during token creation are logged but do not cause the API to reveal token state to callers.

---

## 4. Logging and Error Handling

### Logging

Logging must be implemented using a singleton `logrus` logger configured with JSON formatting.

Requirements:

* Structured logging
* Request logging middleware
* Panic recovery logging
* Error logging

Example:

```json
{
  "level": "info",
  "method": "POST",
  "path": "/api/v1/auth/login",
  "status": 200,
  "duration_ms": 14
}
```

---

### Request Tracing

A Request ID middleware is mandatory.

Requirements:

* Generate a unique request ID for every request
* Expose it through the `X-Request-Id` header
* Include it in logs whenever possible

---

### Error Management

Application errors must be declared in:

```text
internal/errs/errors.go
```

Example:

```go
var (
ErrUserNotFound       = errors.New("user not found")
ErrInvalidCredentials = errors.New("invalid credentials")
ErrUserAlreadyExists  = errors.New("user already exists")
)
```

Requirements:

* Domain and service layers return domain errors.
* Transport layer maps errors to:

  * HTTP status codes
  * API error codes
  * Human-readable messages

Error mapping must be centralized in:

```text
internal/transport/http/response/response.go
```

No duplicated error handling logic is allowed across handlers.

---

## 5. API Documentation (OpenAPI / Swagger)

The API specification is maintained as a single source of truth in:

```text
internal/docs/openapi.yaml
```

The file is embedded into the binary at compile time via `internal/docs/openapi.go` using `//go:embed`.

The interactive Swagger UI is served at:

```
GET /swagger/index.html
GET /swagger/openapi.yaml   ← raw spec consumed by the UI
```

Routes are only registered when `APP_ENV != production`.

### Mandatory rule: keep the spec in sync

**Every time you add, remove, or change an HTTP endpoint you must update `internal/docs/openapi.yaml` in the same commit/PR.**

Checklist for any endpoint change:

- [ ] New path + HTTP method added under `paths:`.
- [ ] Request body schema added/updated in `components/schemas/`.
- [ ] All possible response codes documented (including `400`, `401`, `403`, `422`, `429`, `500` where applicable).
- [ ] `operationId` is unique and matches the handler name in snake_case (e.g. `authLogin`, `usersMe`).
- [ ] `tags:` matches one of the existing tag groups (`auth`, `password-reset`, `users`, `admin`, `uploads`, `health`) or a new tag is added to the top-level `tags:` list.
- [ ] `security: - BearerAuth: []` is present on every protected endpoint.
- [ ] The `servers:` block still points to the correct local development URL.

### What must NOT be done

* Do not add swaggo/swag annotations (`// @Summary`, `// @Router`, etc.) — the project uses a hand-maintained YAML spec, not code-generated docs.
* Do not create a separate `docs/swagger.json` or `docs/swagger.yaml` at the project root — the single source is `internal/docs/openapi.yaml`.
* Do not remove or rename existing `operationId` values without updating all references.

---

## 6. Non-Negotiable Rules

The following rules are mandatory and must never be violated:

* Follow Clean Architecture principles.
* Use Dependency Injection.
* Pass `context.Context` through all layers.
* Use Repository Pattern.
* Keep business logic inside services only.
* Keep SQL inside repositories only.
* Keep JWT implementation behind `TokenManager`.
* Use PostgreSQL via pgx only.
* Use Chi for HTTP routing.
* Use Cobra for CLI commands.
* Use Logrus for logging.
* Use golang-migrate for migrations.
* Do not use ORM libraries.
* Do not use global state for business logic.
* Do not place business logic inside HTTP handlers.
* Do not introduce layer dependency violations.
* **Keep `internal/docs/openapi.yaml` in sync with every endpoint change.**

Any code generated by an AI agent must comply with these rules.