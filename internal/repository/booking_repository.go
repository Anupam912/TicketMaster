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
	ErrBookingNotFound    = errors.New("booking not found")
	ErrNoAvailableSeats   = errors.New("no available seats")
	ErrBookingNotReserved = errors.New("booking is not reserved")
	ErrBookingExpired     = errors.New("booking has expired")
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

func (r *BookingRepository) CreateReservation(userID, eventID uuid.UUID, seatNumber string, expiresAt time.Time) (*models.Booking, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var ticketPrice float64
	err = tx.QueryRow(`
		UPDATE events
		SET available_seats = available_seats - 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND available_seats > 0
		RETURNING ticket_price
	`, eventID).Scan(&ticketPrice)
	if err != nil {
		if err == sql.ErrNoRows {
			exists, existsErr := r.eventExistsTx(tx, eventID)
			if existsErr != nil {
				return nil, existsErr
			}
			if !exists {
				return nil, ErrEventNotFound
			}
			return nil, ErrNoAvailableSeats
		}
		return nil, err
	}

	seat, err := reserveSeatTx(tx, eventID, seatNumber, expiresAt)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	booking := &models.Booking{
		ID:          uuid.New(),
		UserID:      userID,
		EventID:     eventID,
		SeatID:      seat.ID,
		Status:      models.BookingStatusReserved,
		TotalAmount: ticketPrice,
		ReservedAt:  now,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = tx.QueryRow(`
		INSERT INTO bookings (id, user_id, event_id, seat_id, status, total_amount, reserved_at, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`,
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
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return booking, nil
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

func (r *BookingRepository) CompletePurchase(id uuid.UUID) (*models.Booking, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	booking, err := findBookingForUpdateTx(tx, id)
	if err != nil {
		return nil, err
	}

	if booking.Status == models.BookingStatusPurchased {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return booking, nil
	}

	if booking.Status != models.BookingStatusReserved {
		return nil, ErrBookingNotReserved
	}
	if time.Now().After(booking.ExpiresAt) {
		return nil, ErrBookingExpired
	}

	var purchasedAt time.Time
	err = tx.QueryRow(`
		UPDATE bookings
		SET status = $1, purchased_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND status = $3 AND expires_at > CURRENT_TIMESTAMP
		RETURNING purchased_at, updated_at
	`, models.BookingStatusPurchased, id, models.BookingStatusReserved).Scan(&purchasedAt, &booking.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrBookingNotReserved
		}
		return nil, err
	}

	if _, err := tx.Exec(`
		UPDATE seats
		SET status = $1, reserved_at = NULL, reserved_until = NULL
		WHERE id = $2
	`, models.SeatStatusSold, booking.SeatID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	booking.Status = models.BookingStatusPurchased
	booking.PurchasedAt = &purchasedAt
	return booking, nil
}

func (r *BookingRepository) ExtendReservation(id uuid.UUID, extendUntil time.Time) (*models.Booking, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	booking, err := findBookingForUpdateTx(tx, id)
	if err != nil {
		return nil, err
	}
	if booking.Status != models.BookingStatusReserved {
		return nil, ErrBookingNotReserved
	}
	if time.Now().After(booking.ExpiresAt) {
		return nil, ErrBookingExpired
	}
	if booking.ExpiresAt.Before(extendUntil) {
		booking.ExpiresAt = extendUntil
	}

	result, err := tx.Exec(`
		UPDATE bookings
		SET expires_at = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND status = $3 AND expires_at > CURRENT_TIMESTAMP
	`, booking.ExpiresAt, id, models.BookingStatusReserved)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, ErrBookingNotReserved
	}

	if _, err := tx.Exec(`
		UPDATE seats
		SET reserved_until = $1
		WHERE id = $2 AND status = $3
	`, booking.ExpiresAt, booking.SeatID, models.SeatStatusReserved); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return booking, nil
}

func (r *BookingRepository) ReleaseExpiredReservation(id uuid.UUID) (*models.Booking, error) {
	return r.releaseReservation(id, models.BookingStatusExpired, true)
}

func (r *BookingRepository) CancelReservation(id uuid.UUID) (*models.Booking, error) {
	return r.releaseReservation(id, models.BookingStatusCancelled, false)
}

func (r *BookingRepository) releaseReservation(id uuid.UUID, status models.BookingStatus, requireExpired bool) (*models.Booking, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	booking, err := findBookingForUpdateTx(tx, id)
	if err != nil {
		return nil, err
	}
	if booking.Status != models.BookingStatusReserved {
		return nil, ErrBookingNotReserved
	}
	if requireExpired && time.Now().Before(booking.ExpiresAt) {
		return nil, ErrBookingNotReserved
	}

	query := `
		UPDATE bookings
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND status = $3
	`
	if requireExpired {
		query += ` AND expires_at <= CURRENT_TIMESTAMP`
	}

	result, err := tx.Exec(query, status, id, models.BookingStatusReserved)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, ErrBookingNotReserved
	}

	if _, err := tx.Exec(`
		UPDATE seats
		SET status = $1, reserved_at = NULL, reserved_until = NULL
		WHERE id = $2
	`, models.SeatStatusAvailable, booking.SeatID); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`
		UPDATE events
		SET available_seats = available_seats + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, booking.EventID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	booking.Status = status
	return booking, nil
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

func (r *BookingRepository) eventExistsTx(tx *sql.Tx, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM events WHERE id = $1)`, eventID).Scan(&exists)
	return exists, err
}

