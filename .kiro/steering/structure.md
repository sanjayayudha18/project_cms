# Project Structure

## Layout

```
CMS/
├── frontend/                 # React + TypeScript + Vite
│   ├── src/
│   │   ├── components/       # Shared UI components
│   │   ├── features/         # Feature modules (forecasting, invoice, cash-count)
│   │   ├── lib/              # Utilities, API client, auth
│   │   ├── routes/           # TanStack Router file-based routes
│   │   └── styles/           # Design tokens, Tailwind config
│   ├── package.json
│   └── vite.config.ts
│
├── backend/                  # Go API + Worker
│   ├── cmd/
│   │   ├── api/              # HTTP server entrypoint
│   │   └── worker/           # asynq worker entrypoint
│   ├── internal/
│   │   ├── domain/           # Business entities, value objects
│   │   ├── service/          # Business logic (forecasting, invoice, cash-count)
│   │   ├── handler/          # HTTP handlers (Chi routes)
│   │   ├── repository/       # DB access (sqlc-generated + interfaces)
│   │   ├── middleware/       # Auth, logging, RBAC, rate-limit
│   │   ├── queue/            # asynq task definitions + handlers
│   │   └── pdf/              # Typst template rendering
│   ├── migrations/           # SQL migration files
│   ├── queries/              # sqlc SQL query files
│   ├── templates/            # Typst PDF templates
│   ├── go.mod
│   └── go.sum
│
├── docker/                   # Dockerfiles
│   ├── Dockerfile.api
│   ├── Dockerfile.frontend
│   └── Dockerfile.worker
│
├── docker-compose.yml        # Local dev orchestration
├── docker-compose.prod.yml   # Production overrides
├── Taskfile.yml              # Task runner config
├── .env.example              # Environment variables template
│
├── documents/                # Business requirements (existing)
├── archives/                 # Archived docs (existing)
└── .kiro/                    # Steering, agents, hooks (existing)
```

## Guidelines

- Group by feature (forecasting, invoice, cash-count) rather than by file type.
- Co-locate tests with source: `*_test.go` next to the file under test (Go convention).
- Frontend tests in `__tests__/` directories co-located with the feature.
- Keep the folder structure flat until complexity demands nesting.
- Update this file whenever new top-level directories are introduced.
