# 📈 b3pulse

[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=guttosm_b3pulse&metric=coverage)](https://sonarcloud.io/summary/new_code?id=guttosm_b3pulse)
[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=guttosm_b3pulse&metric=bugs)](https://sonarcloud.io/summary/new_code?id=guttosm_b3pulse)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=guttosm_b3pulse&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=guttosm_b3pulse)

b3pulse is a Go service for ingesting B3 trade CSVs (stocks), persisting the relevant fields, and exposing a single REST endpoint that returns aggregates per ticker over a time window.

---

## 🚀 Features

- ⚡ Fast ingestion of the last 7 business days from CSV files (local directory)
- 🗃 PostgreSQL persistence with schema/migrations via Goose
- 🔎 Single REST endpoint with filters:
  - ticker (required)
  - data_inicio (optional, ISO-8601). If omitted, consider the last 7 days (ending yesterday)
- 📊 Aggregates returned:
  - max_range_value: maximum unit price (PrecoNegocio) over the filtered period
  - max_daily_volume: maximum total quantity traded in a single day for the ticker
- 📖 Swagger API docs (dev)
- 🩺 Health/readiness endpoints (as applicable)
- 🧪 Tests with testify/sqlmock and optional Testcontainers

---

## 🧱 Stack

| Layer       | Tech                          |
|-------------|-------------------------------|
| Language    | Go 1.25                       |
| Web         | Gin                           |
| Database    | PostgreSQL                    |
| Migrations  | Goose                         |
| CI/CD       | GitHub Actions (optional)     |
| Testing     | go test + Testcontainers      |
| Docs        | Swagger via Swaggo            |

---

## 🧑‍💻 Getting Started

### 📦 Requirements

- Docker + Docker Compose
- Go 1.25+
- Make

### 🛠 Local Setup

```bash
# Create local volume folders
make setup

# Install Go dependencies and tools (swag, etc.)
make install

# Build and run the app and dependencies
make docker-up
```

Swagger docs: <http://localhost:8080/swagger/index.html>

---

### 🧪 Running Tests

```bash
# Run unit + integration tests
make test

# Open HTML coverage report
make coverage-html
```

---

### 🧹 Development Tasks

```bash
make lint           # Run golangci-lint
make fmt            # Format code
make tidy           # Clean up go.mod/go.sum
make swagger        # Generate Swagger docs
make build          # Compile binary
make clean          # Remove binary + coverage
```

---

### 🐳 Docker Commands

```bash
make docker-up        # Start all services
make docker-down      # Stop all containers
make docker-restart   # Rebuild and restart everything
```

---

### 🗃 Run DB Migrations

```bash
make migrate
```

Migrations are applied using the Goose container against the local Postgres.

---

## ▶️ Running Locally

Prefer Makefile targets if aligned with your paths. If "run-api" or "ingest" fails due to path mismatch, use the direct commands below.

```bash
# API
# Preferred (if Makefile already points to ./cmd/main.go):
make run-api
# Fallback:
go run ./cmd/main.go --mode=api --port=8080

# Ingestion (requires CSVs under ./data/input)
# Preferred (Docker Compose + Make):
make docker-api-up   # ensures DB + migrations are ready
make docker-ingest   # runs one-off ingestion (7 business days)

# Local fallback (no Docker for app):
go run ./cmd/main.go --mode=ingest --dir=./data/input --days=7 --parallel=7

# Force reingestion (delete and re-insert for days)
go run ./cmd/main.go --mode=ingest --dir=./data/input --days=7 --parallel=7 --force
```

---

## 📁 Project Structure

```text
b3pulse/
├── cmd/
│   ├── main.go                # Entrypoint (API/ingestion via flags)
├── internal/
│   ├── api/
│   │   ├── handler.go         # HTTP handlers
│   │   └── router.go          # Router setup
│   ├── app/
│   │   └── app.go             # Application wiring/orchestration
│   ├── ingestion/             # CSV ingestion & parsing
│   ├── middleware/            # Middlewares (if any)
│   └── storage/
│       └── trade_repo.go      # Database access layer
├── config/
│   └── config.go              # Configuration (Viper/env)
├── data/
├── db/
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
└── README.md
```

---

## 📡 API Endpoint (Core)

| Method | Path                       | Description                                              |
|--------|----------------------------|----------------------------------------------------------|
| GET    | /api/v1/aggregate          | Aggregates for a ticker with optional start date filter  |
| GET    | /healthz                   | Liveness probe (registered in app wiring)                |
| GET    | /readyz                    | Readiness probe (DB; registered in app wiring)          |

Example request:

```http
GET /api/v1/aggregate?ticker=PETR4&data_inicio=2024-09-01
```

Example response:

```json
{
  "ticker": "PETR4",
  "max_range_value": 20.50,
  "max_daily_volume": 150000
}
```

- ticker: required
- data_inicio: optional (ISO-8601). If omitted, consider the last 7 business days ending yesterday.

Curl example:

```bash
curl -s "http://localhost:8080/api/v1/aggregate?ticker=PETR4&data_inicio=2025-09-11" | jq .
```

Swagger UI:

- <http://localhost:8080/swagger/index.html>

---

## 🔌 Ports and Troubleshooting

Default ports:

- API: 8080
- Postgres: 5432

Check if a port is in use:

```bash
lsof -i :8080
```

Kill the process using the port (use with care):

```bash
kill -9 <PID>
```

Docker Compose quick start (Stone team evaluators):

```bash
make docker-api-up      # starts Postgres, runs migrations, starts API
make docker-ingest      # runs ingestion one-off using ./data files
make docker-down        # stops everything
```

---

## 🧪 Quality Checks

```bash
make vet           # Run go vet static analysis
make staticcheck   # Run staticcheck (auto-installs if missing)
make lint          # Run golangci-lint
make analyze       # Run all static analysis tools above
```

---

## 📄 License

MIT © 2025 Gustavo Moraes
