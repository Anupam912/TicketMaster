package models

import (
	"time"

	"github.com/google/uuid"
)

type BookingStatus string

const (
	BookingStatusReserved  BookingStatus = "reserved"
	BookingStatusPurchased BookingStatus = "purchased"
	BookingStatusCancelled BookingStatus = "cancelled"
	BookingStatusExpired   BookingStatus = "expired"
)

type Booking struct {
	ID          uuid.UUID     `json:"id" db:"id"`
	UserID      uuid.UUID     `json:"user_id" db:"user_id"`
	EventID     uuid.UUID     `json:"event_id" db:"event_id"`
	SeatID      uuid.UUID     `json:"seat_id" db:"seat_id"`
	Status      BookingStatus `json:"status" db:"status"`
	TotalAmount float64       `json:"total_amount" db:"total_amount"`
	ReservedAt  time.Time     `json:"reserved_at" db:"reserved_at"`
	PurchasedAt *time.Time    `json:"purchased_at,omitempty" db:"purchased_at"`
	ExpiresAt   time.Time     `json:"expires_at" db:"expires_at"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
}

type ReserveSeatRequest struct {
	EventID    uuid.UUID `json:"event_id" binding:"required"`
	SeatNumber string    `json:"seat_number" binding:"required"`
}

type BulkReserveRequest struct {
	EventID     uuid.UUID `json:"event_id" binding:"required"`
	SeatNumbers []string  `json:"seat_numbers" binding:"required,min=1,max=500"`
}

type PurchaseBookingRequest struct {
	BookingID      uuid.UUID `json:"booking_id" binding:"required"`
	IdempotencyKey string    `json:"idempotency_key"`
}

type BookingWithDetails struct {
	Booking
	EventTitle string    `json:"event_title"`
	EventDate  time.Time `json:"event_date"`
	SeatNumber string    `json:"seat_number"`
	VenueName  string    `json:"venue_name"`
}
