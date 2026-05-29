package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/repository"

	"github.com/google/uuid"
)

// Configuration constants.
const (
	maxBulkSeats  = 500
	bulkChunkSize = 50
)

// Sentinel errors for booking service operations.
var (
	ErrBookingNotFound    = errors.New("booking not found")
	ErrNoAvailableSeats   = errors.New("no available seats")
	ErrSeatBeingBooked    = errors.New("seat is currently being booked by another user")
	ErrSeatNotAvailable   = errors.New("seat is not available")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrNotReserved        = errors.New("booking is not in reserved status")
	ErrBookingExpired     = errors.New("booking has expired")
	ErrOnlyReservedCancel = errors.New("only reserved bookings can be cancelled")
	ErrNoSeatsRequested   = errors.New("no seats requested")
	ErrTooManySeats       = errors.New("maximum 500 seats per bulk request")
)

// CacheInvalidator is a callback function type for invalidating event cache.
type CacheInvalidator func(eventID uuid.UUID)

type BookingEventPublisher interface {
	PublishBookingEvent(ctx context.Context, eventType string, booking *models.Booking, metadata map[string]interface{}) error
}

// BookingService handles booking-related business logic.
type BookingService struct {
	bookingRepo      *repository.BookingRepository
	eventRepo        *repository.EventRepository
	seatRepo         *repository.SeatRepository
	config           *config.Config
	expiryQueue      *queue.ExpiryQueue
	cacheInvalidator CacheInvalidator
	paymentService   *PaymentService
	eventPublisher   BookingEventPublisher
}

// NewBookingService creates a new BookingService instance.
func NewBookingService(
	bookingRepo *repository.BookingRepository,
	eventRepo *repository.EventRepository,
	seatRepo *repository.SeatRepository,
	cfg *config.Config,
	expiryQueue *queue.ExpiryQueue,
	paymentService *PaymentService,
) *BookingService {
	return &BookingService{
		bookingRepo:    bookingRepo,
		eventRepo:      eventRepo,
		seatRepo:       seatRepo,
		config:         cfg,
		expiryQueue:    expiryQueue,
		paymentService: paymentService,
	}
}

// SetCacheInvalidator sets the function to call when event cache needs invalidation.
func (s *BookingService) SetCacheInvalidator(invalidator CacheInvalidator) {
	s.cacheInvalidator = invalidator
}

func (s *BookingService) SetEventPublisher(publisher BookingEventPublisher) {
	s.eventPublisher = publisher
}

func (s *BookingService) invalidateCache(eventID uuid.UUID) {
	if s.cacheInvalidator != nil {
		s.cacheInvalidator(eventID)
	}
}

// ReserveSeat reserves a specific seat for a user with a time-limited hold.
func (s *BookingService) ReserveSeat(userID uuid.UUID, req *models.ReserveSeatRequest) (*models.Booking, error) {
	event, err := s.eventRepo.FindByID(req.EventID)
	if err != nil {
		if errors.Is(err, repository.ErrEventNotFound) {
			return nil, repository.ErrEventNotFound
		}
		return nil, fmt.Errorf("find event: %w", err)
	}

	if event.AvailableSeats <= 0 {
		return nil, ErrNoAvailableSeats
	}

	expiresAt := time.Now().Add(s.config.Booking.ReservationTimeout())

	seat, err := s.seatRepo.ReserveSeatWithLock(req.EventID, req.SeatNumber, expiresAt)
	if err != nil {
		if errors.Is(err, repository.ErrSeatAlreadyBooked) {
			return nil, ErrSeatBeingBooked
		}
		if errors.Is(err, repository.ErrSeatNotAvailable) {
			return nil, ErrSeatNotAvailable
		}
		return nil, fmt.Errorf("reserve seat with lock: %w", err)
	}

	if err := s.eventRepo.DecrementAvailableSeats(req.EventID); err != nil {
		_ = s.seatRepo.ReleaseSeat(seat.ID)
		return nil, fmt.Errorf("decrement available seats: %w", err)
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
		_ = s.seatRepo.ReleaseSeat(seat.ID)
		_ = s.eventRepo.IncrementAvailableSeats(req.EventID)
		return nil, fmt.Errorf("create booking: %w", err)
	}

	if s.expiryQueue != nil {
		_ = s.expiryQueue.Add(context.Background(), booking.ID.String(), booking.ExpiresAt)
	}

	s.invalidateCache(req.EventID)
	s.publishEvent("booking.reserved", booking, map[string]interface{}{"source": "booking_service"})

	return booking, nil
}

// PurchaseBooking completes the purchase of a reserved booking.
func (s *BookingService) PurchaseBooking(userID, bookingID uuid.UUID) (*models.Booking, error) {
	booking, err := s.bookingRepo.FindByID(bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return nil, ErrBookingNotFound
		}
		return nil, fmt.Errorf("find booking: %w", err)
	}

	if booking.UserID != userID {
		return nil, ErrUnauthorized
	}

	if booking.Status != models.BookingStatusReserved {
		return nil, ErrNotReserved
	}

	if time.Now().After(booking.ExpiresAt) {
		_ = s.ReleaseExpiredBooking(booking)
		return nil, ErrBookingExpired
	}

	if err := s.processPayment(booking); err != nil {
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	if err := s.bookingRepo.UpdateStatus(bookingID, models.BookingStatusPurchased); err != nil {
		return nil, fmt.Errorf("update booking status: %w", err)
	}

	if s.expiryQueue != nil {
		_ = s.expiryQueue.Remove(context.Background(), bookingID.String())
	}

	if err := s.seatRepo.MarkAsSold(booking.SeatID); err != nil {
		return nil, fmt.Errorf("mark seat as sold: %w", err)
	}

	booking, err = s.bookingRepo.FindByID(bookingID)
	if err != nil {
		return nil, fmt.Errorf("fetch updated booking: %w", err)
	}
	s.publishEvent("booking.purchased", booking, map[string]interface{}{"source": "booking_service"})

	return booking, nil
}

