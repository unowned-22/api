# Go REST API (Clean Architecture)

Production-ready REST API built with Go using Clean Architecture, Cobra CLI, pgx (PostgreSQL), JWT authentication, and logrus for structured logging.

The API supports role-based access control with the following roles:

* `admin`
* `moderator`
* `user`

Newly registered users receive the `user` role by default.
Roles own permissions, and users inherit permissions through their assigned role.
New protected features should prefer permission checks over direct role checks.

Initial permissions:

* `users.read`
* `users.create`
* `users.update`
* `users.delete`
* `roles.read`
* `roles.update`
* `admin.access`

## Requirements

* Go 1.26+
* PostgreSQL

---

## Quick Start

### 1. Environment Configuration

Create a `.env` file in the project root directory (default values are already initialized):

```env
APP_PORT=8080
APP_ENV=development
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=unnamed_db
DB_SSL_MODE=disable
JWT_SECRET=super-secret-key-change-me-in-production
JWT_ISSUER=api-service
JWT_AUDIENCE=client-app
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h

# Auth Endpoint Rate Limiting (per IP and email)
LOGIN_RATE_LIMIT=5
LOGIN_RATE_LIMIT_WINDOW=5m
REGISTER_RATE_LIMIT=3
REGISTER_RATE_LIMIT_WINDOW=1h
FORGOT_PASSWORD_RATE_LIMIT=3
FORGOT_PASSWORD_RATE_LIMIT_WINDOW=15m
RESEND_VERIFICATION_RATE_LIMIT=3
RESEND_VERIFICATION_RATE_LIMIT_WINDOW=15m
```

### 2. Build the Application

Compile the executable:

```bash
go build -o app cmd/app/main.go
```

### 3. Run Database Migrations

Apply migrations to create the database schema:

```bash
./app migrate up
```

### 4. Start the HTTP Server

Run the server:

```bash
./app serve
```

---

## API Documentation (Swagger UI)

When `APP_ENV` is set to `development`, the interactive Swagger UI is available at:

```
http://localhost:8080/swagger/index.html
```

The raw OpenAPI 3.0 spec is served at:

```
http://localhost:8080/swagger/openapi.yaml
```

The spec is embedded into the binary from `internal/docs/openapi.yaml` — no separate build step required.

> **Note for contributors:** every new or changed endpoint must be reflected in `internal/docs/openapi.yaml` in the same PR. See `AGENTS.md` § 5 for the full checklist.

Swagger UI is **not** available when `APP_ENV=production`.

---

## CLI Usage

The application supports the following CLI commands:

### Start the HTTP server

```bash
./app serve
```

### Apply all migrations

```bash
./app migrate up
```

### Roll back migrations

By default this rolls back one migration. Use `--steps` to roll back more than one migration.

```bash
./app migrate down
./app migrate down --steps 3
```

### Roll back all migrations

Full rollback requires explicit confirmation.

```bash
./app migrate reset --force
```

### Show current migration version

```bash
./app migrate version
```

### Create a new migration template

```bash
./app migrate create name_of_migration
```

---

## API Request Examples (cURL)

### 1. Register a New User

Create a new user account. `full_name`, `username`, and optionally `phone` are required in addition to credentials.

| Field | Required | Rules |
|---|---|---|
| `email` | Yes | valid email |
| `password` | Yes | min 8 characters |
| `full_name` | Yes | 2–100 characters |
| `username` | Yes | 3–30 characters, only `a-z A-Z 0-9 . - _` |
| `phone` | No | must start with `+`, max 20 characters |

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepassword123",
    "full_name": "Jane Smith",
    "username": "jane.smith",
    "phone": "+79991234567"
  }'
```

#### Successful Response

```json
{
  "data": {
    "message": "user registered successfully, please check your email to verify your account"
  }
}
```

---

### 2. Authenticate (Login)

Request a JWT access token and refresh token:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepassword123"
  }'
```

#### Successful Response

