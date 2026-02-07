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
	ErrBookingNotFound = errors.New("booking not found")
)

type BookingRepository struct{}

func NewBookingRepository() *BookingRepository {
	return &BookingRepository{}
}

func (r *BookingRepository) Create(booking *models.Booking) error {
	query := `
		INSERT INTO 
		bookings (id, user_id, event_id, seat_id, status, total_amount, reserved_at, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	booking.ID = uuid.New()
	now := time.Now()
	booking.CreatedAt = now
	booking.UpdatedAt = now

	err := database.DB.QueryRow(
		query,
		booking.ID,
		booking.UserID,
		booking.EventID,
		booking.SeatID,
		booking.Status,
		booking.TotalAmount,
		booking.ReservedAt,
		booking.ExpiresAt,
		booking.CreatedAt,
		booking.UpdatedAt,
	).Scan(&booking.ID, &booking.CreatedAt, &booking.UpdatedAt)

	return err
}

func (r *BookingRepository) FindByID(id uuid.UUID) (*models.Booking, error) {
	query := `
		SELECT 
		id, user_id, event_id, seat_id, status, total_amount, reserved_at, purchased_at, expires_at, created_at, updated_at
		FROM bookings
		WHERE id = $1
	`

	booking := &models.Booking{}
	var purchasedAt sql.NullTime

	err := database.DB.QueryRow(query, id).Scan(
		&booking.ID,
		&booking.UserID,
		&booking.EventID,
		&booking.SeatID,
		&booking.Status,
		&booking.TotalAmount,
		&booking.ReservedAt,
		&purchasedAt,
		&booking.ExpiresAt,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}

	if purchasedAt.Valid {
		booking.PurchasedAt = &purchasedAt.Time
	}

	return booking, nil
}

func (r *BookingRepository) FindByUserID(userID uuid.UUID) ([]*models.BookingWithDetails, error) {
	query := `
		SELECT 
			b.id, b.user_id, b.event_id, b.seat_id, b.status, b.total_amount, 
			b.reserved_at, b.purchased_at, b.expires_at, b.created_at, b.updated_at,
			e.title as event_title, e.event_date,
			s.seat_number, v.name as venue_name
		FROM bookings b
		JOIN events e ON b.event_id = e.id
		JOIN seats s ON b.seat_id = s.id
		JOIN venues v ON e.venue_id = v.id
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC
	`
	rows, err := database.GetReadDB().Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*models.BookingWithDetails
	for rows.Next() {
		booking := &models.BookingWithDetails{}
		var purchasedAt sql.NullTime

		err := rows.Scan(
			&booking.ID,
			&booking.UserID,
			&booking.EventID,
			&booking.SeatID,
			&booking.Status,
			&booking.TotalAmount,
			&booking.ReservedAt,
			&purchasedAt,
			&booking.ExpiresAt,
			&booking.CreatedAt,
			&booking.UpdatedAt,
			&booking.EventTitle,
			&booking.EventDate,
			&booking.SeatNumber,
			&booking.VenueName,
		)
		if err != nil {
			return nil, err
		}

		if purchasedAt.Valid {
			booking.PurchasedAt = &purchasedAt.Time
		}

		bookings = append(bookings, booking)
	}

	return bookings, rows.Err()
}

func (r *BookingRepository) UpdateStatus(id uuid.UUID, status models.BookingStatus) error {
	query := `
		UPDATE bookings
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	if status == models.BookingStatusPurchased {
		query = `
			UPDATE bookings
			SET status = $1, purchased_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			WHERE id = $2
		`
	}

	_, err := database.DB.Exec(query, status, id)
	return err
}

func (r *BookingRepository) FindExpiredReservations() ([]*models.Booking, error) {
	query := `
		SELECT 
		id, user_id, event_id, seat_id, status, total_amount, reserved_at, purchased_at, expires_at, created_at, updated_at
		FROM bookings
		WHERE status = $1 AND expires_at < CURRENT_TIMESTAMP
	`

	rows, err := database.DB.Query(query, models.BookingStatusReserved)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*models.Booking
	for rows.Next() {
		booking := &models.Booking{}
		var purchasedAt sql.NullTime

		err := rows.Scan(
			&booking.ID,
			&booking.UserID,
			&booking.EventID,
			&booking.SeatID,
			&booking.Status,
			&booking.TotalAmount,
			&booking.ReservedAt,
			&purchasedAt,
			&booking.ExpiresAt,
			&booking.CreatedAt,
			&booking.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if purchasedAt.Valid {
			booking.PurchasedAt = &purchasedAt.Time
		}

		bookings = append(bookings, booking)
	}

	return bookings, rows.Err()
}
