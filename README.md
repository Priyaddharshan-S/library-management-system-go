# Library Management System

A simplified library management system built step by step: Go + Gin API,
PostgreSQL, JWT auth with two roles (Admin, Member), email notifications
for reservations, and a plain HTML/CSS/JS frontend — no frameworks, no
build step.

## Stack

- **Backend:** Go + Gin
- **Database:** PostgreSQL
- **Frontend:** vanilla HTML/CSS/JavaScript (`frontend/index.html`)
- **Auth:** JWT, roles `admin` and `member`
- **Email:** pluggable `notification.Sender` interface — console output by
  default, real SMTP (Gmail, Mailtrap, etc.) when configured
- **Architecture:** layered — Controller → Service → Repository → Postgres

## Project layout

```
library-system/
├── cmd/api/main.go              # entrypoint: wiring + routes
├── internal/
│   ├── config/                  # env var loading
│   ├── models/                  # User, Book, Loan, Reservation, ReportSummary
│   ├── repository/               # Postgres data access (+ tests)
│   ├── service/                  # business logic (+ tests)
│   ├── controller/               # HTTP handlers (Gin)
│   ├── middleware/                # JWT auth + role checks, CORS
│   └── notification/              # email sender interface + implementations
├── frontend/index.html          # the entire UI, one file
├── migrations/                  # SQL, run automatically by Postgres on first start
├── docker-compose.yml           # Postgres only (recommended dev setup)
├── docker-compose.full.yml      # optional: API + Postgres both in Docker
└── Dockerfile                   # used by docker-compose.full.yml
```

## Setup

### 1. Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)

### 2. Configure environment

```
cp .env.example .env
```

Edit `.env` if needed. Notably:
- `DATABASE_URL` — the port must match whatever `docker-compose.yml` maps
  Postgres to on your machine (default `5432`; change both if that port is
  already taken on your PC — see Troubleshooting).
- `SMTP_HOST` — leave blank to print reservation-ready emails to the
  console instead of sending them. Fill in real SMTP details to send
  actual email.

### 3. Start Postgres

```
docker compose up -d
```

This also runs every file in `migrations/` automatically the first time,
creating all tables **and seeding one admin account** (see below).

### 4. Run the API

```
go mod tidy
go run ./cmd/api
```

You should see `Connected to database successfully` and `Starting server
on port 8080`.

### 5. Open the frontend

Just open `frontend/index.html` directly in a browser (double-click it, or
`start frontend/index.html` on Windows Git Bash). It's a static file — no
server needed. It talks to the API at `http://localhost:8080` by default;
change the `API_BASE` constant near the top of the `<script>` block if
your API runs elsewhere.

### 6. Log in

A default admin account is seeded automatically:

```
Email:    admin@library.local
Password: admin123
```

**Change this password (or remove `migrations/005_seed_admin.sql`) before
using this anywhere beyond your own local testing** — this password hash
is public, in this repo.

Register additional accounts through the frontend, or via:
```
curl -X POST http://localhost:8080/auth/register -H "Content-Type: application/json" -d "{\"name\":\"Jane\",\"email\":\"jane@example.com\",\"password\":\"secret123\"}"
```
New registrations are always `member` role. Promote someone to admin via:
```
docker exec -it <your-db-container-name> psql -U library_user -d library_db -c "UPDATE users SET role='admin' WHERE email='jane@example.com';"
```

## Running tests

**Unit tests** (fast, no database needed):
```
go test ./internal/service/...
```

**Integration tests** (borrow, return, and the concurrent-borrow race
condition — these need a real, running Postgres):
```
DATABASE_URL="postgres://library_user:library_pass@localhost:5432/library_db?sslmode=disable" go test ./internal/repository/...
```
(use whatever port your `docker-compose.yml` actually maps)

Run everything at once:
```
DATABASE_URL="postgres://library_user:library_pass@localhost:5432/library_db?sslmode=disable" go test ./...
```

If `DATABASE_URL` isn't set, the integration tests skip themselves cleanly
rather than failing — so `go test ./...` always works, even without
Postgres running.

## API overview

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/auth/register` | — | always creates a `member` |
| POST | `/auth/login` | — | returns a JWT |
| GET | `/me` | any | whoami, proves the token works |
| GET | `/books` | any | supports `?q=searchterm` |
| GET | `/books/:id` | any | |
| POST/PUT/DELETE | `/books...` | admin | |
| POST | `/loans/borrow` | admin | `{book_id, user_id}` — circulation desk checkout |
| POST | `/loans/:id/return` | admin | triggers reservation email if someone's waiting |
| GET | `/loans/active`, `/loans/overdue` | admin | |
| POST | `/reservations` | any | reserves for the logged-in user |
| DELETE | `/reservations/:id` | any (own) / admin (any) | |
| GET | `/reservations/mine` | any | |
| GET | `/reservations/book/:bookId` | admin | full queue for a book |
| GET | `/users`, `/users/:id`, `/users/:id/loans` | admin | patrons + loan history |
| GET | `/reports/summary` | admin | totals + top 5 borrowed |

## Optional: fully-Dockerized run (deployment sanity check)

Everything above runs Postgres in Docker and the API natively — the
recommended setup for development. If you want to check the whole thing
also works fully containerized (API included):

```
docker compose -f docker-compose.full.yml up --build
```

This uses different ports (`5434` for Postgres, `8081` for the API) so it
won't conflict with your normal dev setup. If you use this, update the
frontend's `API_BASE` to `http://localhost:8081`.

## Troubleshooting

- **Port 5432 already in use** — you likely have a native Postgres
  install running. Either stop that service, or change the port mapping
  in `docker-compose.yml` (e.g. `"5433:5432"`) and update `DATABASE_URL`
  in `.env` to match.
- **`JWT_SECRET is required but was not set`** — copy `.env.example` to
  `.env` (step 2 above).
- **Frontend shows "Could not reach the server"** — the API isn't
  running, or `API_BASE` in `index.html` doesn't match the port you're
  running it on.
- **Frontend loads but every request fails / CORS error in browser
  console** — confirm `cmd/api/main.go` has the CORS middleware (search
  for `Access-Control-Allow-Origin`).
- **Copied new phase files but old code still runs** — check for
  accidentally nested folders (e.g. `cmd/cmd/api/main.go`) from a bad
  copy/drag-and-drop:
  ```
  find . -name main.go
  ```
  Should show exactly one path.
- **Changed a migration file but it's not taking effect** — migrations
  only run once, on an empty database volume. Wipe and restart:
  ```
  docker compose down -v
  docker compose up -d
  ```
  (this deletes all data — fine for development, never do this in
  production)
