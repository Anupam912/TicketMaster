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
- **Asynchronous Processing**: Event-driven architecture with Redis Streams for handling 50K+ concurrent requests
- **Idempotency**: Prevents duplicate charges with idempotency keys (critical for financial systems)
- **Real-Time Updates**: WebSocket server for live seat availability updates
- **Database Scaling**: Read replica support for horizontal read scaling
- Database transactions with row-level locking
- Redis caching for event data
- Rate limiting for high-traffic scenarios
- Virtual waiting room for ticket drops

### Installation

1. Clone the repository
2. Install dependencies:
```bash
go mod download
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your database and Redis credentials
```

4. Run database migrations:
```bash
go run cmd/migrate/main.go
```

5. Start the server:
```bash
go run cmd/server/main.go
```

## Concurrency Control

The system uses PostgreSQL's `SELECT FOR UPDATE` with `NOWAIT` to prevent double-booking. When a user reserves a seat, the row is locked until the transaction completes, ensuring atomicity.


**Asynchronous Booking Processing**: Uses Redis Streams as a message queue to handle traffic spikes. API returns HTTP 202 (Accepted) immediately while background workers process requests.

**Idempotency for Payments**: Middleware prevents duplicate charges when network requests fail. Uses idempotency keys stored in Redis with 24-hour TTL.

**Real-Time Seat Availability**: WebSocket server broadcasts seat status changes to all connected clients, eliminating the need for polling.

**Database Read Replicas**: Supports read/write separation with automatic failover. Read queries use replicas, writes use primary database.
