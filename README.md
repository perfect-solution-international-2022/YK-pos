# PSI POS

PSI POS is a point-of-sale system made up of two applications:

- `backend`: Go/Fiber REST API with PostgreSQL and Redis.
- `frontend`: Next.js/React local-first POS terminal.

The frontend runs at `http://localhost:3000` and the backend API runs at
`http://localhost:8080`.

## Prerequisites

Install the following before starting:

- Go 1.25 or newer (the backend also declares a Go 1.26.5 toolchain).
- Node.js 18 or newer and npm.
- PostgreSQL 12 or newer.
- Redis 6 or newer when `REDIS_ENABLED=true` (the default in the backend
  example configuration).

There is currently no repository-level Docker Compose file, so PostgreSQL
and Redis must be started separately. The example configuration expects:

| Service | Host | Port | Database/user defaults |
| --- | --- | --- | --- |
| PostgreSQL | `localhost` | `5433` | database `pos`, user `root`, password `1234` |
| Redis | `localhost` | `6380` | database `0`, no password |

Change these values in `backend/.env` if your local services use different
credentials or ports.

## First-time setup

Open two PowerShell terminals from this workspace (`D:\Project\Office\psi pos`).

### 1. Configure and start the backend

```powershell
cd .\backend
Copy-Item .env.example .env
go mod download
go run .\cmd\migrate up
go run .\cmd\api
```

The API listens on `http://localhost:8080`. Check it with:

```powershell
Invoke-WebRequest http://localhost:8080/health
Invoke-WebRequest http://localhost:8080/health/ready
```

`/health` checks that the process is running. `/health/ready` also requires
the configured PostgreSQL and Redis services to be reachable.

The backend loads `.env` automatically. For anything other than local
development, replace the example JWT secrets with long, randomly generated
values and do not commit `.env`.

### 2. Configure and start the frontend

In the second terminal:

```powershell
cd .\frontend
Copy-Item .env.example .env.local
npm ci
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

The supplied `.env.local` uses the real backend:

```dotenv
NEXT_PUBLIC_USE_MOCK_API=false
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

To run the frontend without PostgreSQL, Redis, or the Go API, set
`NEXT_PUBLIC_USE_MOCK_API=true` and start only the frontend. The mock API is
intended for local UI development.

## Common commands

Backend commands (run inside `backend`):

```powershell
go run .\cmd\api                 # start the API
go run .\cmd\migrate status      # show migration status
go run .\cmd\migrate up          # apply migrations
go test -short -race ./...        # unit tests
go vet ./...                      # static analysis
go build ./...                    # compile all packages
```

Frontend commands (run inside `frontend`):

```powershell
npm run dev       # development server
npm run build     # production build
npm run start     # serve the production build
npm run lint      # ESLint
npm run test      # Vitest test suite
npm run test:watch
```

## How the applications work together

The frontend stores products, carts, and pending orders locally in IndexedDB
through Dexie. It can continue operating offline. When the real API is
enabled, its HTTP client uses `NEXT_PUBLIC_API_BASE_URL`; the background sync
process pushes pending changes and pulls server data when connectivity returns.

The backend persists shared business, product, inventory, user, and order data
in PostgreSQL and uses Redis for token denylisting and rate limiting when
enabled. Database schema changes are versioned under
`backend/database/migrations`.

## Project layout

```text
psi pos/
├── backend/         # Go API, migrations, tests, API documentation
├── frontend/        # Next.js app, IndexedDB layer, UI, tests
└── README.md        # this workspace startup guide
```

The two child directories currently retain their own Git repositories. Run
Git commands from the relevant child directory unless you intentionally
configure a parent monorepo.

## Troubleshooting

- If `/health` works but `/health/ready` fails, verify PostgreSQL and Redis
  host/port/credentials in `backend/.env`.
- If the browser reports a CORS error, ensure
  `CORS_ALLOWED_ORIGINS=http://localhost:3000` is present in the backend
  configuration and restart the API.
- If the frontend is using mock data unexpectedly, set
  `NEXT_PUBLIC_USE_MOCK_API=false` in `frontend/.env.local` and restart
  `npm run dev`.
- If npm scripts fail because dependencies are missing, remove the frontend's
  ignored `node_modules` directory and run `npm ci` again.
