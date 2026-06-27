# Architecture Guidelines for AI Agents (Clean Architecture)

This document defines the strict architectural rules and constraints that all AI agents must follow when adding new features, modifying existing code, or performing refactoring.

---

## Agent Workspace Index

To reduce repeated directory scanning, agents may consult this short index describing the repository layout and where to find key concerns.

- `cmd/` — application entrypoints (`cmd/app/main.go`) (thin entrypoint; composition root moved to `internal/bootstrap/`).
- `internal/bootstrap/` — composition root and dependency wiring (`internal/bootstrap/app.go`, `internal/bootstrap/worker.go`).
- `internal/realtime/` — serve-side realtime consumers and WebSocket broadcast helpers.
- `internal/domain/closefriend/` — close-friends domain contract.
- `internal/transport/http/` — HTTP handlers, router, DTOs and response helpers.
- `internal/service/` — business logic / services.
- `internal/repository/postgres/` — raw SQL repository implementations (pgx).
- `internal/domain/` — domain entities and interfaces (contracts).
- `internal/infrastructure/` — integrations (mailer, queue, storage).
- `internal/middleware/` — HTTP middleware (auth, rate limiting, tracing).
- `internal/worker/` — background workers and message handlers.
- `migrations/` — SQL migrations (apply when updating schema).
- `internal/docs/openapi.yaml` — canonical OpenAPI spec for HTTP endpoints.

Agents should prefer this index for quick navigation before doing deep recursive scans.

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

The Domain layer contains only business entities and interfaces (contracts). It is split into focused packages — each owns exactly one concept:

| Package | Contents |
|---|---|
| `internal/domain/user` | `User` entity · `UserRepository` · `UserService` |
| `internal/domain/role` | `Role` entity · `RoleRepository` |
| `internal/domain/permission` | `Permission` entity · `PermissionRepository` · `PermissionService` |
| `internal/domain/token` | `RefreshToken` entity · `RefreshTokenRepository` · `Manager` · `ManagerExtended` |
| `internal/domain/userdevice` | `Device` entity · `Repository` |
| `internal/domain/usersession` | `UserSession` entity · `SessionView` · `UserSessionRepository` |

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

`internal/domain/token/token.go` defines the token Manager contracts.

```go
// Manager is the primary contract — used by services and middleware.
type Manager interface {
Generate(userID int64) (string, error)
Parse(token string) (int64, error)
}

// ManagerExtended embeds Manager and adds role + version-aware token support.
// Used by AuthService and JWTAuth middleware.
// Note: access tokens now include a `ver` claim representing the user's
// current `token_version` value. Implementations MUST surface the token
// version during generation and parsing so middleware can reject stale JWTs.
type ManagerExtended interface {
Manager
// GenerateWithRole creates a token embedding role and the user's token version.
GenerateWithRole(userID int64, tokenVersion int) (string, error)
// ParseWithRole returns userID, role, and tokenVersion extracted from the token.
ParseWithRole(token string) (int64, string, int, error)
}
```

The same file also defines `RefreshToken` and `RefreshTokenRepository` — keeping all token-related domain contracts in one place.

Important notes:
- Access tokens include a `ver` claim mirroring `users.token_version`. When a user's `token_version` increments, previously issued JWTs become invalid.
- Services must call the `UserRepository` contract to increment `token_version` when performing global session invalidation events (e.g. password change, logout-all, admin force logout).
- The JWT middleware (`internal/middleware/jwt.go`) MUST use a `UserService` or `UserRepository` to compare the token's `ver` claim with the current `token_version` and return `401 Unauthorized` on mismatch.

`RefreshToken` tracks lifecycle state (`active`, `revoked`, `expired`) and stores only a hashed refresh token value (`token_hash`). It also carries rotation-chain fields:

| Field | Description |
|---|---|
| `session_id` | FK to the stable `user_sessions` row this token belongs to |
| `parent_token_id` | ID of the token this one replaced (nil for the first token in a chain) |
| `replaced_by_token_id` | ID of the token that replaced this one (set atomically on rotation) |

