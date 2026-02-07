package repository

import (
	"database/sql"
	"errors"
	"time"

	"event-ticketing-system/internal/database"
	"event-ticketing-system/internal/models"

	"github.com/google/uuid"
)

var (
	ErrEventNotFound = errors.New("event not found")
)

type EventRepository struct{}

func NewEventRepository() *EventRepository {
	return &EventRepository{}
}

func (r *EventRepository) Create(event *models.Event) error {
	query := `
		INSERT INTO
		events (id, venue_id, title, description, event_date, ticket_price, total_seats, available_seats, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	event.ID = uuid.New()
	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now
	event.AvailableSeats = event.TotalSeats

	err := database.DB.QueryRow(
		query,
		event.ID,
		event.VenueID,
		event.Title,
		event.Description,
		event.EventDate,
		event.TicketPrice,
		event.TotalSeats,
		event.AvailableSeats,
		event.CreatedBy,
		event.CreatedAt,
		event.UpdatedAt,
	).Scan(&event.ID, &event.CreatedAt, &event.UpdatedAt)

	return err
}

func (r *EventRepository) FindByID(id uuid.UUID) (*models.Event, error) {
	query := `
		SELECT 
		id, venue_id, title, description, event_date, ticket_price, total_seats, available_seats, created_by, created_at, updated_at
		FROM events
		WHERE id = $1
	`

	event := &models.Event{}
	// Use read replica for read operations
	err := database.GetReadDB().QueryRow(query, id).Scan(
		&event.ID,
		&event.VenueID,
		&event.Title,
		&event.Description,
		&event.EventDate,
		&event.TicketPrice,
		&event.TotalSeats,
		&event.AvailableSeats,
		&event.CreatedBy,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrEventNotFound
		}
		return nil, err
	}

	return event, nil
}

func (r *EventRepository) ListAll() ([]*models.Event, error) {
	query := `
		SELECT id, venue_id, title, description, event_date, ticket_price, total_seats, available_seats, created_by, created_at, updated_at
		FROM events
		ORDER BY event_date ASC
	`
	rows, err := database.GetReadDB().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event := &models.Event{}
		err := rows.Scan(
			&event.ID,
			&event.VenueID,
			&event.Title,
			&event.Description,
			&event.EventDate,
			&event.TicketPrice,
			&event.TotalSeats,
			&event.AvailableSeats,
			&event.CreatedBy,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

func (r *EventRepository) DecrementAvailableSeats(eventID uuid.UUID) error {
	query := `
		UPDATE events
		SET available_seats = available_seats - 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND available_seats > 0
		RETURNING available_seats
	`

	var availableSeats int
	err := database.DB.QueryRow(query, eventID).Scan(&availableSeats)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("no available seats")
		}
		return err
	}

	return nil
}

func (r *EventRepository) IncrementAvailableSeats(eventID uuid.UUID) error {
	query := `
		UPDATE events
		SET available_seats = available_seats + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := database.DB.Exec(query, eventID)
	return err
}
