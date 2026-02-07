package models

import (
	"time"

	"github.com/google/uuid"
)

type Venue struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	Name      string                 `json:"name" db:"name"`
	Address   string                 `json:"address" db:"address"`
	Capacity  int                    `json:"capacity" db:"capacity"`
	SeatLayout map[string]interface{} `json:"seat_layout" db:"seat_layout"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

type CreateVenueRequest struct {
	Name      string                 `json:"name" binding:"required"`
	Address   string                 `json:"address" binding:"required"`
	Capacity  int                    `json:"capacity" binding:"required,min=1"`
	SeatLayout map[string]interface{} `json:"seat_layout"`
}
