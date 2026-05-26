/**
 * Basic health check and read-only endpoint load test.
 * Tests: /health, /api/events
 */
import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, THRESHOLDS } from '../config.js';

export const options = {
    stages: [
        { duration: '30s', target: 50 },   // Ramp up to 50 users
        { duration: '1m', target: 50 },    // Stay at 50 users
        { duration: '30s', target: 100 },  // Ramp up to 100 users
        { duration: '1m', target: 100 },   // Stay at 100 users
        { duration: '30s', target: 0 },    // Ramp down
    ],
    thresholds: THRESHOLDS,
};

export default function () {
    // Health check
    const healthRes = http.get(`${BASE_URL}/health`);
    check(healthRes, {
        'health status 200': (r) => r.status === 200,
        'health response valid': (r) => r.json('status') === 'healthy',
    });

    // List events
    const eventsRes = http.get(`${BASE_URL}/api/events`);
    check(eventsRes, {
        'events status 200': (r) => r.status === 200,
        'events is array': (r) => Array.isArray(r.json()),
    });

    sleep(1);
}