Refresh rotation must invalidate a refresh token immediately after it is used (atomic `Rotate` call in the repository).

### Session Model

Sessions are stable records that survive token rotation. A session is created once on login and lives until explicitly terminated or expired.

- `UserSession` lives in `internal/domain/usersession`.
- `Device` lives in `internal/domain/userdevice` and is resolved or created on each login using `(user_id, fingerprint)` as the unique key.
- `SessionView` joins `UserSession` with device fields (`device_name`, `browser`, `os`) in a single repository query — the service layer must not issue separate queries to assemble this.

```go
// UserSessionRepository contract
type UserSessionRepository interface {
Create(ctx context.Context, session *UserSession) error
GetByID(ctx context.Context, id int64) (*UserSession, error)
ListActiveByUserID(ctx context.Context, userID int64) ([]*SessionView, error)
Terminate(ctx context.Context, id int64) error
TerminateAll(ctx context.Context, userID int64) error
UpdateLastActivity(ctx context.Context, id int64, t time.Time) error
}
```

Login flow:

1. Validate credentials and email verification.
2. Resolve or create the `Device` record (`GetByFingerprint` → `Create`).
3. Create a `UserSession` linked to that device.
4. Create the first `RefreshToken` in the rotation chain, linked to the session.
5. Return access token + raw refresh token.

### Infrastructure Implementation

JWT implementation belongs exclusively to the infrastructure layer.

Location:

```text
internal/auth/jwt.go
```

`JWTManager` satisfies both `token.Manager` and `token.ManagerExtended`. Compile-time checks are enforced with `var _ token.Manager = (*JWTManager)(nil)`.

Requirements:

* JWTManager must emit and validate standard registered claims: `iss`, `aud`, `sub`, `jti`, `iat`, `nbf`, and `exp`.
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
  - updating the user's hashed password, marking the token used, terminating all sessions (`UserSessionRepository.TerminateAll`) and revoking all refresh tokens (`RefreshTokenRepository.RevokeAllByUserID`) for the user.
- Transport: HTTP handlers expose two endpoints:
  - `POST /api/v1/auth/forgot-password` — accepts `{"email": "..."}` and always responds 200 with a generic message (prevents account enumeration).
  - `POST /api/v1/auth/reset-password` — accepts `{"token": "...", "new_password": "..."}` and performs the reset.

Security notes:
- Reset tokens are short-lived and single-use; services must check `expires_at` and `used_at`.
- On successful password reset, all sessions are terminated and all refresh tokens are revoked to force re-authentication.
- Email sending failures during token creation are logged but do not cause the API to reveal token state to callers.
- Administrators may also deactivate accounts (set `deactivated_at`); deactivated accounts must be denied login and token refresh and all sessions/tokens must be revoked.

### Refresh token reuse detection

The system detects reuse of revoked refresh tokens (an indicator of stolen tokens). When detected, services must:

- Immediately revoke all tokens belonging to the **affected session** (`RefreshTokenRepository.RevokeSessionTokens`).
- Terminate the **affected session only** (`UserSessionRepository.Terminate`). Other sessions for the same user are left intact.
- Publish an audit event named `audit.refresh_token_reuse_detected` containing `user_id`, `session_id`, `ip_address`, `user_agent`, and `token_hash`.
- Optionally send a notification email to the affected user.

> **Rationale:** Session-scoped revocation limits blast radius. A stolen token from one device should not force re-authentication on all the user's other devices. Full user-wide revocation remains available via `LogoutAll` / `DeactivateUser`.

Consumers handling audit events should persist `audit.refresh_token_reuse_detected` entries to `audit_logs` for security investigation.

---

Stories feature notes
---------------------

The project includes a Stories feature implemented under `internal/domain/story`, `internal/service`, `internal/repository/postgres/story_repository.go`, and exposed via `internal/transport/http/handler/story_handler.go`.

