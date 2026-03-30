# Frontend build stage
FROM node:20-alpine AS frontend-builder

RUN corepack enable && corepack prepare pnpm@10.28.0 --activate

WORKDIR /app/frontend

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build-only

# Backend build stage
FROM golang:1.25.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app/backend

# Copy go mod files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy source code
COPY backend/ ./
COPY --from=frontend-builder /app/frontend/dist ./internal/frontend/dist

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o cornerstone \
    ./cmd/server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/backend/cornerstone .

# Copy config example (optional)
COPY --from=builder /app/backend/.env.example .env.example

# Change ownership
RUN chown -R app:app /app

# Switch to non-root user
USER app

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./cornerstone"]