// ReleaseExpiredBookingByID releases a booking by ID if it's expired and still reserved.
func (s *BookingService) ReleaseExpiredBookingByID(ctx context.Context, bookingID uuid.UUID) error {
	booking, err := s.bookingRepo.FindByID(bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return nil
		}
		return fmt.Errorf("find booking: %w", err)
	}

	if booking.Status != models.BookingStatusReserved {
		return nil
	}

	return s.ReleaseExpiredBooking(booking)
}

// ReleaseExpiredBooking releases an expired booking, freeing the seat and updating counts.
func (s *BookingService) ReleaseExpiredBooking(booking *models.Booking) error {
	if err := s.bookingRepo.UpdateStatus(booking.ID, models.BookingStatusExpired); err != nil {
		return fmt.Errorf("update booking status: %w", err)
	}

	if err := s.seatRepo.ReleaseSeat(booking.SeatID); err != nil {
		return fmt.Errorf("release seat: %w", err)
	}

	if err := s.eventRepo.IncrementAvailableSeats(booking.EventID); err != nil {
		return fmt.Errorf("increment available seats: %w", err)
	}

	s.invalidateCache(booking.EventID)
	s.publishEvent("booking.expired", booking, map[string]interface{}{"source": "booking_service"})

	return nil
}

// BulkReserve reserves multiple seats for a user in a single operation.
// Seats are processed in parallel chunks. If any reservation fails, all
// successful reservations are rolled back.
func (s *BookingService) BulkReserve(userID uuid.UUID, req *models.BulkReserveRequest) ([]*models.Booking, error) {
	seats := req.SeatNumbers
	if len(seats) == 0 {
		return nil, ErrNoSeatsRequested
	}
	if len(seats) > maxBulkSeats {
		return nil, ErrTooManySeats
	}

	chunks := s.chunkSeats(seats, bulkChunkSize)

	var (
		mu         sync.Mutex
		successful []*models.Booking
		firstErr   error
		wg         sync.WaitGroup
	)

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

func (s *BookingService) chunkSeats(seats []string, size int) [][]string {
	chunks := make([][]string, 0, (len(seats)+size-1)/size)
	for i := 0; i < len(seats); i += size {
		end := i + size
		if end > len(seats) {
			end = len(seats)
		}
		chunks = append(chunks, seats[i:end])
	}
	return chunks
}

// GetUserBookings retrieves all bookings for a specific user.
func (s *BookingService) GetUserBookings(userID uuid.UUID) ([]*models.BookingWithDetails, error) {
	bookings, err := s.bookingRepo.FindByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("find user bookings: %w", err)
	}
	return bookings, nil
}

// GetBookingByID retrieves a booking by ID, verifying the requesting user owns it.
func (s *BookingService) GetBookingByID(userID, bookingID uuid.UUID) (*models.Booking, error) {
	booking, err := s.bookingRepo.FindByID(bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return nil, ErrBookingNotFound
		}
		return nil, fmt.Errorf("find booking: %w", err)
	}

	if booking.UserID != userID {
		return nil, ErrUnauthorized
	}

	return booking, nil
}

// CancelBooking cancels a reserved booking and releases the seat.
func (s *BookingService) CancelBooking(userID, bookingID uuid.UUID) error {
	booking, err := s.bookingRepo.FindByID(bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return fmt.Errorf("find booking: %w", err)
	}

	if booking.UserID != userID {
		return ErrUnauthorized
	}

	if booking.Status != models.BookingStatusReserved {
		return ErrOnlyReservedCancel
	}

	if err := s.bookingRepo.UpdateStatus(bookingID, models.BookingStatusCancelled); err != nil {
		return fmt.Errorf("update booking status: %w", err)
	}

	if err := s.seatRepo.ReleaseSeat(booking.SeatID); err != nil {
		return fmt.Errorf("release seat: %w", err)
	}

	if err := s.eventRepo.IncrementAvailableSeats(booking.EventID); err != nil {
		return fmt.Errorf("increment available seats: %w", err)
	}

	s.invalidateCache(booking.EventID)
	s.publishEvent("booking.cancelled", booking, map[string]interface{}{"source": "booking_service"})

	return nil
}

func (s *BookingService) processPayment(booking *models.Booking) error {
	if s.paymentService == nil {
		return nil
	}

	_, err := s.paymentService.ProcessPayment(context.Background(), booking)
	if err != nil {
		return fmt.Errorf("process payment: %w", err)
	}

	return nil
}

// CleanupExpiredReservations finds and releases all expired reservations.
func (s *BookingService) CleanupExpiredReservations(ctx context.Context) error {
	expiredBookings, err := s.bookingRepo.FindExpiredReservations()
	if err != nil {
		return fmt.Errorf("find expired reservations: %w", err)
	}

	for _, booking := range expiredBookings {
		if err := s.ReleaseExpiredBooking(booking); err != nil {
			log.Printf("Error releasing expired booking %s: %v", booking.ID, err)
		}
	}

	return nil
}

func (s *BookingService) publishEvent(eventType string, booking *models.Booking, metadata map[string]interface{}) {
	if s.eventPublisher == nil || booking == nil {
		return
	}
	if err := s.eventPublisher.PublishBookingEvent(context.Background(), eventType, booking, metadata); err != nil {
		log.Printf("Warning: failed to publish booking event type=%s booking_id=%s: %v", eventType, booking.ID, err)
	}
}
