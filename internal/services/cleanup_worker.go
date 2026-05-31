package services

import (
	"context"
	"log"
	"time"
)

const (
	cleanupJobInitialInterval = time.Minute
	cleanupJobMaxBackoff      = 15 * time.Minute
)

// RunExpiredReservationCleanup periodically releases expired reservations from the database.
// On persistent failures the wait between attempts doubles up to cleanupJobMaxBackoff.
func RunExpiredReservationCleanup(ctx context.Context, bookingService *BookingService) {
	interval := cleanupJobInitialInterval

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := bookingService.CleanupExpiredReservations(ctx)
		if err != nil {
			log.Printf("Error cleaning up expired reservations: %v", err)
			interval = nextCleanupBackoff(interval)
			log.Printf("Cleanup job backing off for %s", interval)
		} else {
			interval = cleanupJobInitialInterval
		}

		if !waitWithContext(ctx, interval) {
			return
		}
	}
}

func nextCleanupBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next < cleanupJobInitialInterval {
		next = cleanupJobInitialInterval
	}
	if next > cleanupJobMaxBackoff {
		return cleanupJobMaxBackoff
	}
	return next
}
