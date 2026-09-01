# Project Structure

## Layout — Go Workspace

```
CMS2/
├── go.work                       # Go workspace: links backend/, backend-cit/, pkg/
│
├── pkg/                          # Shared infra (own go.mod, no dependency on either backend)
│   ├── auth/                     # JWT TokenService, blacklist, Provider/UserRepository interfaces
│   ├── middleware/               # RequireAuth, RequireRoles, rate limiter
│   ├── config/                   # Env config loader, Load(defaultPort)
│   ├── response/                 # {success,data} JSON envelope (used by backend-cit only)
│   └── database/                 # pgxpool: primary (write) + replica (read)
│
├── backend/                      # ATM backend — own go.mod, port 8080
│   ├── cmd/
│   │   └── api/main.go           # HTTP server entrypoint (bootstrap + route registration)
│   ├── internal/
│   │   ├── auth/                 # Login service (LDAP + local), issues JWT
│   │   ├── user/                 # User CRUD
│   │   ├── vendor/               # Vendor management
│   │   ├── vendorpic/            # Vendor PIC
│   │   ├── vault/                # Vault management
│   │   ├── location/             # Location/branch
│   │   ├── atm/                  # ATM master data
│   │   ├── assignment/           # Vendor-ATM assignments
│   │   ├── dsr/                  # Daily Status Report (ATM)
│   │   ├── replenishment/        # Replenishment instructions
│   │   ├── forecast/             # H+2 forecast
│   │   ├── invoice/              # Invoice upload + validate + approve
│   │   ├── reconciliation/       # ATM reconciliation
│   │   ├── audit/                # Audit log
│   │   ├── approval/             # Maker-checker
│   │   ├── document/             # Document storage
│   │   ├── notification/         # In-app + SMTP
│   │   └── export/               # CSV/XLSX/PDF export
│   ├── migrations/               # ALL DB migrations (ATM + CIT tables)
│   ├── go.mod
│   └── go.sum
│
├── backend-cit/                  # CIT backend — own go.mod, port 8081
│   ├── cmd/
│   │   └── api/main.go           # Health check + RequireAuth-protected routes; validates tokens, issues none
│   ├── internal/
│   │   ├── cit/                  # CIT orders, handover
│   │   ├── journal/              # CIT journals
│   │   ├── dsr/                  # CIT DSR uploads
│   │   ├── reconciliation/       # CIT reconciliation (order vs DSR vs journal)
│   │   └── integration/          # Corebanking escrow batch ingest
│   ├── go.mod
│   └── go.sum
│
├── frontend/
│   ├── CompanyPortal-Vite/       # Internal app (LDAP login)
│   │   ├── src/
│   │   │   ├── components/       # Shared UI components
│   │   │   ├── features/         # Feature modules
│   │   │   ├── lib/              # Utilities, API client, auth
│   │   │   ├── routes/           # TanStack Router file-based routes
│   │   │   └── styles/           # Design tokens, Tailwind config
│   │   ├── package.json
│   │   └── vite.config.ts
│   │
│   └── VendorPortal-Vite/        # Vendor portal (local login)
│       ├── src/
│       │   ├── components/
│       │   ├── features/
│       │   ├── lib/
│       │   ├── routes/
│       │   └── styles/
│       ├── package.json
│       └── vite.config.ts
│
├── docker-compose.yml            # Local dev: backend + backend-cit + redis (Postgres external)
├── backend/Dockerfile            # Multi-stage, builds from repo root (context .) for go.work
├── backend-cit/Dockerfile        # Multi-stage, builds from repo root (context .) for go.work
├── .env.example                  # Environment variables template
│
├── documents/                    # Business requirements (existing)
├── archives/                     # Archived docs (existing)
└── .kiro/                        # Steering, agents, hooks
```

## Key Architecture Rules

- `pkg/` depends on neither backend. `backend/` and `backend-cit/` depend on `pkg/` only — never on each other. Compiler-enforced acyclic.
- `backend/migrations/` is the sole owner of ALL DB migrations (including CIT tables).
- ATM backend keeps existing flat JSON response shape for wire compatibility. CIT backend uses `pkg/response` envelope.
- Each backend builds and deploys as an independent artifact (separate Compute Engine in prod).

## Guidelines

- Group by feature (forecasting, invoice, cit) rather than by file type.
- Co-locate tests with source: `*_test.go` next to the file under test (Go convention).
- Frontend tests in `__tests__/` directories co-located with the feature.
- Keep the folder structure flat until complexity demands nesting.
- Update this file whenever new top-level directories are introduced.