- Storage model: one DB row per published story (not per user); slides are stored as opaque JSON (`json.RawMessage` end-to-end — neither the repository nor the service validates slide internals beyond array length). Media is uploaded separately via `POST /api/v1/stories/media` into a private bucket; only the storage key is persisted in the slide JSON, and short-lived (15 minute) presigned URLs are generated on every read (`presignSlides` in the handler).
- Endpoints: `POST /api/v1/stories` (publish), `GET /api/v1/stories/me`, `GET /api/v1/stories/feed`, `DELETE /api/v1/stories/{id}`, `POST /api/v1/stories/{id}/view`, `POST /api/v1/stories/{id}/like`, `POST /api/v1/stories/{id}/unlike`, `POST /api/v1/stories/{id}/reply`, `POST /api/v1/stories/media`. Keep `internal/docs/openapi.yaml` in sync with this list per §6.
- Repository uses `Create` (one row per publish); `StoryService` satisfies `story.StoryService` in full — `Publish`, `Feed`, `ListMyStories`, `AddView`, `ListViewsByViewer`, `Delete`, `Like`, `Unlike`, `Reply`, `ListReplies` are all implemented.

- Visibility enforcement — **currently not applied on the live feed path**: the handler's `Feed` calls `StoryService.Feed`, which is expected to call `repo.ListFeed`. That SQL query only filters out expired stories and rows where the viewer is explicitly listed in `hidden_from_user_ids` — it does **not** look at `visibility` at all. In practice this means `friends` and `close` stories are currently shown to *any* authenticated viewer in the feed, exactly like `everyone`, unless the author explicitly muted that viewer via `hidden_from`.
- There is a method `StoryService.ListVisibleStories(ctx, viewerID, authorID)` that implements correct per-author visibility filtering (checks `friendshipSvc.IsFriend` for `friends`, and uses the `close_friends` table for `close`). It is not part of the `story.StoryService` interface and is not called by the feed path — feed visibility enforcement remains an open item (see note below).
- `close_friends` is now a supported feature with its own API under `/api/v1/users/me/close-friends` (`GET`, `POST`, `DELETE`). Keep `internal/docs/openapi.yaml` in sync whenever those endpoints or their DTOs change.
- Realtime notification handlers that push to connected WebSocket clients belong in `serve` alongside the hub and RabbitMQ realtime consumers. Do not add new hub-dependent notification handlers to `worker`.

Agents making changes to Story code should: (1) fix the build-breaking gap above first, (2) decide and document how `friends`/`close` visibility is enforced in the feed query before shipping it as a real feature, (3) preserve the per-story-row model and the private-media presign pattern unless a migration plan and data-backfill are provided.

## Transactional Outbox Pattern

The application uses a Transactional Outbox to reliably publish domain events to external brokers (RabbitMQ) while keeping domain state changes and event persistence atomic.

- Persist events into `outbox_events` within the same DB transaction as domain changes.
- A background worker reads `pending` events (using `FOR UPDATE SKIP LOCKED`), marks them `processing`, republishes to the broker, and then marks them `processed` or `failed`.
- Supported statuses: `pending`, `processing`, `processed`, `failed`.
- Use an `OutboxPublisher` adapter in `internal/infrastructure/outbox` to write events into the outbox instead of publishing directly to AMQP from services.
- The worker lives in `internal/worker/outbox` and republishes using the existing AMQP publisher (`internal/infrastructure/queue`).
- Implement an in‑proc bridge (`internal/infrastructure/outbox/bridge.go`) that lets the in‑process event bus write events to outbox for durability.
- Use `retry_count` and a configurable retry policy; consider adding `next_attempt_at` for backoff in the future.
- For strong guarantees across services consider implementing an Outbox drain/dispatcher with idempotence keys and an explicit deduplication strategy.

Location:

- Outbox domain contract: `internal/domain/outbox`
- Postgres repo: `internal/repository/postgres/outbox`
- Worker: `internal/worker/outbox`
- Outbox publisher adapter: `internal/infrastructure/outbox`

