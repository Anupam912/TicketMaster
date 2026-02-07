package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"event-ticketing-system/internal/database"
	"event-ticketing-system/internal/models"

	"github.com/google/uuid"
)

var (
	ErrVenueNotFound = errors.New("venue not found")
)

type VenueRepository struct{}

func NewVenueRepository() *VenueRepository {
	return &VenueRepository{}
}

func (r *VenueRepository) Create(venue *models.Venue) error {
	query := `
		INSERT INTO venues (id, name, address, capacity, seat_layout, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	venue.ID = uuid.New()
	now := time.Now()
	venue.CreatedAt = now
	venue.UpdatedAt = now

	seatLayoutJSON, err := json.Marshal(venue.SeatLayout)
	if err != nil {
		return err
	}

	err = database.DB.QueryRow(
		query,
		venue.ID,
		venue.Name,
		venue.Address,
		venue.Capacity,
		seatLayoutJSON,
		venue.CreatedAt,
		venue.UpdatedAt,
	).Scan(&venue.ID, &venue.CreatedAt, &venue.UpdatedAt)

	return err
}

func (r *VenueRepository) FindByID(id uuid.UUID) (*models.Venue, error) {
	query := `
		SELECT id, name, address, capacity, seat_layout, created_at, updated_at
		FROM venues
		WHERE id = $1
	`

	venue := &models.Venue{}
	var seatLayoutJSON []byte

	err := database.GetReadDB().QueryRow(query, id).Scan(
		&venue.ID,
		&venue.Name,
		&venue.Address,
		&venue.Capacity,
		&seatLayoutJSON,
		&venue.CreatedAt,
		&venue.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrVenueNotFound
		}
		return nil, err
	}

	if len(seatLayoutJSON) > 0 {
		if err := json.Unmarshal(seatLayoutJSON, &venue.SeatLayout); err != nil {
			return nil, err
		}
	}

	return venue, nil
}

func (r *VenueRepository) ListAll() ([]*models.Venue, error) {
	query := `
		SELECT id, name, address, capacity, seat_layout, created_at, updated_at
		FROM venues
		ORDER BY created_at DESC
	`
	rows, err := database.GetReadDB().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var venues []*models.Venue
	for rows.Next() {
		venue := &models.Venue{}
		var seatLayoutJSON []byte

		err := rows.Scan(
			&venue.ID,
			&venue.Name,
			&venue.Address,
			&venue.Capacity,
			&seatLayoutJSON,
			&venue.CreatedAt,
			&venue.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if len(seatLayoutJSON) > 0 {
			if err := json.Unmarshal(seatLayoutJSON, &venue.SeatLayout); err != nil {
				return nil, err
			}
		}

		venues = append(venues, venue)
	}

	return venues, rows.Err()
}
