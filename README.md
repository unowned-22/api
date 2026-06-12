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

Create a new user account:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepassword123"
  }'
```

### Successful Response

```json
{
  "data": {
    "message": "user registered successfully"
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

### Successful Response

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

Use a valid refresh token to get a new access token:

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<PASTE_YOUR_REFRESH_TOKEN_HERE>"
  }'
```

### Successful Response

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
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

### Successful Response

```json
{
  "data": {
    "message": "logged out successfully"
  }
}
```

---

### 5. Get User Profile (Protected Route)

Use the JWT access token in the `Authorization` header:

```bash
curl -X GET http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer <PASTE_YOUR_ACCESS_TOKEN_HERE>"
```

### Successful Response

```json
{
  "data": {
    "id": 1,
    "email": "user@example.com",
    "role": "user",
    "created_at": "2026-06-12T09:53:01Z"
  }
}
```

---

### 6. Admin Ping (Admin Protected Route)

Use an access token for a user with the `admin` role:

```bash
curl -X GET http://localhost:8080/api/v1/admin/ping \
  -H "Authorization: Bearer <PASTE_ADMIN_ACCESS_TOKEN_HERE>"
```

### Successful Response

```json
{
  "data": {
    "message": "admin access granted"
  }
}
```

---

### 7. List Admin Permissions (Permission Protected Route)

Use an access token for a user whose role has the `admin.access` permission:

```bash
curl -X GET http://localhost:8080/api/v1/admin/permissions \
  -H "Authorization: Bearer <PASTE_ACCESS_TOKEN_HERE>"
```

### Successful Response

```json
{
  "data": {
    "permissions": [
      {
        "id": 1,
        "name": "admin.access",

      ### 8. Password Reset

      Initiate a password reset (silent response to prevent account enumeration):

      ```bash
      curl -X POST http://localhost:8080/api/v1/auth/forgot-password \
        -H "Content-Type: application/json" \
        -d '{"email": "user@example.com"}'
      ```

      Successful response (always 200):

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

      Successful response:

      ```json
      {
        "data": {
          "message": "password has been reset successfully"
        }
      }
      ```

      Notes:
      - The `forgot-password` endpoint intentionally returns a 200 OK regardless of whether the email exists to avoid leaking valid accounts.
      - Reset tokens expire (short-lived) and are marked used after a successful reset. All refresh tokens for the user are revoked on password change.
        "description": "Access administration endpoints",
        "created_at": "2026-06-12T09:53:01Z"
      }
    ]
  }
}
```

---

## Error Response Format

All API errors are returned in a consistent format.

Example for invalid credentials:

```json
{
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "invalid email or password"
  }
}
```

Example for invalid refresh token:

```json
{
  "error": {
    "code": "INVALID_REFRESH_TOKEN",
    "message": "refresh token is invalid"
  }
}
```
