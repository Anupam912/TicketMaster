package models

import (
	"time"

	"github.com/google/uuid"
)

type SeatStatus string

const (
	SeatStatusAvailable SeatStatus = "available"
	SeatStatusReserved  SeatStatus = "reserved"
	SeatStatusSold      SeatStatus = "sold"
)

type Seat struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	EventID      uuid.UUID  `json:"event_id" db:"event_id"`
	SeatNumber   string     `json:"seat_number" db:"seat_number"`
	RowNumber    string     `json:"row_number" db:"row_number"`
	Section      string     `json:"section" db:"section"`
	Status       SeatStatus `json:"status" db:"status"`
	ReservedAt   *time.Time `json:"reserved_at,omitempty" db:"reserved_at"`
	ReservedUntil *time.Time `json:"reserved_until,omitempty" db:"reserved_until"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}
