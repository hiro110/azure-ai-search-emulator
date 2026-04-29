# Build stage — CGO requires gcc
FROM golang:1.26-bullseye AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /app/server .

# Runtime stage — needs libc (not scratch) because of CGO/sqlite3
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/server .

RUN mkdir -p /data

ENV PORT=8080
ENV DB_PATH=/data/data.db

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:${PORT}/healthz || exit 1

ENTRYPOINT ["/app/server"]
