# TASKS.md

## Goal

Refactor and improve `main.go` and related infrastructure while preserving existing functionality.

The application is a Go REST API using:

* Cobra CLI
* PostgreSQL
* pgxpool
* golang-migrate
* layered architecture (repositories → services → handlers)
* graceful shutdown

Do not introduce breaking API changes.

---

# Task 1: Fix PostgreSQL pool shutdown

## Problem

The PostgreSQL connection pool is closed twice:

```go
defer pool.Close()

...

pool.Close()
```

## Requirements

* Ensure the pool is closed exactly once.
* Keep graceful shutdown behavior unchanged.
* Avoid duplicate cleanup logic.

## Acceptance Criteria

* `pool.Close()` is called only once.
* Application shuts down cleanly.

---

# Task 2: Improve application lifecycle management

## Problem

A root context is created but is not propagated through the application lifecycle.

```go
ctx, cancel := context.WithCancel(context.Background())
```

## Requirements

* Use a root context for application lifetime.
* Propagate it where appropriate.
* Call `cancel()` during shutdown.
* Prepare architecture for future background workers.

## Acceptance Criteria

* Root context controls application lifetime.
* Shutdown cancels the root context.

---

# Task 3: Add HTTP server timeouts

## Problem

HTTP server has no timeouts configured.

Current:

```go
srv := &http.Server{
    Addr: ":" + cfg.AppPort,
    Handler: router,
}
```

## Requirements

Add:

```go
ReadTimeout
ReadHeaderTimeout
WriteTimeout
IdleTimeout
```

Use reasonable production-safe defaults.

Suggested:

```go
ReadTimeout:       10s
ReadHeaderTimeout: 5s
WriteTimeout:      30s
IdleTimeout:       60s
```

## Acceptance Criteria

* HTTP server contains timeout configuration.
* Application compiles and works normally.

---

# Task 4: Replace hardcoded SSL mode

## Problem

Database migration connection string uses:

```go
sslmode=disable
```

## Requirements

Add configuration field:

```go
DBSSLMode
```

Load from environment.

Examples:

```env
DB_SSL_MODE=disable
DB_SSL_MODE=require
DB_SSL_MODE=verify-full
```

## Acceptance Criteria

* SSL mode is configurable.
* No hardcoded sslmode values remain.

---

# Task 5: Improve migration rollback safety

## Problem

Current implementation:

```go
m.Down()
```

This rolls back ALL migrations.

## Requirements

Implement rollback by steps.

Example:

```bash
app migrate down --steps 1
app migrate down --steps 3
```

Use:

```go
m.Steps(-steps)
```

Provide validation.

## Acceptance Criteria

* Default rollback is 1 migration.
* User may specify custom number of steps.
* Full rollback requires explicit action.

---

# Task 6: Add safe full rollback command

## Requirements

Create separate command:

```bash
app migrate reset
```

This command may execute:

```go
m.Down()
```

Require explicit confirmation flag:

```bash
app migrate reset --force
```

Without force:

```text
refuse execution
```

## Acceptance Criteria

* Accidental full rollback is impossible.

---

# Task 7: Improve server startup error handling

## Problem

Server startup errors occur inside a goroutine.

Current:

```go
go func() {
    if err := srv.ListenAndServe(); err != nil {
        ...
    }
}()
```

## Requirements

Introduce error channel.

Example:

```go
errCh := make(chan error, 1)
```

Handle:

* startup errors
* shutdown signal

using:

```go
select
```

## Acceptance Criteria

* Startup failures are reported cleanly.
* No fatal exits hidden inside goroutines.

---

# Task 8: Add signal cleanup

## Requirements

After:

```go
signal.Notify(...)
```

add:

```go
defer signal.Stop(...)
```

## Acceptance Criteria

* Signal handlers are properly released.

---

# Task 9: Make logger initialization explicit

## Problem

Logger initialization cannot return an error.

## Requirements

Review logger package.

If initialization can fail:

```go
func Init() error
```

Handle the error properly.

If initialization cannot fail:

* document why.

## Acceptance Criteria

* Logger initialization behavior is explicit.

---

# Task 10: Improve migration file creation safety

## Problem

Migration creation is not atomic.

Example:

* up migration created
* down migration creation fails

Result:

* inconsistent migration state

## Requirements

If second file creation fails:

* remove first file
* return error

## Acceptance Criteria

* Migration creation is atomic.

---

# Task 11: Add configuration validation

## Requirements

Validate at startup:

* AppPort
* DBHost
* DBPort
* DBUser
* DBName
* JWTSecret

Application should fail fast with clear errors.

## Acceptance Criteria

* Invalid configuration is detected before startup.

---

# Task 12: Add structured startup logging

## Requirements

Log:

* application version
* environment
* port
* database host

Example:

```text
service=api
version=1.0.0
env=production
port=8080
```

## Acceptance Criteria

* Startup logs provide operational visibility.

---

# Task 13: Optional advanced improvement

## Requirements

Evaluate replacing manual lifecycle management with:

```go
golang.org/x/sync/errgroup
```

or

```go
oklog/run
```

Only implement if it simplifies code.

Do not over-engineer.

---

# Constraints

* Preserve existing architecture.
* Preserve existing REST API behavior.
* Preserve existing CLI commands.
* Keep code idiomatic Go.
* Follow Go best practices.
* Run `go fmt`.
* Ensure project builds successfully after refactor.

---

# Deliverables

Provide:

1. Modified source code.
2. Summary of all changes.
3. Any new environment variables.
4. Migration guide if behavior changed.
