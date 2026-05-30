# Scalable Event Booking API

A high-concurrency event ticketing system built with Go, PostgreSQL, and Redis.

- **Concurrency Control**: Database transactions with optimistic/pessimistic locking
- **Virtual Waiting Room**: Rate limiting and queue management
- **Caching**: Redis-based caching for event details
- **Booking Lifecycle**: Reserve → Purchase → Release with automatic timeout

## Tech Stack

- **Backend**: Go (Golang)
- **Database**: PostgreSQL
- **Cache/Queue**: Redis

## Features

### Core Features

- User authentication with JWT
- Role-based access control (Admin/User)
- Event and venue management
- Seat booking with concurrency control
- Booking lifecycle: Reserve (10min) → Purchase → Release

### Advanced Features

- **Asynchronous Processing**: Event-driven architecture with Kafka command consumers for handling 50K+ concurrent requests
- **Domain Event Consumers**: Kafka booking event consumers update cache and broadcast seat changes across instances
- **Idempotency**: Prevents duplicate charges with idempotency keys (critical for financial systems)
- **Real-Time Updates**: WebSocket server for live seat availability updates
- **Database Scaling**: Read replica support for horizontal read scaling
- **Event-Aware Admission Control**: Per-event and per-user admission limits smooth hot ticket drops before they hit workers
- Database transactions with row-level locking
- Redis caching for event data
- Rate limiting for high-traffic scenarios
- Virtual waiting room for ticket drops

### Quick Start with Docker

The easiest way to run the application is using Docker:

```bash
# Build and start all services
make up

# Run database migrations (first time only)
make migrate

# View logs
make logs
```

The API will be available at `http://localhost:8080`.

**Available Make commands:**

- `make up` - Start all services
- `make down` - Stop all services
- `make migrate` - Run database migrations
- `make logs` - View logs
- `make clean` - Remove containers and volumes
- `make shell-db` - Open psql shell
- `make help` - Show all commands

### Manual Installation

If you prefer running without Docker:

1. Clone the repository
2. Install dependencies:

```bash
go mod download
```

1. Set up environment variables:

```bash
cp .env.example .env
# Edit .env with your database and Redis credentials
```

1. Start PostgreSQL and Redis locally

2. Run database migrations:

```bash
go run cmd/migrate/main.go
```

1. Start the server:

```bash
go run cmd/server/main.go
```

## Concurrency Control

The system uses PostgreSQL's `SELECT FOR UPDATE` with `NOWAIT` to prevent double-booking. When a user reserves a seat, the row is locked until the transaction completes, ensuring atomicity.

**Asynchronous Booking Processing**: Uses Kafka command topics as a message queue to handle traffic spikes. API returns HTTP 202 (Accepted) immediately while background workers process requests.

**Booking Event Consumers**: Consumes `booking-events` with consumer group `booking-event-consumers`, routes invalid events to `KAFKA_BOOKING_EVENTS_DLQ_TOPIC`, invalidates event cache, and broadcasts seat updates.

**Idempotency for Payments**: Middleware prevents duplicate charges when network requests fail. Uses idempotency keys stored in Redis with 24-hour TTL.

**Real-Time Seat Availability**: WebSocket server broadcasts seat status changes to all connected clients, eliminating the need for polling.

**Database Read Replicas**: Supports read/write separation with automatic failover. Read queries use replicas, writes use primary database.

**Event-Aware Admission Control**: Reservation requests are admitted by event ID using Redis-backed sliding windows. Configure with `BOOKING_ADMISSION_WINDOW_SEC`, `BOOKING_ADMISSION_EVENT_LIMIT`, and `BOOKING_ADMISSION_CLIENT_LIMIT`.

## Railway Deployment with Kafka

The Docker image is Railway-ready and listens on the `PORT` variable. It also supports Railway-style service URLs:

- `DATABASE_URL` or `POSTGRES_URL` for PostgreSQL
- `REDIS_URL` for Redis
- `KAFKA_URL` for private Railway Kafka access, with `KAFKA_BROKERS` still supported for local Docker

Recommended Railway variables:

```bash
RUN_MIGRATIONS_ON_START=true
DATABASE_URL=${{Postgres.DATABASE_URL}}
REDIS_URL=${{Redis.REDIS_URL}}
KAFKA_URL=${{Kafka.KAFKA_URL}}
KAFKA_TOPIC_PARTITIONS=6
KAFKA_TOPIC_REPLICATION_FACTOR=1
KAFKA_REQUIRED_ACKS=leader
```

The service creates missing Kafka topics on startup for command, event, and DLQ topics. For Railway's single-broker Kafka templates, keep `KAFKA_TOPIC_REPLICATION_FACTOR=1`.

If Railway logs show `relation "bookings" does not exist`, migrations did not run against the configured database. Set `RUN_MIGRATIONS_ON_START=true` and redeploy. The app also auto-runs migrations when Railway system variables are present unless `RUN_MIGRATIONS_ON_START=false` is set.

If logs show `Kafka command queue not configured`, the app did not receive a Kafka broker variable. Add `KAFKA_URL=${{Kafka.KAFKA_URL}}` to the API service variables; use `KAFKA_PUBLIC_URL` only for external clients, not app-to-Kafka traffic inside Railway.

If Railway reports a healthcheck failure, inspect the deployment logs before the first `Server starting on :$PORT` line. A fatal migration or database connection error exits the process before `/health` can respond. The healthcheck path should remain `/health`.
