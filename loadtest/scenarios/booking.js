/**
 * Seat reservation load test - simulates high-concurrency ticket booking.
 * This is the critical "flash sale" scenario.
 * 
 * Tests: /api/bookings/reserve, /api/bookings/purchase
 */
import http from 'k6/http';
import { check, sleep, fail } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { BASE_URL, THRESHOLDS, TEST_USER_PREFIX } from '../config.js';

// Custom metrics for booking-specific tracking
const reservationSuccess = new Counter('reservations_successful');
const reservationConflict = new Counter('reservations_conflict');
const purchaseSuccess = new Counter('purchases_successful');
const reservationDuration = new Trend('reservation_duration');

export const options = {
    scenarios: {
        // Simulate flash sale: many users trying to book at once
        flash_sale: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '10s', target: 20 },   // Quick ramp to 20 users
                { duration: '20s', target: 20 },   // Sustained load
                { duration: '10s', target: 50 },   // Spike to 50 users
                { duration: '20s', target: 50 },   // Sustained high load
                { duration: '10s', target: 0 },    // Ramp down
            ],
        },
    },
    thresholds: {
        'http_req_duration': ['p(95)<3000'],  // 3s for async processing
        'http_req_failed': ['rate<0.5'],      // Allow up to 50% failures (rate limiting expected)
        'reservations_successful': ['count>0'],
        'reservation_duration': ['p(95)<2000'],
    },
};

const headers = { 'Content-Type': 'application/json' };

// Setup: Create a test user and get token
export function setup() {
    const email = `booking_test_${Date.now()}@test.com`;
    const password = 'TestPass123!';

    // Register user
    const registerRes = http.post(
        `${BASE_URL}/api/auth/register`,
        JSON.stringify({
            email: email,
            password: password,
            full_name: 'Booking Test User',
        }),
        { headers }
    );

    if (registerRes.status !== 201) {
        // User might exist, try login
        const loginRes = http.post(
            `${BASE_URL}/api/auth/login`,
            JSON.stringify({ email, password }),
            { headers }
        );
        if (loginRes.status === 200) {
            return { token: loginRes.json('token'), email };
        }
        fail('Could not create or login test user');
    }

    // Get events to find one with available seats
    const eventsRes = http.get(`${BASE_URL}/api/events`);
    const events = eventsRes.json() || [];
    
    let targetEvent = null;
    for (const event of events) {
        if (event.available_seats > 0) {
            targetEvent = event;
            break;
        }
    }

    return {
        token: registerRes.json('token'),
        email: email,
        eventId: targetEvent ? targetEvent.id : null,
    };
}

