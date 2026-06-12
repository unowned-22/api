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

The Domain layer contains only business entities and interfaces (contracts).

**STRICTLY FORBIDDEN** to import:

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

```go id="pxrk1w"
golang.org/x/crypto/bcrypt
```

* Plain-text password storage is strictly prohibited.

---

### Repository Layer (`internal/repository/`)

Responsible for data persistence.

Requirements:

* Must use:

```go id="5zkr8u"
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

```go id="e1u6jf"
github.com/go-chi/chi/v5
```

* Direct usage of:

```go id="sd4hqi"
http.Error(...)
```

is prohibited.

All responses must be returned through the centralized response package and follow a unified JSON format.

Success response:

```json id="jv73bz"
{
  "data": {}
}
```

Error response:

```json id="0pdb6j"
{
  "error": {
    "code": "ERR_CODE",
    "message": "readable message"
  }
}
```

Business logic inside handlers is strictly prohibited.

---

### CLI / CMD Layer (`cmd/`)

Application entry point and bootstrap layer based on:

```go id="6iwv4u"
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

```go id="xj6t2e"
5 * time.Second
```

Shutdown sequence:

1. Stop accepting new requests
2. Complete active requests
3. Close PostgreSQL connection pool
4. Write shutdown logs

---

## 3. Authentication

Authentication must be implemented through the `TokenManager` abstraction.

### Domain Contract

The Domain layer defines the interface:

```go id="0g1eij"
type TokenManager interface {
    Generate(userID int64) (string, error)
    Parse(token string) (int64, error)
}
```

### Infrastructure Implementation

JWT implementation belongs exclusively to the infrastructure layer.

Example location:

```text
internal/auth/jwt.go
```

Requirements:

* Services must depend only on `TokenManager`.
* Services must never import JWT packages directly.
* JWT implementation must be replaceable without changing business logic.

Possible future replacements:

* Redis Sessions
* OAuth2
* Keycloak
* OpenID Connect

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

```json id="gt6p6i"
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

```go id="4y8f5t"
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

## 5. Non-Negotiable Rules

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

Any code generated by an AI agent must comply with these rules.