Copy the `access_token` and `refresh_token` values from the response:

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "Y7vL6mQ9qF..."
  }
}
```

---

### 3. Refresh Access Token

Use a valid refresh token to get a new access token and a rotated refresh token. The old refresh token becomes invalid immediately after use.

> Refresh tokens are stored securely in the database as a SHA-256 hash (`token_hash`). The raw token is never persisted.

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<PASTE_YOUR_REFRESH_TOKEN_HERE>"
  }'
```

#### Successful Response

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "Y7vL6mQ9qF..."
  }
}
```

---

### 4. Logout (Revoke Refresh Token)

Revoke a refresh token:

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<PASTE_YOUR_REFRESH_TOKEN_HERE>"
  }'
```

#### Successful Response

```json
{
  "data": {
    "message": "logged out successfully"
  }
}
```

---

### 5. List Active Sessions (Protected Route)

Use the JWT access token in the `Authorization` header:

```bash
curl -X GET http://localhost:8080/api/v1/auth/sessions \
  -H "Authorization: Bearer <PASTE_YOUR_ACCESS_TOKEN_HERE>"
```

#### Successful Response

```json
{
  "data": {
    "sessions": [
      {
        "id": 42,
        "user_id": 1,
        "device_name": "Chrome on macOS",
        "user_agent": "Mozilla/5.0",
        "ip_address": "203.0.113.10",
        "created_at": "2026-06-13T10:15:30Z",
        "last_used_at": "2026-06-13T11:20:00Z"
      }
    ]
  }
}
```

---

### 6. Revoke Session (Protected Route)

Revoke a session and its associated refresh token. Users can revoke their own sessions; administrators can revoke any session.

```bash
curl -X DELETE http://localhost:8080/api/v1/auth/sessions/42 \
  -H "Authorization: Bearer <PASTE_YOUR_ACCESS_TOKEN_HERE>"
```

#### Successful Response

```json
{
  "data": {
    "message": "session revoked successfully"
  }
}
```

---

### 7. Get User Profile (Protected Route)

Use the JWT access token in the `Authorization` header:

```bash
curl -X GET http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer <PASTE_YOUR_ACCESS_TOKEN_HERE>"
```

#### Successful Response

```json
{
  "data": {
    "id": 1,
    "email": "user@example.com",
    "full_name": "Jane Smith",
    "username": "jane.smith",
    "phone": "+79991234567",
    "role": "user",
    "created_at": "2026-06-12T09:53:01Z"
  }
}
```

---

### 8. Admin Ping (Admin Protected Route)

Use an access token for a user with the `admin` role:

```bash
curl -X GET http://localhost:8080/api/v1/admin/ping \
  -H "Authorization: Bearer <PASTE_ADMIN_ACCESS_TOKEN_HERE>"
```

#### Successful Response

```json
{
  "data": {
    "message": "admin access granted"
  }
}
```

---

### 9. List Admin Permissions (Permission Protected Route)

Use an access token for a user whose role has the `admin.access` permission:

```bash
curl -X GET http://localhost:8080/api/v1/admin/permissions \
  -H "Authorization: Bearer <PASTE_ACCESS_TOKEN_HERE>"
```

#### Successful Response

```json
{
  "data": {
    "permissions": [
      {
        "id": 1,
        "name": "admin.access",
        "description": "Access administration endpoints",
        "created_at": "2026-06-12T09:53:01Z"
      }
    ]
  }
}
```

---

### 10. Password Reset

Initiate a password reset (silent response to prevent account enumeration):

```bash
curl -X POST http://localhost:8080/api/v1/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com"}'
```

#### Successful Response (always 200)

```json
{
  "data": {
    "message": "if the email exists, a reset link has been sent"
  }
}
```

Complete the reset using the token from the email:

```bash
curl -X POST http://localhost:8080/api/v1/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "token": "<RESET_TOKEN_FROM_EMAIL>",
    "new_password": "newSecurePassword123"
  }'
```

#### Successful Response

```json
{
  "data": {
    "message": "password has been reset successfully"
  }
}
```

> **Note:** The `forgot-password` endpoint intentionally returns 200 OK regardless of whether the email exists to avoid leaking valid accounts. Reset tokens are short-lived and marked as used after a successful reset. All refresh tokens for the user are revoked on password change.

