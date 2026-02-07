package models

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID            uuid.UUID `json:"id" db:"id"`
	VenueID       uuid.UUID `json:"venue_id" db:"venue_id"`
	Title         string    `json:"title" db:"title"`
	Description   string    `json:"description" db:"description"`
	EventDate     time.Time `json:"event_date" db:"event_date"`
	TicketPrice   float64   `json:"ticket_price" db:"ticket_price"`
	TotalSeats    int       `json:"total_seats" db:"total_seats"`
	AvailableSeats int      `json:"available_seats" db:"available_seats"`
	CreatedBy     uuid.UUID `json:"created_by" db:"created_by"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type CreateEventRequest struct {
	VenueID     uuid.UUID `json:"venue_id" binding:"required"`
	Title       string    `json:"title" binding:"required"`
	Description string    `json:"description"`
	EventDate   time.Time `json:"event_date" binding:"required"`
	TicketPrice float64   `json:"ticket_price" binding:"required,min=0"`
	TotalSeats  int       `json:"total_seats" binding:"required,min=1"`
}
