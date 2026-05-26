/**
 * Shared configuration for load tests.
 */
export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const ADMIN_CREDS = {
    email: 'admin@ticketmaster.com',
    password: 'Admin@123'
};

export const TEST_USER_PREFIX = 'loadtest_user_';

export const THRESHOLDS = {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
};
