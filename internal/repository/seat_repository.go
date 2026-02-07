package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"event-ticketing-system/internal/database"
	"event-ticketing-system/internal/models"

	"github.com/google/uuid"
)

var (
	ErrSeatNotFound      = errors.New("seat not found")
	ErrSeatNotAvailable  = errors.New("seat is not available")
	ErrSeatAlreadyBooked = errors.New("seat is already booked")
)

type SeatRepository struct{}

func NewSeatRepository() *SeatRepository {
	return &SeatRepository{}
}

func (r *SeatRepository) Create(seat *models.Seat) error {
	query := `
		INSERT INTO 
		seats (id, event_id, seat_number, row_number, section, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	seat.ID = uuid.New()
	seat.CreatedAt = time.Now()
	if seat.Status == "" {
		seat.Status = models.SeatStatusAvailable
	}

	err := database.DB.QueryRow(
		query,
		seat.ID,
		seat.EventID,
		seat.SeatNumber,
		seat.RowNumber,
		seat.Section,
		seat.Status,
		seat.CreatedAt,
	).Scan(&seat.ID, &seat.CreatedAt)

	return err
}

func (r *SeatRepository) FindByEventAndSeatNumber(eventID uuid.UUID, seatNumber string) (*models.Seat, error) {
	query := `
		SELECT 
		id, event_id, seat_number, row_number, section, status, reserved_at, reserved_until, created_at
		FROM seats
		WHERE event_id = $1 AND seat_number = $2
	`

	seat := &models.Seat{}
	var reservedAt, reservedUntil sql.NullTime

	err := database.DB.QueryRow(query, eventID, seatNumber).Scan(
		&seat.ID,
		&seat.EventID,
		&seat.SeatNumber,
		&seat.RowNumber,
		&seat.Section,
		&seat.Status,
		&reservedAt,
		&reservedUntil,
		&seat.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSeatNotFound
		}
		return nil, err
	}

	if reservedAt.Valid {
		seat.ReservedAt = &reservedAt.Time
	}
	if reservedUntil.Valid {
		seat.ReservedUntil = &reservedUntil.Time
	}

	return seat, nil
}

func (r *SeatRepository) ReserveSeatWithLock(eventID uuid.UUID, seatNumber string, reservedUntil time.Time) (*models.Seat, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `
		SELECT 
		id, event_id, seat_number, row_number, section, status, reserved_at, reserved_until, created_at
		FROM seats
		WHERE event_id = $1 AND seat_number = $2
		FOR UPDATE NOWAIT
	`

	seat := &models.Seat{}
	var reservedAt, reservedUntilDB sql.NullTime

	err = tx.QueryRow(query, eventID, seatNumber).Scan(
		&seat.ID,
		&seat.EventID,
		&seat.SeatNumber,
		&seat.RowNumber,
		&seat.Section,
		&seat.Status,
		&reservedAt,
		&reservedUntilDB,
		&seat.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSeatNotFound
		}

		if err.Error() == "pq: could not obtain lock on row in relation \"seats\"" {
			return nil, ErrSeatAlreadyBooked
		}
		return nil, err
	}

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

	updateQuery := `
		UPDATE seats
		SET status = $1, reserved_at = CURRENT_TIMESTAMP, reserved_until = $2
		WHERE id = $3
	`

	_, err = tx.Exec(updateQuery, models.SeatStatusReserved, reservedUntil, seat.ID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	now := time.Now()
	seat.Status = models.SeatStatusReserved
	seat.ReservedAt = &now
	seat.ReservedUntil = &reservedUntil

	return seat, nil
}

func (r *SeatRepository) MarkAsSold(seatID uuid.UUID) error {
	query := `
		UPDATE seats
		SET status = $1, reserved_at = NULL, reserved_until = NULL
		WHERE id = $2
	`

	_, err := database.DB.Exec(query, models.SeatStatusSold, seatID)
	return err
}

func (r *SeatRepository) ReleaseSeat(seatID uuid.UUID) error {
	query := `
		UPDATE seats
		SET status = $1, reserved_at = NULL, reserved_until = NULL
		WHERE id = $2
	`

	_, err := database.DB.Exec(query, models.SeatStatusAvailable, seatID)
	return err
}

func (r *SeatRepository) CreateBulkSeats(eventID uuid.UUID, totalSeats int) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO seats (id, event_id, seat_number, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := 1; i <= totalSeats; i++ {
		_, err = stmt.Exec(
			uuid.New(),
			eventID,
			seatNumber(i),
			models.SeatStatusAvailable,
			time.Now(),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func seatNumber(num int) string {
	row := (num-1)/10 + 1
	seat := (num-1)%10 + 1
	return fmt.Sprintf("R%d-S%d", row, seat)
}

func formatSeatNumber(num int) string {
	row := (num-1)/10 + 1
	seat := (num-1)%10 + 1
	return fmt.Sprintf("R%d-S%d", row, seat)
}