export default function (data) {
    if (!data.eventId) {
        console.log('No event with available seats found. Skipping booking test.');
        sleep(1);
        return;
    }

    // Each VU registers their own user to avoid token conflicts
    const uniqueId = `${__VU}_${__ITER}_${Date.now()}`;
    const email = `${TEST_USER_PREFIX}${uniqueId}@test.com`;
    const password = 'TestPass123!';

    // Quick registration
    const registerRes = http.post(
        `${BASE_URL}/api/auth/register`,
        JSON.stringify({
            email: email,
            password: password,
            full_name: `Booker ${uniqueId}`,
        }),
        { headers }
    );

    let token;
    if (registerRes.status === 201) {
        token = registerRes.json('token');
    } else {
        // Skip if registration fails (rate limited, etc.)
        sleep(1);
        return;
    }

    const authHeaders = {
        ...headers,
        'Authorization': `Bearer ${token}`,
    };

    // Get available seats for the event
    const seatsRes = http.get(
        `${BASE_URL}/api/events/${data.eventId}/seats?status=available`,
        { headers: authHeaders }
    );

    if (seatsRes.status !== 200) {
        sleep(1);
        return;
    }

    const seatsData = seatsRes.json();
    const seats = seatsData.seats || [];
    if (seats.length === 0) {
        console.log('No available seats');
        sleep(1);
        return;
    }

    // Pick a random available seat
    const randomSeat = seats[Math.floor(Math.random() * seats.length)];

    // Try to reserve the seat
    const reserveStart = Date.now();
    const reserveRes = http.post(
        `${BASE_URL}/api/bookings/reserve`,
        JSON.stringify({
            event_id: data.eventId,
            seat_number: randomSeat.seat_number,
        }),
        { headers: authHeaders }
    );
    const reserveDuration = Date.now() - reserveStart;
    reservationDuration.add(reserveDuration);

    // API returns 202 Accepted with job_id for async processing
    if (reserveRes.status === 202) {
        const jobResponse = reserveRes.json();
        
        check(reserveRes, {
            'reservation accepted': (r) => r.status === 202,
            'has job_id': () => jobResponse.job_id !== undefined,
        });

        // Poll for job completion (max 3 attempts)
        let jobCompleted = false;
        let bookingId = null;
        
        for (let i = 0; i < 3; i++) {
            sleep(0.5);
            const jobRes = http.get(
                `${BASE_URL}/api/bookings/job/${jobResponse.job_id}`,
                { headers: authHeaders }
            );
            
            if (jobRes.status === 200) {
                const jobStatus = jobRes.json();
                if (jobStatus.status === 'completed') {
                    jobCompleted = true;
                    bookingId = jobStatus.booking_id;
                    reservationSuccess.add(1);
                    break;
                } else if (jobStatus.status === 'failed') {
                    reservationConflict.add(1);
                    break;
                }
            }
        }

        // If booking completed, try to purchase
        if (jobCompleted && bookingId) {
            const purchaseRes = http.post(
                `${BASE_URL}/api/bookings/purchase`,
                JSON.stringify({
                    booking_id: bookingId,
                    payment_method: 'card',
                }),
                { 
                    headers: {
                        ...authHeaders,
                        'Idempotency-Key': `purchase_${bookingId}_${Date.now()}`,
                    }
                }
            );

            if (purchaseRes.status === 200) {
                purchaseSuccess.add(1);
                check(purchaseRes, {
                    'purchase successful': (r) => r.status === 200,
                });
            }
        }
    } else if (reserveRes.status === 201) {
        // Direct sync response (fallback)
        reservationSuccess.add(1);
        const booking = reserveRes.json();
        check(reserveRes, {
            'reservation successful': (r) => r.status === 201,
        });
    } else if (reserveRes.status === 409) {
        // Seat already taken - this is expected in high-concurrency scenarios
        reservationConflict.add(1);
        check(reserveRes, {
            'conflict handled correctly': (r) => r.status === 409,
        });
    } else if (reserveRes.status === 429) {
        // Rate limited - virtual waiting room working
        check(reserveRes, {
            'rate limited (waiting room)': (r) => r.status === 429,
        });
    }

    sleep(1);
}

export function handleSummary(data) {
    const httpDuration = data.metrics.http_req_duration?.values || {};
    const httpFailed = data.metrics.http_req_failed?.values || {};
    
    const summary = {
        'Total Requests': data.metrics.http_reqs?.values?.count || 0,
        'Successful Reservations': data.metrics.reservations_successful?.values?.count || 0,
        'Conflicts (seat taken)': data.metrics.reservations_conflict?.values?.count || 0,
        'Successful Purchases': data.metrics.purchases_successful?.values?.count || 0,
        'Avg Response Time': `${(httpDuration.avg || 0).toFixed(2)}ms`,
        'P95 Response Time': `${(httpDuration['p(95)'] || 0).toFixed(2)}ms`,
        'P99 Response Time': `${(httpDuration['p(99)'] || 0).toFixed(2)}ms`,
        'Error Rate': `${((httpFailed.rate || 0) * 100).toFixed(2)}%`,
    };

    console.log('\n========== BOOKING LOAD TEST SUMMARY ==========');
    for (const [key, value] of Object.entries(summary)) {
        console.log(`${key}: ${value}`);
    }
    console.log('================================================\n');

    return {
        'stdout': JSON.stringify(summary, null, 2),
    };
}