Usage guidance:

- Services should write events via the `event.Publisher` abstraction which can be backed by the OutboxPublisher during normal operation.
- The worker republishes to RabbitMQ so existing downstream consumers continue to work without changes.


## Rate Limiting

The application protects critical authentication endpoints against brute-force and credential stuffing attacks using endpoint-specific rate limiting implemented in the middleware layer.

### Design

Rate limiting is implemented through two mechanisms:

1. **Global Rate Limiter** (`middleware.RateLimit`): A shared token bucket limiter using `golang.org/x/time/rate` applied to all endpoints by default. Configured via `RATE_LIMIT` (requests) and `RATE_LIMIT_WINDOW` (duration).

2. **Auth Endpoint Rate Limiter** (`middleware.AuthRateLimiter`): Custom in-memory rolling window tracker for authentication endpoints with per-IP and per-email tracking. Separate limits and windows per endpoint.

### Location

Rate limiting middleware is defined in:

```text
internal/middleware/ratelimit.go
internal/middleware/auth_ratelimit.go
```

### Implementation Requirements

**AuthRateLimiter Interface:**

- `Allow(identifier string) (allowed bool, remaining int)`: Check if a request is allowed; return remaining attempts in current window
- `Stop()`: Gracefully shutdown the limiter, stopping cleanup goroutine

**Endpoint-Specific Configuration:**

| Endpoint | Limiter | Tracking | Config Variables |
|---|---|---|---|
| `POST /api/v1/auth/login` | `AuthRateLimiter` | Per IP + Email | `LOGIN_RATE_LIMIT`, `LOGIN_RATE_LIMIT_WINDOW` |
| `POST /api/v1/auth/register` | `AuthRateLimiter` | Per IP + Email | `REGISTER_RATE_LIMIT`, `REGISTER_RATE_LIMIT_WINDOW` |
| `POST /api/v1/auth/forgot-password` | `AuthRateLimiter` | Per IP + Email | `FORGOT_PASSWORD_RATE_LIMIT`, `FORGOT_PASSWORD_RATE_LIMIT_WINDOW` |
| `POST /api/v1/auth/resend-verification` | `AuthRateLimiter` | Per IP + Email | `RESEND_VERIFICATION_RATE_LIMIT`, `RESEND_VERIFICATION_RATE_LIMIT_WINDOW` |
| `POST /api/v1/auth/verify-email` | Global | Per IP | `RATE_LIMIT`, `RATE_LIMIT_WINDOW` |
| `POST /api/v1/auth/reset-password` | Global | Per IP | `RATE_LIMIT`, `RATE_LIMIT_WINDOW` |
| `POST /api/v1/auth/refresh` | Global | Global | `RATE_LIMIT`, `RATE_LIMIT_WINDOW` |
| `POST /api/v1/auth/logout` | Global | Global | `RATE_LIMIT`, `RATE_LIMIT_WINDOW` |
| `POST /api/v1/auth/logout-all` | Authenticated | Global | `RATE_LIMIT`, `RATE_LIMIT_WINDOW` |

### Middleware Helpers

**`AuthRateLimitByIP(limiter)`**: Middleware that extracts client IP and calls `limiter.Allow(ip)`. Used for endpoints requiring only IP-based limiting.

**`AuthRateLimitByEmail(limiter, emailExtractor)`**: Middleware that extracts client IP and email from request (via custom function), creates combined identifier `"ip:email"`, and calls `limiter.Allow()`. Used for endpoints where email is the user identifier.

The `emailExtractor` function reads the request body, parses JSON to extract the email field, and restores the body for the next handler. Example:

```go
emailExtractorFunc := func(r *http.Request) (string, error) {
  var req struct {
    Email string `json:"email"`
  }
  body, _ := io.ReadAll(r.Body)
  r.Body = io.NopCloser(bytes.NewBuffer(body))
  json.Unmarshal(body, &req)
  return req.Email, nil
}
```

### Dependency Injection

