package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/websocket"

	"github.com/google/uuid"
)

type BookingWorker struct {
	bookingService *BookingService
	queue          *queue.Queue
	hub            *websocket.Hub
	config         *config.Config
}

func NewBookingWorker(
	bookingService *BookingService,
	q *queue.Queue,
	hub *websocket.Hub,
	cfg *config.Config,
) *BookingWorker {
	return &BookingWorker{
		bookingService: bookingService,
		queue:          q,
		hub:            hub,
		config:         cfg,
	}
}

func (w *BookingWorker) StartBookingWorker(ctx context.Context) {
	consumerGroup := "booking-workers"
	consumerID := fmt.Sprintf("booking-worker-%s", uuid.New().String()[:8])

	for {
		select {
		case <-ctx.Done():
			log.Println("Booking worker shutting down...")
			return
		default:
			job, messageID, err := w.queue.DequeueBookingJob(ctx, consumerGroup, consumerID)
			if err != nil {
				log.Printf("Error dequeuing booking job: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if job == nil {
				continue
			}

			if err := w.processBookingJob(ctx, consumerGroup, messageID, job); err != nil {
				log.Printf("Error processing booking job %s: %v", job.ID, err)
			}
		}
	}
}

func (w *BookingWorker) processBookingJob(ctx context.Context, consumerGroup, messageID string, job *queue.BookingJob) error {
	req := &models.ReserveSeatRequest{
		EventID:    job.EventID,
		SeatNumber: job.SeatNumber,
	}

	booking, err := w.bookingService.ReserveSeat(job.UserID, req)
	if err != nil {
		_ = w.queue.HandleBookingJobFailure(ctx, job, err.Error())
		_ = w.queue.AckBookingJob(ctx, consumerGroup, messageID)
		return fmt.Errorf("failed to reserve seat: %w", err)
	}

	_ = w.queue.CompleteJob(ctx, job.ID, booking.ID)
	_ = w.queue.AckBookingJob(ctx, consumerGroup, messageID)

	if w.hub != nil {
		seatRepo := repository.NewSeatRepository()
		seat, err := seatRepo.FindByEventAndSeatNumber(job.EventID, job.SeatNumber)
		if err == nil && seat != nil {
			w.hub.BroadcastSeatUpdate(job.EventID, seat.ID, "reserved")
		}
	}

	log.Printf("Successfully processed booking job %s for user %s, booking %s", job.ID, job.UserID, booking.ID)
	return nil
}

type PurchaseWorker struct {
	bookingService *BookingService
	queue          *queue.Queue
	hub            *websocket.Hub
}

func NewPurchaseWorker(
	bookingService *BookingService,
	q *queue.Queue,
	hub *websocket.Hub,
) *PurchaseWorker {
	return &PurchaseWorker{
		bookingService: bookingService,
		queue:          q,
		hub:            hub,
	}
}

func (w *PurchaseWorker) StartPurchaseWorker(ctx context.Context) {
	consumerGroup := "purchase-workers"
	consumerID := fmt.Sprintf("purchase-worker-%s", uuid.New().String()[:8])

	for {
		select {
		case <-ctx.Done():
			log.Println("Purchase worker shutting down...")
			return
		default:
			job, messageID, err := w.queue.DequeuePurchaseJob(ctx, consumerGroup, consumerID)
			if err != nil {
				log.Printf("Error dequeuing purchase job: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if job == nil {
				continue
			}

			if err := w.processPurchaseJob(ctx, consumerGroup, messageID, job); err != nil {
				log.Printf("Error processing purchase job %s: %v", job.ID, err)
			}
		}
	}
}

func (w *PurchaseWorker) processPurchaseJob(ctx context.Context, consumerGroup, messageID string, job *queue.PurchaseJob) error {
	booking, err := w.bookingService.PurchaseBooking(job.UserID, job.BookingID)
	if err != nil {
		_ = w.queue.HandlePurchaseJobFailure(ctx, job, err.Error())
		_ = w.queue.AckPurchaseJob(ctx, consumerGroup, messageID)
		return fmt.Errorf("failed to purchase booking: %w", err)
	}

	_ = w.queue.CompleteJob(ctx, job.ID, booking.ID)
	_ = w.queue.AckPurchaseJob(ctx, consumerGroup, messageID)

	if w.hub != nil {
		w.hub.BroadcastSeatUpdate(booking.EventID, booking.SeatID, "sold")
	}

	log.Printf("Successfully processed purchase job %s for booking %s", job.ID, job.BookingID)
	return nil
}
