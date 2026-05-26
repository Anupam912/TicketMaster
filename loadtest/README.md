# Load Testing with k6

This directory contains load tests for the Event Ticketing System using [k6](https://k6.io/).

## Prerequisites

### Install k6

**macOS:**
```bash
brew install k6
```

**Docker (alternative):**
```bash
docker pull grafana/k6
```

### Ensure Test Data Exists

Before running booking tests, make sure you have:
1. An admin user created
2. At least one venue created
3. At least one event with seats

```bash
# Create venue (as admin)
curl -X POST http://localhost:8080/api/venues \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin_token>" \
  -d '{
    "name": "Load Test Arena",
    "address": "123 Test Street",
    "city": "Test City",
    "capacity": 1000
  }'

# Create event (as admin)
curl -X POST http://localhost:8080/api/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin_token>" \
  -d '{
    "name": "Load Test Concert",
    "description": "A concert for load testing",
    "venue_id": "<venue_id>",
    "event_date": "2026-12-31T20:00:00Z",
    "ticket_price": 50.00,
    "total_seats": 500
  }'
```

## Test Scenarios

### 1. Health & Read Endpoints
Basic load test for health checks and event listing.

```bash
k6 run scenarios/health.js
```

### 2. Authentication
Tests user registration, login, and profile access.

```bash
k6 run scenarios/auth.js
```

### 3. Booking (Flash Sale Simulation)
Critical test - simulates high-concurrency ticket booking.

```bash
k6 run scenarios/booking.js
```

### 4. Full User Journey
Complete user flow from registration to purchase.

```bash
k6 run scenarios/full-journey.js
```

## Running with Docker

```bash
# From the loadtest directory
docker run --rm -i --network=host \
  -v $(pwd):/scripts \
  grafana/k6 run /scripts/scenarios/health.js
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_URL` | `http://localhost:8080` | API base URL |

Example:
```bash
k6 run -e BASE_URL=http://your-server:8080 scenarios/health.js
```

### Adjusting Load

Modify the `stages` in each test file to adjust:
- `duration`: How long each stage lasts
- `target`: Number of virtual users

Example for heavier load:
```javascript
export const options = {
    stages: [
        { duration: '1m', target: 200 },   // Ramp to 200 users
        { duration: '5m', target: 200 },   // Sustained load
        { duration: '1m', target: 500 },   // Spike to 500 users
        { duration: '2m', target: 500 },   // Sustained spike
        { duration: '1m', target: 0 },     // Ramp down
    ],
};
```

## Understanding Results

### Key Metrics

| Metric | Description | Target |
|--------|-------------|--------|
| `http_req_duration` | Response time | p95 < 500ms |
| `http_req_failed` | Error rate | < 1% |
| `reservations_successful` | Successful bookings | > 0 |
| `reservations_conflict` | Seat conflicts (expected) | High in flash sale |

### Sample Output

```
========== BOOKING LOAD TEST SUMMARY ==========
Total Requests: 15234
Successful Reservations: 487
Conflicts (seat taken): 892
Successful Purchases: 485
Avg Response Time: 45.23ms
P95 Response Time: 156.78ms
P99 Response Time: 342.12ms
Error Rate: 0.12%
================================================
```

## Tips

1. **Start small**: Begin with low VU counts and gradually increase
2. **Monitor the app**: Watch Docker logs during tests: `docker logs -f ticketmaster-app-1`
3. **Check Redis**: High-concurrency tests exercise Redis rate limiting
4. **Database connections**: Watch for connection pool exhaustion
5. **Clean up**: Load tests create many users; truncate `users` table between runs if needed

## Cleanup Test Data

```sql
-- Connect to postgres
docker exec -it ticketmaster-postgres-1 psql -U postgres -d event_ticketing

-- Remove load test users
DELETE FROM bookings WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'loadtest_%');
DELETE FROM users WHERE email LIKE 'loadtest_%';
```
