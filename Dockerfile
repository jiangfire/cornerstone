# Build stage
FROM golang:1.26.4-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X github.com/jiangfire/cornerstone/internal/cli.Version=${VERSION}" \
    -trimpath \
    -tags=sqlite_omit_load_extension \
    -o cornerstone \
    ./cmd

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

WORKDIR /app

COPY --from=builder /app/cornerstone .

COPY --from=builder /app/.env.example .env.example

RUN chown -R app:app /app

USER app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ready || exit 1

CMD ["./cornerstone", "serve"]
