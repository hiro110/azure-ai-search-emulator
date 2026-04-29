# Architecture

## Directory Structure

```
├── internal/
│   ├── api/                # Gin routing and HTTP handlers
│   │   └── handler.go      # All HTTP handler functions
│   ├── application/        # Use case layer (services)
│   │   └── index_service.go
│   │   └── document_service.go
│   ├── domain/             # Domain layer (entities and repository interfaces)
│   │   └── index.go        # Index entity, IndexRepository interface, ErrIndexNotFound
│   │   └── document.go     # Document entity, DocumentRepository interface, ErrDocumentNotFound
│   └── infrastructure/     # Infrastructure layer (DB implementations)
│       └── sqlite_index_repository.go
│       └── sqlite_document_repository.go
├── main.go             # Entry point: DI wiring and server startup
├── docs/               # Documentation
│   └── architecture.md # This file
```

Packages under `internal/` are restricted by the Go compiler — they cannot be imported by external modules.

## Layer Responsibilities

- **internal/api/**
  Gin routing and HTTP handlers only. Handles request/response validation and DTO mapping.

- **internal/application/**
  Use case (service) layer. Orchestrates business logic and calls the persistence layer via repository interfaces.

- **internal/domain/**
  Domain models (entities, value objects) and repository interfaces. The core of all business rules.

- **internal/infrastructure/**
  Interaction with databases and external services. Implements the repository interfaces (e.g., SQLite).

- **main.go**
  Dependency injection, server startup, and route initialization. Contains no business logic.

## Dependency Flow

Dependencies flow inward only:

```
api → application → domain ← infrastructure
```

## Example: Creating an Index

1. `internal/api/handler.go` receives the HTTP request and passes the DTO to `internal/application/index_service.go`
2. `internal/application/index_service.go` executes use case logic (validation, duplicate check, etc.)
3. `internal/domain/index.go` repository interface is called, which delegates to `internal/infrastructure/sqlite_index_repository.go` for DB persistence
4. The result is returned as a response DTO

## Benefits

- `main.go` is minimal; responsibilities are clearly separated across layers
- Easy to test and extend
- Aligns with Domain-Driven Design (DDD) principles
