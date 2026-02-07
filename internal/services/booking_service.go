package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/repository"

	"github.com/google/uuid"
)

type BookingService struct {
	bookingRepo  *repository.BookingRepository
	eventRepo    *repository.EventRepository
	seatRepo     *repository.SeatRepository
	config       *config.Config
	expiryQueue  *queue.ExpiryQueue
}

func NewBookingService(
	bookingRepo *repository.BookingRepository,
	eventRepo *repository.EventRepository,
	seatRepo *repository.SeatRepository,
	cfg *config.Config,
	expiryQueue *queue.ExpiryQueue,
) *BookingService {
	return &BookingService{
		bookingRepo:  bookingRepo,
		eventRepo:    eventRepo,
		seatRepo:     seatRepo,
		config:       cfg,
		expiryQueue:  expiryQueue,
	}
}

func (s *BookingService) ReserveSeat(userID uuid.UUID, req *models.ReserveSeatRequest) (*models.Booking, error) {
	event, err := s.eventRepo.FindByID(req.EventID)
	if err != nil {
		if err == repository.ErrEventNotFound {
			return nil, errors.New("event not found")
		}
		return nil, err
	}

	if event.AvailableSeats <= 0 {
		return nil, errors.New("no available seats")
	}

	expiresAt := time.Now().Add(s.config.Booking.ReservationTimeout())

	seat, err := s.seatRepo.ReserveSeatWithLock(req.EventID, req.SeatNumber, expiresAt)
	if err != nil {
		if err == repository.ErrSeatAlreadyBooked {
			return nil, errors.New("seat is currently being booked by another user")
		}
		if err == repository.ErrSeatNotAvailable {
			return nil, errors.New("seat is not available")
		}
		return nil, err
	}

	if err := s.eventRepo.DecrementAvailableSeats(req.EventID); err != nil {
		s.seatRepo.ReleaseSeat(seat.ID)
		return nil, errors.New("failed to update available seats")
	}

	booking := &models.Booking{
		UserID:      userID,
		EventID:     req.EventID,
		SeatID:      seat.ID,
		Status:      models.BookingStatusReserved,
		TotalAmount: event.TicketPrice,
		ReservedAt:  time.Now(),
		ExpiresAt:   expiresAt,
	}

	if err := s.bookingRepo.Create(booking); err != nil {
		s.seatRepo.ReleaseSeat(seat.ID)
		s.eventRepo.IncrementAvailableSeats(req.EventID)
		return nil, err
	}

	if s.expiryQueue != nil {
		_ = s.expiryQueue.Add(context.Background(), booking.ID.String(), booking.ExpiresAt)
	}

	return booking, nil
}

func (s *BookingService) PurchaseBooking(userID uuid.UUID, bookingID uuid.UUID) (*models.Booking, error) {
	booking, err := s.bookingRepo.FindByID(bookingID)
	if err != nil {
		if err == repository.ErrBookingNotFound {
			return nil, errors.New("booking not found")
		}
		return nil, err
	}

	if booking.UserID != userID {
		return nil, errors.New("unauthorized")
	}

	if booking.Status != models.BookingStatusReserved {
		return nil, errors.New("booking is not in reserved status")
	}

	if time.Now().After(booking.ExpiresAt) {
		s.ReleaseExpiredBooking(booking)
		return nil, errors.New("booking has expired")
	}

	if err := s.processPayment(booking); err != nil {
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	if err := s.bookingRepo.UpdateStatus(bookingID, models.BookingStatusPurchased); err != nil {
		return nil, err
	}

	if s.expiryQueue != nil {
		_ = s.expiryQueue.Remove(context.Background(), bookingID.String())
	}

	if err := s.seatRepo.MarkAsSold(booking.SeatID); err != nil {
		return nil, err
	}

	booking, err = s.bookingRepo.FindByID(bookingID)
	if err != nil {
		return nil, err
	}

	return booking, nil
}

func (s *BookingService) ReleaseExpiredBookingByID(ctx context.Context, bookingID uuid.UUID) error {
	booking, err := s.bookingRepo.FindByID(bookingID)
	if err != nil {
		if err == repository.ErrBookingNotFound {
			return nil
		}
		return err
	}
	if booking.Status != models.BookingStatusReserved {
		return nil
	}
	return s.ReleaseExpiredBooking(booking)
}

func (s *BookingService) ReleaseExpiredBooking(booking *models.Booking) error {
	if err := s.bookingRepo.UpdateStatus(booking.ID, models.BookingStatusExpired); err != nil {
		return err
	}

	if err := s.seatRepo.ReleaseSeat(booking.SeatID); err != nil {
		return err
	}

	if err := s.eventRepo.IncrementAvailableSeats(booking.EventID); err != nil {
		return err
	}

	return nil
}

func (s *BookingService) BulkReserve(userID uuid.UUID, req *models.BulkReserveRequest) ([]*models.Booking, error) {
	const chunkSize = 50
	seats := req.SeatNumbers
	if len(seats) == 0 {
		return nil, errors.New("no seats requested")
	}
	if len(seats) > 500 {
		return nil, errors.New("maximum 500 seats per bulk request")
	}

	var chunks [][]string
	for i := 0; i < len(seats); i += chunkSize {
		end := i + chunkSize
		if end > len(seats) {
			end = len(seats)
		}
		chunks = append(chunks, seats[i:end])
	}

	var mu sync.Mutex
	var successful []*models.Booking
	var firstErr error
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		wg.Add(1)
		go func(seatNumbers []string) {
			defer wg.Done()
			for _, seatNum := range seatNumbers {
				r := &models.ReserveSeatRequest{EventID: req.EventID, SeatNumber: seatNum}
				booking, err := s.ReserveSeat(userID, r)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				mu.Lock()
				successful = append(successful, booking)
				mu.Unlock()
			}
		}(chunk)
	}

	wg.Wait()

	if firstErr != nil {
		for _, b := range successful {
			_ = s.ReleaseExpiredBooking(b)
		}
		return nil, fmt.Errorf("bulk reserve failed: %w", firstErr)
	}

	return successful, nil
}

func (s *BookingService) GetUserBookings(userID uuid.UUID) ([]*models.BookingWithDetails, error) {
	return s.bookingRepo.FindByUserID(userID)
}

func (s *BookingService) CancelBooking(userID uuid.UUID, bookingID uuid.UUID) error {
	booking, err := s.bookingRepo.FindByID(bookingID)
	if err != nil {
		if err == repository.ErrBookingNotFound {
			return errors.New("booking not found")
		}
		return err
	}

	if booking.UserID != userID {
		return errors.New("unauthorized")
	}

	if booking.Status != models.BookingStatusReserved {
		return errors.New("only reserved bookings can be cancelled")
	}

	if err := s.bookingRepo.UpdateStatus(bookingID, models.BookingStatusCancelled); err != nil {
		return err
	}

	if err := s.seatRepo.ReleaseSeat(booking.SeatID); err != nil {
		return err
	}

	if err := s.eventRepo.IncrementAvailableSeats(booking.EventID); err != nil {
		return err
	}

	return nil
}

func (s *BookingService) processPayment(booking *models.Booking) error {
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (s *BookingService) CleanupExpiredReservations(ctx context.Context) error {
	expiredBookings, err := s.bookingRepo.FindExpiredReservations()
	if err != nil {
		return err
	}

	for _, booking := range expiredBookings {
		if err := s.ReleaseExpiredBooking(booking); err != nil {
			fmt.Printf("Error releasing expired booking %s: %v\n", booking.ID, err)
		}
	}

	return nil
}
