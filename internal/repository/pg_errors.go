package repository

import (
	"errors"

	"github.com/lib/pq"
)

// PostgreSQL SQLSTATE for lock_not_available (e.g. SELECT ... FOR UPDATE NOWAIT).
const pgSQLStateLockNotAvailable = "55P03"

// isLockNotAvailable reports whether err is a PostgreSQL lock_not_available error.
func isLockNotAvailable(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == pgSQLStateLockNotAvailable
	}
	return false
}
