/**
 * Full user journey load test.
 * Simulates complete user flow: register -> browse events -> reserve -> purchase
 */
import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { BASE_URL, THRESHOLDS, TEST_USER_PREFIX } from '../config.js';

export const options = {
    scenarios: {
        user_journey: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '30s', target: 20 },
                { duration: '2m', target: 20 },
                { duration: '30s', target: 0 },
            ],
        },
    },
    thresholds: THRESHOLDS,
};

const headers = { 'Content-Type': 'application/json' };

export default function () {
    const uniqueId = `${__VU}_${__ITER}_${Date.now()}`;
    const email = `${TEST_USER_PREFIX}journey_${uniqueId}@test.com`;
    const password = 'TestPass123!';
    let token = null;

    // Step 1: Registration
    group('1. Registration', function () {
        const res = http.post(
            `${BASE_URL}/api/auth/register`,
            JSON.stringify({
                email: email,
                password: password,
                full_name: `Journey User ${uniqueId}`,
            }),
            { headers }
        );

        const success = check(res, {
            'registration successful': (r) => r.status === 201,
        });

        if (success) {
            token = res.json('token');
        }
    });

    if (!token) {
        return;
    }

    const authHeaders = {
        ...headers,
        'Authorization': `Bearer ${token}`,
    };

    sleep(1);

    // Step 2: Browse events
    let events = [];
    group('2. Browse Events', function () {
        const res = http.get(`${BASE_URL}/api/events`);
        
        check(res, {
            'events loaded': (r) => r.status === 200,
        });

        events = res.json() || [];
    });

    if (events.length === 0) {
        return;
    }

    sleep(1);

    // Step 3: View event details
    const selectedEvent = events[Math.floor(Math.random() * events.length)];
    let seats = [];

    group('3. View Event Details', function () {
        const eventRes = http.get(`${BASE_URL}/api/events/${selectedEvent.id}`);
        check(eventRes, {
            'event details loaded': (r) => r.status === 200,
        });

        const seatsRes = http.get(
            `${BASE_URL}/api/events/${selectedEvent.id}/seats?status=available`,
            { headers: authHeaders }
        );
        check(seatsRes, {
            'seats loaded': (r) => r.status === 200,
        });

        const seatsData = seatsRes.json() || {};
        seats = seatsData.seats || [];
    });

    if (seats.length === 0) {
        return;
    }

    sleep(2); // User thinks about which seat

    // Step 4: Reserve a seat
    const selectedSeat = seats[Math.floor(Math.random() * seats.length)];
    let booking = null;

    group('4. Reserve Seat', function () {
        const res = http.post(
            `${BASE_URL}/api/bookings/reserve`,
            JSON.stringify({
                event_id: selectedEvent.id,
                seat_number: selectedSeat.seat_number,
            }),
            { headers: authHeaders }
        );

        const success = check(res, {
            'reservation successful or seat taken': (r) => r.status === 201 || r.status === 409,
        });

        if (res.status === 201) {
            booking = res.json();
        }
    });

    if (!booking) {
        return;
    }

    sleep(3); // User enters payment details

    // Step 5: Complete purchase
    group('5. Complete Purchase', function () {
        const res = http.post(
            `${BASE_URL}/api/bookings/purchase`,
            JSON.stringify({
                booking_id: booking.id,
                payment_method: 'card',
            }),
            {
                headers: {
                    ...authHeaders,
                    'Idempotency-Key': `journey_${booking.id}_${Date.now()}`,
                }
            }
        );

        check(res, {
            'purchase successful': (r) => r.status === 200,
        });
    });

    sleep(1);

    // Step 6: View my bookings
    group('6. View My Bookings', function () {
        const res = http.get(
            `${BASE_URL}/api/bookings/my-bookings`,
            { headers: authHeaders }
        );

        check(res, {
            'my bookings loaded': (r) => r.status === 200,
            'has at least one booking': (r) => (r.json() || []).length > 0,
        });
    });

    sleep(1);
}
