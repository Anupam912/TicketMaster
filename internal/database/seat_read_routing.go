package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const seatReadPrimaryTTL = 30 * time.Second

type seatReadPrimaryContextKey struct{}

var seatReadRedis *redis.Client

// InitSeatReadRouting configures Redis-backed read-your-writes routing for seat queries.
func InitSeatReadRouting(client *redis.Client) {
	seatReadRedis = client
}

// WithReadPrimarySeats marks the context so seat reads use the primary database.
func WithReadPrimarySeats(ctx context.Context) context.Context {
	return context.WithValue(ctx, seatReadPrimaryContextKey{}, true)
}

// UsePrimaryForSeatReads reports whether seat reads should use the primary database.
func UsePrimaryForSeatReads(ctx context.Context) bool {
	flag, ok := ctx.Value(seatReadPrimaryContextKey{}).(bool)
	return ok && flag
}

// MarkSeatReadPrimary records a recent seat mutation for read-your-writes consistency.
func MarkSeatReadPrimary(ctx context.Context, userID, eventID uuid.UUID) {
	if userID == uuid.Nil || eventID == uuid.Nil {
		return
	}

	if seatReadRedis != nil {
		key := seatReadPrimaryRedisKey(userID, eventID)
		if err := seatReadRedis.Set(ctx, key, "1", seatReadPrimaryTTL).Err(); err != nil {
			log.Printf("Warning: failed to mark seat read primary user_id=%s event_id=%s: %v", userID, eventID, err)
		}
	}
}

// ContextForSeatReads returns a context that routes seat reads to primary when appropriate.
func ContextForSeatReads(ctx context.Context, userID, eventID uuid.UUID) context.Context {
	if UsePrimaryForSeatReads(ctx) {
		return ctx
	}
	if userID == uuid.Nil || eventID == uuid.Nil {
		return ctx
	}
	if seatReadRedis == nil {
		return ctx
	}

	key := seatReadPrimaryRedisKey(userID, eventID)
	exists, err := seatReadRedis.Exists(ctx, key).Result()
	if err != nil {
		log.Printf("Warning: failed to check seat read primary user_id=%s event_id=%s: %v", userID, eventID, err)
		return ctx
	}
	if exists == 0 {
		return ctx
	}

	return WithReadPrimarySeats(ctx)
}

// DBForSeatRead selects primary or replica for seat read queries.
func DBForSeatRead(ctx context.Context) *sql.DB {
	if UsePrimaryForSeatReads(ctx) {
		return DB
	}
	return GetReadDB()
}

func seatReadPrimaryRedisKey(userID, eventID uuid.UUID) string {
	return fmt.Sprintf("seat_read_primary:%s:%s", userID, eventID)
}