In `cmd/app/main.go`, create one `AuthRateLimiter` instance per endpoint:

```go
loginLimiter := middleware.NewAuthRateLimiter(
config.LoginRateLimit,
config.LoginRateLimitWindow,
)
defer loginLimiter.Stop()

// ... create 3 more limiters for register, forgot-password, resend-verification

router := http.NewRouter(loginLimiter, registerLimiter, forgotPasswordLimiter, resendVerificationLimiter)
```

### Housekeeping

- Each `AuthRateLimiter` runs a cleanup goroutine that removes expired request records every minute to prevent memory leaks.
- Always call `limiter.Stop()` in graceful shutdown sequence to ensure goroutines terminate cleanly.
- Stale records are automatically cleaned when their time window expires.

### Error Response

When rate limit is exceeded, middleware responds with HTTP 429:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "too many requests, please try again later"
  }
}
```

With header:

```http
X-RateLimit-Remaining: 0
```

### Future Considerations

- **Distributed Deployments**: For horizontal scaling, consider replacing in-memory `AuthRateLimiter` with Redis-based implementation. Interface remains unchanged; only `internal/middleware/auth_ratelimit.go` needs modification.
- **Custom Policies**: Add per-user or per-role rate limiting without architectural changes.
- **Metrics**: Expose rate limit violations via Prometheus for monitoring and alerts.

---

## 5. Logging and Error Handling

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
ErrDeviceNotFound     = errors.New("device not found")
ErrSessionExpired     = errors.New("session has expired")
ErrSessionRevoked     = errors.New("session has been revoked")
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

## 6. API Documentation (OpenAPI / Swagger)

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
- [ ] `tags:` matches one of the existing tag groups (`auth`, `password-reset`, `users`, `admin`, `uploads`, `health`, `friends`, `stories`) or a new tag is added to the top-level `tags:` list.
- [ ] `security: - BearerAuth: []` is present on every protected endpoint.
- [ ] The `servers:` block still points to the correct local development URL.

### What must NOT be done

* Do not add swaggo/swag annotations (`// @Summary`, `// @Router`, etc.) — the project uses a hand-maintained YAML spec, not code-generated docs.
* Do not create a separate `docs/swagger.json` or `docs/swagger.yaml` at the project root — the single source is `internal/docs/openapi.yaml`.
* Do not remove or rename existing `operationId` values without updating all references.

---

## 7. Non-Negotiable Rules

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
* **Never inject `*pgxpool.Pool` directly into service structs** — the pool is only allowed in repository constructors and `HealthService`.

Any code generated by an AI agent must comply with these rules.


## Photos & Comments (new feature guidance)

When adding photo metadata, comments, and likes, follow these rules and patterns to remain consistent with the project's architecture and operational requirements:

- Migrations: place SQL migrations in `migrations/` and update `internal/docs/openapi.yaml` in the same commit. The migration should add the new `photos` columns (`device_name`, `device_os`, `device_type`, `latitude`, `longitude`, `location_name`, `exif_data`, `likes_count`, `comments_count`) and create `photo_likes`, `photo_comments`, and `photo_comment_likes` tables with appropriate indexes and FK constraints.

- Domain & Repository: create `internal/domain/photocomment` for the `Comment` entity and contracts. Repository implementations must use `github.com/jackc/pgx/v5/pgxpool` and raw SQL only, and translate DB errors into domain errors declared in `internal/errs`.

- Transactions & Counters: update denormalized counters (`photos.comments_count`, `photos.likes_count`, `photo_comments.likes_count`) inside DB transactions so counters remain consistent under concurrency. Prefer a single transaction when creating a comment and updating the photo counter.

- Reply semantics: enforce single-level replies in the service layer (reject replies to comments that already have a `parent_id`). Map the domain error to HTTP 422 in transport.

- EXIF & Device metadata: handlers may accept optional multipart fields (latitude/longitude/location_name/device_*). Services should also attempt to parse EXIF GPS tags (recommended library: `github.com/rwcarlsen/goexif/exif`) and store selected EXIF tags in `photos.exif_data` as JSONB. When the client omits device info, parse `User-Agent` heuristically in the handler (there is already `internal/pkg/uaparser` for this purpose).