---

## Security: Rate Limiting

The API protects critical authentication endpoints against brute-force and credential stuffing attacks using endpoint-specific rate limiting.

### Rate Limiting Configuration

Rate limits are applied per IP address and per email address (where applicable). Limits are configurable via environment variables:

| Endpoint | Limit | Default | Env Variable | Window Env Variable |
|---|---|---|---|---|
| **POST /api/v1/auth/login** | Per IP + Email | 5 attempts | `LOGIN_RATE_LIMIT` | `LOGIN_RATE_LIMIT_WINDOW` (default: 5m) |
| **POST /api/v1/auth/register** | Per IP + Email | 3 registrations | `REGISTER_RATE_LIMIT` | `REGISTER_RATE_LIMIT_WINDOW` (default: 1h) |
| **POST /api/v1/auth/forgot-password** | Per IP + Email | 3 requests | `FORGOT_PASSWORD_RATE_LIMIT` | `FORGOT_PASSWORD_RATE_LIMIT_WINDOW` (default: 15m) |
| **POST /api/v1/auth/resend-verification** | Per IP + Email | 3 requests | `RESEND_VERIFICATION_RATE_LIMIT` | `RESEND_VERIFICATION_RATE_LIMIT_WINDOW` (default: 15m) |
| **POST /api/v1/auth/verify-email** | Per IP | Shared global limiter | `RATE_LIMIT` | `RATE_LIMIT_WINDOW` (default: 10 requests / 1h) |
| **POST /api/v1/auth/reset-password** | Per IP | Shared global limiter | `RATE_LIMIT` | `RATE_LIMIT_WINDOW` (default: 10 requests / 1h) |
| **POST /api/v1/auth/refresh** | Global | Shared global limiter | `RATE_LIMIT` | `RATE_LIMIT_WINDOW` (default: 10 requests / 1h) |
| **POST /api/v1/auth/logout** | Global | Shared global limiter | `RATE_LIMIT` | `RATE_LIMIT_WINDOW` (default: 10 requests / 1h) |

### Rate Limit Response

When rate limit is exceeded, the API responds with HTTP `429 Too Many Requests`:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "too many requests, please try again later"
  }
}
```

Response headers include:

```http
X-RateLimit-Remaining: 0
```

### Example: Login Rate Limit

```bash
# First 5 requests within 5 minutes — success
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "wrong"}'
# HTTP 401 (invalid credentials, but request counted)

# 6th request within the same 5-minute window — rejected
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "wrong"}'
# HTTP 429 Too Many Requests
# {
#   "error": {
#     "code": "RATE_LIMITED",
#     "message": "too many requests, please try again later"
#   }
# }
```

---

## Error Response Format

All API errors are returned in a consistent format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "human-readable description"
  }
}
```

Validation errors include per-field details:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "validation failed",
    "details": [
      { "field": "username", "message": "must contain only letters, digits, dots, dashes, or underscores" },
      { "field": "phone",    "message": "must start with +" }
    ]
  }
}
```

### Error Codes Reference

| Code | HTTP Status | Description |
|---|---|---|
| `VALIDATION_ERROR` | 422 | One or more fields failed validation |
| `BAD_REQUEST` | 400 | Malformed request body |
| `INVALID_CREDENTIALS` | 401 | Wrong email or password |
| `INVALID_REFRESH_TOKEN` | 401 | Refresh token is missing, expired, or revoked |
| `EMAIL_NOT_VERIFIED` | 403 | Login attempted before email verification |
| `EMAIL_ALREADY_VERIFIED` | 409 | Verification re-attempted on an already-verified account |
| `USER_ALREADY_EXISTS` | 409 | Email is already registered |
| `USERNAME_ALREADY_EXISTS` | 409 | Username is already taken |
| `INVALID_VERIFICATION_TOKEN` | 400 | Email verification token is invalid or expired |
| `FORBIDDEN` | 403 | Insufficient role or permission |
| `USER_NOT_FOUND` | 404 | User does not exist |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_SERVER_ERROR` | 500 | Unexpected server error |