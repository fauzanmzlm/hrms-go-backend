# hrms-go-backend

REST API for a government/enterprise HR and appraisal system, built with Clean Architecture.

Modules:

- **Modul Perkhidmatan** (service records): employee personnel data, service history and grades.
- **Modul Naik Pangkat & Tatatertib** (promotion and disciplinary): promotion applications and disciplinary records with a `PENDING` / `APPROVED` / `REJECTED` approval workflow.

## Stack

Go 1.27, `go-chi/chi` router, MySQL via `jmoiron/sqlx`, JWT (HS256) auth, bcrypt passwords, `log/slog` JSON logging.

## Layout

```
cmd/api/                      entrypoint; manual dependency injection and graceful shutdown
internal/config/              environment-based configuration
internal/domain/              entities, business rules, repository and usecase interfaces (stdlib only)
internal/repository/mysql/    sqlx implementations, transactor, migration runner
internal/usecase/             business logic
internal/delivery/http/       chi router
  handler/                    HTTP handlers, request DTOs, error -> status mapping
  middleware/                 JWT auth, role checks, request logging, panic recovery
pkg/jwt/                      token issue/verify
pkg/password/                 bcrypt hashing
pkg/logger/                   slog JSON logger setup
pkg/response/                 standard JSON success/error envelope
migrations/                   SQL schema, embedded into the binary
```

Dependencies point inward: delivery, usecase and repository all depend on `internal/domain`, and only `cmd/api` knows about every layer.

## Running

With Docker (MySQL and API):

```sh
make up            # API on :8080, admin / ChangeMe-2026
```

Against your own MySQL:

```sh
cp .env.example .env   # edit DB_DSN, JWT_SECRET, admin password
make run
```

With `AUTO_MIGRATE=true` the embedded migrations are applied on startup. A MySQL named lock prevents two instances from migrating at the same time. `make test` runs the unit tests, none of which need a database.

The server stops cleanly on SIGINT/SIGTERM. In-flight requests get `SHUTDOWN_TIMEOUT` to finish, then the database pool is closed.

## API

All responses are JSON. A successful response is `{"data": ...}`. Errors look like this:

```json
{"error": {"code": "VALIDATION_FAILED", "message": "validation failed",
           "fields": {"ic_number": "must be a 12-digit MyKad number"}, "request_id": "..."}}
```

| Status | Code | Meaning |
|---|---|---|
| 400 | `BAD_REQUEST` | Malformed JSON, unknown field, bad path/query parameter |
| 401 | `UNAUTHORIZED` | Missing/invalid token or bad credentials |
| 403 | `FORBIDDEN` | Role not allowed, or acting on someone else's record |
| 404 | `NOT_FOUND` | Resource does not exist |
| 409 | `CONFLICT` / `INVALID_STATUS_TRANSITION` | Duplicate, still referenced, or already reviewed |
| 422 | `VALIDATION_FAILED` / `RULE_VIOLATION` | Field errors, or a business rule blocks the action |
| 500 | `INTERNAL_ERROR` | Unexpected error (details only in the logs) |

Dates in request bodies use `YYYY-MM-DD`. List endpoints accept `page` and `page_size` (default 20, max 100) and return `{items, total, page, page_size}`.

Every route under `/api/v1` except login needs `Authorization: Bearer <token>`. Callers with the `EMPLOYEE` role can only see their own employee record, service history, promotions and disciplinary cases.

| Method and path | Roles |
|---|---|
| `POST /api/v1/auth/login` | public |
| `GET /api/v1/auth/me` | any |
| `POST /api/v1/users` | ADMIN |
| `GET /api/v1/grades`, `GET /grades/{id}` | any |
| `POST /grades`, `PUT /grades/{id}` | ADMIN, HR_OFFICER |
| `DELETE /grades/{id}` | ADMIN |
| `GET /employees` (`search`, `department`, `grade_id`, `status`) | ADMIN, HR_OFFICER, APPROVER |
| `GET /employees/{id}` | any (EMPLOYEE: own) |
| `POST /employees`, `PUT /employees/{id}` | ADMIN, HR_OFFICER |
| `DELETE /employees/{id}` | ADMIN |
| `GET /employees/{id}/service-records`, `GET /service-records/{id}` | any (EMPLOYEE: own) |
| `POST /employees/{id}/service-records`, `PUT`/`DELETE /service-records/{id}` | ADMIN, HR_OFFICER |
| `GET /promotions` (`employee_id`, `status`), `GET /promotions/{id}` | any (EMPLOYEE: own) |
| `POST /promotions` | ADMIN, HR_OFFICER, EMPLOYEE (for self) |
| `POST /promotions/{id}/review` | ADMIN, APPROVER |
| `GET /disciplinary-records` (`employee_id`, `status`, `category`), `GET /disciplinary-records/{id}` | any (EMPLOYEE: own) |
| `POST /disciplinary-records` | ADMIN, HR_OFFICER |
| `POST /disciplinary-records/{id}/review` | ADMIN, APPROVER |

`GET /healthz` checks that the process is alive, and `GET /readyz` also pings the database.

## Business rules

- **Service history.** An employee has at most one current service record (one with no `effective_to`). The database enforces this too. Adding a new current record closes the previous one the day before it starts, and copies the new grade, position and department onto the employee. `PUT /employees/{id}` never changes the grade; grades change only through service records and approved promotions.
- **Promotion.** Only an `ACTIVE` employee can apply. The proposed grade must have a higher `level` than the current grade. An employee can have only one `PENDING` application at a time. An approved disciplinary penalty within `PROMOTION_DISCIPLINARY_BLOCK_MONTHS` blocks both submitting and approving.
- **Approving a promotion** requires `effective_date`. In one transaction it marks the application `APPROVED`, closes the current service record, opens a `PROMOTION` record at the new grade, and updates the employee.
- **Reviews.** Only `PENDING` items can be reviewed. If two reviewers act at the same time, only one succeeds; the other gets 409. Rejecting requires `remarks`. Nobody can review their own promotion or disciplinary case, and the officer who reported a case cannot review it.
- **Disciplinary penalties.** `DEMOTION` and `DISMISSAL` are only allowed for `SERIOUS` cases. Approving a `DISMISSAL` sets the employee to `TERMINATED`.

Example workflow:

```sh
TOKEN=$(curl -s localhost:8080/api/v1/auth/login -d '{"username":"admin","password":"ChangeMe-2026"}' | jq -r .data.access_token)
curl -s localhost:8080/api/v1/grades -H "Authorization: Bearer $TOKEN" \
  -d '{"code":"N41","scheme":"N","level":41,"title":"Pegawai Tadbir"}'
```
