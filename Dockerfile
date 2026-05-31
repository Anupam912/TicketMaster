# Build stage — Go 1.26+ patches stdlib CVEs flagged in gobinary scans (see Trivy gobinary/stdlib)
FROM golang:1.26-alpine3.22 AS builder

WORKDIR /app

RUN apk add --no-cache git && apk upgrade --no-cache

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o migrate ./cmd/migrate

# Runtime stage — minimal Alpine with patched packages (builder toolchain not included)
FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && \
    apk upgrade --no-cache && \
    rm -rf /var/cache/apk/*

RUN addgroup -g 1001 appgroup && \
    adduser -u 1001 -G appgroup -D appuser

COPY --from=builder /app/server .
COPY --from=builder /app/migrate .

# Copy migration files
COPY --from=builder /app/internal/database/migrations.sql ./internal/database/

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

CMD ["sh", "-c", "if [ \"$RUN_MIGRATIONS_ON_START\" = \"true\" ]; then ./migrate; fi; exec ./server"]
