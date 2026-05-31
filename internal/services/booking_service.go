package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/database"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/repository"

	"github.com/google/uuid"
)

// Configuration constants.
const (
	maxBulkSeats                = 500
	bulkChunkSize               = 50
	purchaseProcessingExtension = 2 * time.Minute
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
type CacheInvalidator func(ctx context.Context, eventID uuid.UUID)

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

func (s *BookingService) invalidateCache(ctx context.Context, eventID uuid.UUID) {
	if s.cacheInvalidator != nil {
		s.cacheInvalidator(ctx, eventID)
	}
}

// ReserveSeat reserves a specific seat for a user with a time-limited hold.
func (s *BookingService) ReserveSeat(ctx context.Context, userID uuid.UUID, req *models.ReserveSeatRequest) (*models.Booking, error) {
	expiresAt := time.Now().Add(s.config.Booking.ReservationTimeout())
	booking, err := s.bookingRepo.CreateReservation(userID, req.EventID, req.SeatNumber, expiresAt)
	if err != nil {
		if errors.Is(err, repository.ErrSeatAlreadyBooked) {
			return nil, ErrSeatBeingBooked
		}
		if errors.Is(err, repository.ErrSeatNotAvailable) {
			return nil, ErrSeatNotAvailable
		}
		if errors.Is(err, repository.ErrNoAvailableSeats) {
			return nil, ErrNoAvailableSeats
		}
		if errors.Is(err, repository.ErrEventNotFound) {
			return nil, repository.ErrEventNotFound
		}
		return nil, fmt.Errorf("create reservation: %w", err)
	}

	if s.expiryQueue != nil {
		if err := s.expiryQueue.Add(ctx, booking.ID.String(), booking.ExpiresAt); err != nil {
			log.Printf("Warning: failed to enqueue reservation expiry booking_id=%s: %v", booking.ID, err)
		}
	}

	s.markSeatReadPrimary(ctx, userID, req.EventID)
	s.invalidateCache(ctx, req.EventID)
	s.publishEvent(ctx, "booking.reserved", booking, map[string]interface{}{"source": "booking_service"})

	return booking, nil
}

// PurchaseBooking completes the purchase of a reserved booking.
func (s *BookingService) PurchaseBooking(ctx context.Context, userID, bookingID uuid.UUID) (*models.Booking, error) {
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
		_ = s.ReleaseExpiredBooking(ctx, booking)
		return nil, ErrBookingExpired
	}

	booking, err = s.bookingRepo.ExtendReservation(bookingID, time.Now().Add(purchaseProcessingExtension))
	if err != nil {
		if errors.Is(err, repository.ErrBookingExpired) {
			return nil, ErrBookingExpired
		}
		if errors.Is(err, repository.ErrBookingNotReserved) {
			return nil, ErrNotReserved
		}
		if errors.Is(err, repository.ErrSeatNotAvailable) {
			return nil, ErrSeatNotAvailable
		}
		return nil, fmt.Errorf("extend reservation for purchase: %w", err)
	}
	if s.expiryQueue != nil {
		if err := s.expiryQueue.Add(ctx, booking.ID.String(), booking.ExpiresAt); err != nil {
			log.Printf("Warning: failed to extend reservation expiry booking_id=%s: %v", booking.ID, err)
		}
	}

	booking, err = s.ensurePaymentIntent(ctx, booking)
	if err != nil {
		return nil, err
	}

	booking, err = s.bookingRepo.CompletePurchase(bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingExpired) {
			return nil, ErrBookingExpired
		}
		if errors.Is(err, repository.ErrBookingNotReserved) {
			updated, findErr := s.bookingRepo.FindByID(bookingID)
			if findErr == nil && updated.Status == models.BookingStatusPurchased {
				return s.finishPurchasedBookingPayment(ctx, updated)
			}
			return nil, ErrNotReserved
		}
		return nil, fmt.Errorf("complete purchase: %w", err)
	}

	if s.expiryQueue != nil {
		if err := s.expiryQueue.Remove(ctx, bookingID.String()); err != nil {
			log.Printf("Warning: failed to remove reservation expiry booking_id=%s: %v", bookingID, err)
		}
	}

	if err := s.processPayment(ctx, booking); err != nil {
		s.revertPurchaseAfterPaymentFailure(ctx, bookingID)
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	s.markSeatReadPrimary(ctx, userID, booking.EventID)
	s.invalidateCache(ctx, booking.EventID)
	s.publishEvent(ctx, "booking.purchased", booking, map[string]interface{}{"source": "booking_service"})

	return booking, nil
}

func (s *BookingService) ensurePaymentIntent(ctx context.Context, booking *models.Booking) (*models.Booking, error) {
	if s.paymentService == nil {
		return booking, nil
	}
	if booking.PaymentIntentID != nil && strings.TrimSpace(*booking.PaymentIntentID) != "" {
		return booking, nil
	}

	result, err := s.paymentService.PreparePaymentIntent(ctx, booking)
	if err != nil {
		return nil, fmt.Errorf("prepare payment intent: %w", err)
	}

	updated, err := s.bookingRepo.SetPaymentIntentID(booking.ID, result.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("store payment intent: %w", err)
	}

	return updated, nil
}

