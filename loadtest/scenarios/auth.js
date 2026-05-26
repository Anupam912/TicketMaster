/**
 * Authentication load test.
 * Tests: /api/auth/register, /api/auth/login, /api/auth/me
 */
import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import { BASE_URL, THRESHOLDS, TEST_USER_PREFIX } from '../config.js';

export const options = {
    stages: [
        { duration: '20s', target: 20 },
        { duration: '1m', target: 20 },
        { duration: '20s', target: 0 },
    ],
    thresholds: THRESHOLDS,
};

const headers = { 'Content-Type': 'application/json' };

export default function () {
    const uniqueId = `${__VU}_${__ITER}_${Date.now()}`;
    const email = `${TEST_USER_PREFIX}${uniqueId}@test.com`;
    const password = 'TestPass123!';

    // Register new user
    const registerPayload = JSON.stringify({
        email: email,
        password: password,
        full_name: `Load Test User ${uniqueId}`,
    });

    const registerRes = http.post(
        `${BASE_URL}/api/auth/register`,
        registerPayload,
        { headers }
    );

    const registered = check(registerRes, {
        'register status 201': (r) => r.status === 201,
        'register returns token': (r) => r.json('token') !== undefined,
    });

    if (!registered) {
        console.log(`Register failed: ${registerRes.status} - ${registerRes.body}`);
        return;
    }

    sleep(0.5);

    // Login with new user
    const loginPayload = JSON.stringify({
        email: email,
        password: password,
    });

    const loginRes = http.post(
        `${BASE_URL}/api/auth/login`,
        loginPayload,
        { headers }
    );

    const loggedIn = check(loginRes, {
        'login status 200': (r) => r.status === 200,
        'login returns token': (r) => r.json('token') !== undefined,
    });

    if (!loggedIn) {
        console.log(`Login failed: ${loginRes.status} - ${loginRes.body}`);
        return;
    }

    const token = loginRes.json('token');

    sleep(0.5);

    // Get current user profile
    const meRes = http.get(`${BASE_URL}/api/auth/me`, {
        headers: {
            ...headers,
            'Authorization': `Bearer ${token}`,
        },
    });

    check(meRes, {
        'me status 200': (r) => r.status === 200,
        'me returns correct email': (r) => r.json('user.email') === email,
    });

    sleep(1);
}
