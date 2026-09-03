# Contributing to FaultIQ

Thank you for your interest in contributing! FaultIQ is a graph-based fault analysis and automated remediation platform for distributed systems.

## Ways to Contribute

- **Bug reports** — open a GitHub issue with reproduction steps
- **Feature requests** — open a discussion describing the use case
- **Pull requests** — bug fixes, improvements, new features
- **Documentation** — improve docs, add examples, fix typos
- **SOP playbooks** — share your refined remediation steps for specific fault types

## Development Setup

### Prerequisites
- Go 1.24+
- Node 18+
- Docker + Docker Compose v2

### Quick start
```bash
git clone https://github.com/[your-org]/FaultIQ
cd FaultIQ
cp .env.example .env
# Edit .env with your values
docker compose up --build
```

### Running tests
```bash
# Go services
cd services/detection-engine && go test ./...
cd services/api-gateway && go test ./...
cd services/signal-ingestion && go test ./...

# Frontend TypeScript
cd frontend && npx tsc --noEmit

# End-to-end (requires running stack)
bash scripts/test-e2e.sh
```

## Code Style

### Go
- Standard `gofmt` formatting (enforced by CI)
- No external logging libraries — use stdlib `log`
- Errors must be handled — no silent `_` ignores on errors that can affect correctness
- New tables require a migration in `scripts/migrations/`

### TypeScript/React
- Standard ESLint rules
- Functional components with hooks
- RTK Query for all API calls — no raw `fetch` in components (except the demo page)

## Pull Request Process

1. Fork and create a branch from `main`
2. Make your changes with tests
3. Ensure all tests pass: `go test ./...` and `npx tsc --noEmit`
4. Update documentation if you're adding features
5. Open a PR with a clear description of what and why

## Adding a New Service

If adding a new Go microservice:
1. Create `services/your-service/` with `main.go` and `go.mod`
2. Add a `Dockerfile` following the existing non-root pattern
3. Add to `docker-compose.yml` with health check and resource limits
4. Add to `.gitignore` to exclude the compiled binary

## Database Changes

All schema changes must be backward-compatible (use `ADD COLUMN IF NOT EXISTS`, never `DROP COLUMN`). The 3 migration files (`001_core.sql`, `002_incidents.sql`, `003_analytics.sql`) represent the complete current schema — update them in place for new columns on existing tables.

## SOP Playbook Contributions

The `services/detection-engine/recommendations.go` contains built-in playbooks. To add better steps for a specific fault type, open a PR with:
- The fault type you're improving
- The new steps based on real incident experience
- Why these steps work better than the current ones

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