func (s *BookingService) finishPurchasedBookingPayment(ctx context.Context, booking *models.Booking) (*models.Booking, error) {
	booking, err := s.ensurePaymentIntent(ctx, booking)
	if err != nil {
		return nil, err
	}

	if err := s.processPayment(ctx, booking); err != nil {
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	s.markSeatReadPrimary(ctx, booking.UserID, booking.EventID)
	s.invalidateCache(ctx, booking.EventID)
	s.publishEvent(ctx, "booking.purchased", booking, map[string]interface{}{"source": "booking_service"})

	return booking, nil
}

func (s *BookingService) revertPurchaseAfterPaymentFailure(ctx context.Context, bookingID uuid.UUID) {
	reverted, err := s.bookingRepo.RevertFailedPurchase(bookingID)
	if err != nil {
		log.Printf(
			"CRITICAL: payment failed and purchase revert failed booking_id=%s: %v",
			bookingID, err,
		)
		return
	}
	s.markSeatReadPrimary(ctx, reverted.UserID, reverted.EventID)
	s.invalidateCache(ctx, reverted.EventID)
	s.publishEvent(ctx, "booking.cancelled", reverted, map[string]interface{}{
		"source": "booking_service",
		"reason": "payment_failed",
	})
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

	return s.ReleaseExpiredBooking(ctx, booking)
}

// ReleaseExpiredBooking releases an expired booking, freeing the seat and updating counts.
func (s *BookingService) ReleaseExpiredBooking(ctx context.Context, booking *models.Booking) error {
	released, err := s.bookingRepo.ReleaseExpiredReservation(booking.ID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotReserved) {
			return nil
		}
		return fmt.Errorf("release expired reservation: %w", err)
	}

	s.markSeatReadPrimary(ctx, booking.UserID, released.EventID)
	s.invalidateCache(ctx, released.EventID)
	s.publishEvent(ctx, "booking.expired", released, map[string]interface{}{"source": "booking_service"})

	return nil
}

// BulkReserve reserves multiple seats for a user in a single operation.
// Seats are processed in parallel chunks. If any reservation fails, all
// successful reservations are rolled back.
func (s *BookingService) BulkReserve(ctx context.Context, userID uuid.UUID, req *models.BulkReserveRequest) ([]*models.Booking, error) {
	seats := req.SeatNumbers
	if len(seats) == 0 {
		return nil, ErrNoSeatsRequested
	}
	if len(seats) > maxBulkSeats {
		return nil, ErrTooManySeats
	}

	chunks := s.chunkSeats(seats, bulkChunkSize)

	type reserveOutcome struct {
		booking *models.Booking
		err     error
	}

	outcomes := make(chan reserveOutcome, len(seats))
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		wg.Add(1)
		go func(seatNumbers []string) {
			defer wg.Done()
			for _, seatNum := range seatNumbers {
				r := &models.ReserveSeatRequest{EventID: req.EventID, SeatNumber: seatNum}
				booking, err := s.ReserveSeat(ctx, userID, r)
				outcomes <- reserveOutcome{booking: booking, err: err}
				if err != nil {
					return
				}
			}
		}(chunk)
	}

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	successful := make([]*models.Booking, 0, len(seats))
	var firstErr error
	for outcome := range outcomes {
		if outcome.err != nil {
			if firstErr == nil {
				firstErr = outcome.err
			}
			continue
		}
		successful = append(successful, outcome.booking)
	}

	if firstErr != nil {
		for _, b := range successful {
			if cancelled, err := s.bookingRepo.CancelReservation(b.ID); err == nil {
				s.invalidateCache(ctx, cancelled.EventID)
				if s.expiryQueue != nil {
					_ = s.expiryQueue.Remove(ctx, cancelled.ID.String())
				}
			}
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
func (s *BookingService) CancelBooking(ctx context.Context, userID, bookingID uuid.UUID) error {
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

	cancelled, err := s.bookingRepo.CancelReservation(bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotReserved) {
			return ErrOnlyReservedCancel
		}
		return fmt.Errorf("cancel reservation: %w", err)
	}

	if s.expiryQueue != nil {
		if err := s.expiryQueue.Remove(ctx, bookingID.String()); err != nil {
			log.Printf("Warning: failed to remove reservation expiry booking_id=%s: %v", bookingID, err)
		}
	}

	s.markSeatReadPrimary(ctx, userID, cancelled.EventID)
	s.invalidateCache(ctx, cancelled.EventID)
	s.publishEvent(ctx, "booking.cancelled", cancelled, map[string]interface{}{"source": "booking_service"})

	return nil
}

func (s *BookingService) markSeatReadPrimary(ctx context.Context, userID, eventID uuid.UUID) {
	database.MarkSeatReadPrimary(ctx, userID, eventID)
}

func (s *BookingService) processPayment(ctx context.Context, booking *models.Booking) error {
	if s.paymentService == nil {
		return nil
	}

	if booking.PaymentIntentID == nil || strings.TrimSpace(*booking.PaymentIntentID) == "" {
		return ErrPaymentIntentMissing
	}

	_, err := s.paymentService.ProcessPayment(ctx, booking)
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
		if err := s.ReleaseExpiredBooking(ctx, booking); err != nil {
			log.Printf("Error releasing expired booking %s: %v", booking.ID, err)
		}
	}

	return nil
}

func (s *BookingService) publishEvent(ctx context.Context, eventType string, booking *models.Booking, metadata map[string]interface{}) {
	if s.eventPublisher == nil || booking == nil {
		return
	}
	if err := s.eventPublisher.PublishBookingEvent(ctx, eventType, booking, metadata); err != nil {
		log.Printf("Warning: failed to publish booking event type=%s booking_id=%s: %v", eventType, booking.ID, err)
	}
}
