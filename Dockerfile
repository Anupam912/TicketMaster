# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Copy source code
COPY . .

# Download dependencies and update go.sum
RUN go mod tidy

# Build binaries
RUN CGO_ENABLED=0 go build -o server ./cmd/server
RUN CGO_ENABLED=0 go build -o migrate ./cmd/migrate

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1001 appgroup && \
    adduser -u 1001 -G appgroup -D appuser

# Copy binaries from builder
COPY --from=builder /app/server .
COPY --from=builder /app/migrate .

# Copy migration files
COPY --from=builder /app/internal/database/migrations.sql ./internal/database/

# Set ownership
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

CMD ["./server"]
