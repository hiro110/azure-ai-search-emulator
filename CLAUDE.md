# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Azure AI Search Emulator is a Go server that emulates the Azure Cognitive Search REST API locally using Gin (HTTP) and SQLite (persistence). It exists for development/testing without requiring an Azure subscription.

## Commands

All Go commands should be run via `mise exec -- go` since Go is managed via mise.

```bash
# Run (direct)
mise exec -- go run main.go

# Run with hot reload
air

# Test
mise exec -- go test -v -race -coverprofile=coverage.out ./...
mise exec -- go tool cover -func=coverage.out

# Lint
golangci-lint run

# Build
mise exec -- go build -o emulator .

# Dependencies
mise exec -- go mod tidy
```

**Environment variables required at runtime:**
- `API_KEY` — required; all endpoints except `/healthz` reject requests without it
- `PORT` — default `8080`
- `DB_PATH` — default `./data.db`

## Architecture

The project uses Domain-Driven Design (DDD) with four strict layers. Dependencies flow inward only: `api → application → domain ← infrastructure`.

```
main.go                     # DI wiring + server startup (no logic)
api/handler.go              # Gin routes + HTTP handlers
application/
  index_service.go          # Business logic for index CRUD
  document_service.go       # Business logic for document CRUD + search
  app_services.go           # Service container (aggregates services)
domain/
  index.go                  # Index entity + IndexRepository interface + ErrIndexNotFound
  document.go               # Document entity + DocumentRepository interface + ErrDocumentNotFound
infrastructure/
  sqlite_index_repository.go     # Implements IndexRepository via SQLite
  sqlite_document_repository.go  # Implements DocumentRepository via SQLite
```

**Data storage model:** Both the index schema and document content are stored as raw JSON strings in SQLite — no column normalization. Only specific fields (e.g., the key field name) are parsed on demand.

**Full-text search implementation:** Case-insensitive substring matching on the document content JSON string. No ranking or field-specific queries.

**Batch operations:** The `/docs/index` endpoint supports Azure-style actions (`upload`, `merge`, `mergeOrUpload`, `delete`). `merge` performs a deep merge of JSON fields. Batch operations are NOT transactional — each document result is independent.

## Key Patterns

**Adding a new endpoint:** Add the route + handler in `api/handler.go`, add the business method to the relevant service in `application/`, add any new repository methods to the interface in `domain/` and implement in `infrastructure/`.

**Error handling convention:** Domain errors (`ErrIndexNotFound`, `ErrDocumentNotFound`) are returned from services and converted to HTTP 404 in handlers. Validation errors return 400, conflicts return 409.

**Context parameter:** All service/repository methods accept `context.Context` but don't use it yet — do not remove it.

**Authentication middleware:** Applied to all routes registered after the middleware call in `main.go`. `/healthz` is registered before the middleware and is therefore public.

## API Routes Reference

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check (no auth) |
| POST/GET | `/indexes` | Create / list indexes |
| GET/PUT/DELETE | `/indexes/{name}` | Get / update / delete index |
| GET | `/indexes/{name}/stats` | Document count + storage size |
| POST | `/indexes/{name}/docs` | Add or update single document |
| GET | `/indexes/{name}/docs/{key}` | Get document by key |
| GET | `/indexes/{name}/docs/$count` | Document count |
| GET/POST | `/indexes/{name}/docs?search=` / `/docs/search` | Search documents |
| POST | `/indexes/{name}/docs/index` | Batch operations |

## Docker

```bash
docker compose up -d          # Recommended; uses named volume for SQLite persistence
docker compose down
```

The Docker image uses a multi-stage build (CGO enabled for SQLite). The runtime image is `debian:bullseye-slim` with a `/healthz` health check.