func reserveSeatTx(tx *sql.Tx, eventID uuid.UUID, seatNumber string, reservedUntil time.Time) (*models.Seat, error) {
	seat := &models.Seat{}
	var reservedAt, reservedUntilDB sql.NullTime
	var rowNumber, section sql.NullString

	err := tx.QueryRow(`
		SELECT id, event_id, seat_number, row_number, section, status, reserved_at, reserved_until, created_at
		FROM seats
		WHERE event_id = $1 AND seat_number = $2
		FOR UPDATE NOWAIT
	`, eventID, seatNumber).Scan(
		&seat.ID,
		&seat.EventID,
		&seat.SeatNumber,
		&rowNumber,
		&section,
		&seat.Status,
		&reservedAt,
		&reservedUntilDB,
		&seat.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSeatNotFound
		}
		if err.Error() == `pq: could not obtain lock on row in relation "seats"` {
			return nil, ErrSeatAlreadyBooked
		}
		return nil, err
	}

	seat.RowNumber = rowNumber.String
	seat.Section = section.String
	if reservedAt.Valid {
		seat.ReservedAt = &reservedAt.Time
	}
	if reservedUntilDB.Valid {
		seat.ReservedUntil = &reservedUntilDB.Time
	}
	if seat.Status != models.SeatStatusAvailable {
		return nil, ErrSeatNotAvailable
	}
	if seat.ReservedUntil != nil && time.Now().Before(*seat.ReservedUntil) {
		return nil, ErrSeatNotAvailable
	}

	if _, err := tx.Exec(`
		UPDATE seats
		SET status = $1, reserved_at = CURRENT_TIMESTAMP, reserved_until = $2
		WHERE id = $3
	`, models.SeatStatusReserved, reservedUntil, seat.ID); err != nil {
		return nil, err
	}

	now := time.Now()
	seat.Status = models.SeatStatusReserved
	seat.ReservedAt = &now
	seat.ReservedUntil = &reservedUntil

	return seat, nil
}

func findBookingForUpdateTx(tx *sql.Tx, id uuid.UUID) (*models.Booking, error) {
	booking := &models.Booking{}
	var purchasedAt sql.NullTime

	err := tx.QueryRow(`
		SELECT id, user_id, event_id, seat_id, status, total_amount, reserved_at, purchased_at, expires_at, created_at, updated_at
		FROM bookings
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
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