- Notifications: use `internal/domain/notification` to create notifications for `photo_commented`, `comment_replied`, and `comment_liked`. Prefer writing notifications to the transactional outbox when cross-process delivery is required.
- Realtime delivery: `serve` owns the WebSocket hub and RabbitMQ realtime consumers. `worker` must not create or depend on a `ws.Hub`.

- OpenAPI: document every new endpoint, request/response schema, and response codes in `internal/docs/openapi.yaml` and keep `operationId` values consistent with handler names.

Add this section to PR descriptions when introducing photos/comments changes so reviewers can verify migration, OpenAPI, and transactional correctness.

---

## Messenger Feature Guidance

Rules established during TASK-13/14 that all agents must follow when touching messenger code.

### Atomicity: message + attachments

`MessageRepository.CreateWithAttachments` is the **only** correct way to persist a new message with attachments. It wraps the `messages` insert and all `message_attachments` inserts in a single DB transaction, so a mid-loop failure cannot leave an orphaned message without some of its attachments.

**Never** call `Create` + `CreateAttachment` in a loop from the service layer — that is the anti-pattern this method was introduced to eliminate.

`draftRepo.Delete` and `convRepo.UpdateLastMessage` are intentionally left **outside** the transaction in `SendMessage`: a draft that wasn't cleaned up is cosmetic; `last_message_id` self-corrects on the next send. Log their failures at `Warn`, never return them as errors to the caller.

### Block enforcement on every send

`PrivacyRepo.IsBlocked` must be called inside `SendMessage` for **every** `TypeDirect` message, not only during `GetOrCreateDirect`. A block placed after a conversation was created must take effect immediately.

Two directions must be checked:
- Recipient blocked sender (`IsBlocked(otherID, senderID)`)
- Sender blocked recipient (`IsBlocked(senderID, otherID)`)

Both result in `errs.ErrUserBlocked` → HTTP 403 `USER_BLOCKED`. This check does **not** apply to groups or channels (moderation there is role/kick-based).

### Pre-load pattern: avoid duplicate ListMembers

When `SendMessage` for a direct conversation needs the member list for multiple purposes (block-check, publish payload), load it **once** and pass the slice to helpers. Never call `ListMembers` twice for the same `convID` in the same call stack.

Concrete rule: `otherIDFromMembers` is a pure function operating on an already-loaded slice. `publishMessageSentWithMembers` accepts `members []*messenger.ConversationMember` and falls back to a DB call only when `members == nil` (group/channel path). This pattern must be preserved when extending `SendMessage`.

### Hub lock order

`Hub` uses two mutexes. They must **always** be acquired in this order:

```
presenceMu → mu
```

No code path may acquire `mu` first and then `presenceMu`. Violating this order will cause a deadlock. Document any new goroutine that touches both locks with an explicit comment referencing this order.

### Worker: memoize ListMembers per conversation

Background workers that process multiple messages per ticker pass (e.g. `DisappearingMessageWorker`, `ScheduledMessageWorker`) must memoize `ListMembers` by `conversation_id` within a single invocation. Multiple expired/due messages in the same conversation must not produce multiple identical DB queries.

Pattern:

```go
membersCache := make(map[int64][]*messenger.ConversationMember)
for _, m := range msgs {
members, ok := membersCache[m.ConversationID]
if !ok {
members, err = memberRepo.ListMembers(ctx, m.ConversationID)
if err != nil { continue }
membersCache[m.ConversationID] = members
}
// use members
}
```

### Error catalogue additions

| Sentinel | Package | HTTP | Code |
|---|---|---|---|
| `ErrUserBlocked` | `internal/errs` | 403 | `USER_BLOCKED` |

New messenger errors must be declared in `internal/errs/errors.go` with an explanatory comment and mapped in `internal/transport/http/response/response.go`.