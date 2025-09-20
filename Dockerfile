# ───────────────────────────────────────────────────────────────
# Builder
# ───────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# Cache go mod
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Generate Swagger docs inside the build context so the binary can import docs
RUN go run github.com/swaggo/swag/cmd/swag@latest init -g ./cmd/main.go --parseDependency --parseInternal

# Build binary
RUN go build -o b3pulse ./cmd/main.go

# ───────────────────────────────────────────────────────────────
# Runtime
# ───────────────────────────────────────────────────────────────
FROM alpine:3.22

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/b3pulse .

# Metadata
LABEL org.opencontainers.image.title="b3pulse" \
      org.opencontainers.image.description="B3 trade ingestion & aggregation service" \
      org.opencontainers.image.authors="Gustavo Moraes"

EXPOSE 8080

# Entrypoint always runs the binary
ENTRYPOINT ["./b3pulse"]

# Default: run API mode, can be overridden with --mode=ingest
CMD ["--mode=api", "--port=8080"]
