package services

import (
	"context"
	"fmt"
	"log"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/websocket"
)

type BookingWorker struct {
	bookingService *BookingService
	seatRepo       *repository.SeatRepository
	queue          *queue.Queue
	hub            *websocket.Hub
	config         *config.Config
}

func NewBookingWorker(
	bookingService *BookingService,
	seatRepo *repository.SeatRepository,
	q *queue.Queue,
	hub *websocket.Hub,
	cfg *config.Config,
) *BookingWorker {
	return &BookingWorker{
		bookingService: bookingService,
		seatRepo:       seatRepo,
		queue:          q,
		hub:            hub,
		config:         cfg,
	}
}

func (w *BookingWorker) StartBookingWorker(ctx context.Context) {
	log.Println("Starting booking worker with ConsumerGroup API...")
	
	handler := func(ctx context.Context, job *queue.BookingJob, messageID string) error {
		return w.processBookingJob(ctx, job, messageID)
	}
	
	if err := w.queue.StartBookingConsumer(ctx, handler); err != nil {
		log.Printf("Failed to start booking consumer: %v", err)
	}
}

func (w *BookingWorker) processBookingJob(ctx context.Context, job *queue.BookingJob, messageID string) error {
	req := &models.ReserveSeatRequest{
		EventID:    job.EventID,
		SeatNumber: job.SeatNumber,
	}

	booking, err := w.bookingService.ReserveSeat(ctx, job.UserID, req)
	if err != nil {
		_ = w.queue.HandleBookingJobFailure(ctx, job, err.Error())
		return fmt.Errorf("failed to reserve seat: %w", err)
	}

	_ = w.queue.CompleteJob(ctx, job.ID, booking.ID)

	if w.hub != nil {
		seat, err := w.seatRepo.FindByEventAndSeatNumber(job.EventID, job.SeatNumber)
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
	log.Println("Starting purchase worker with ConsumerGroup API...")
	
	handler := func(ctx context.Context, job *queue.PurchaseJob, messageID string) error {
		return w.processPurchaseJob(ctx, job, messageID)
	}
	
	if err := w.queue.StartPurchaseConsumer(ctx, handler); err != nil {
		log.Printf("Failed to start purchase consumer: %v", err)
	}
}

func (w *PurchaseWorker) processPurchaseJob(ctx context.Context, job *queue.PurchaseJob, messageID string) error {
	booking, err := w.bookingService.PurchaseBooking(ctx, job.UserID, job.BookingID)
	if err != nil {
		_ = w.queue.HandlePurchaseJobFailure(ctx, job, err.Error())
		return fmt.Errorf("failed to purchase booking: %w", err)
	}

	_ = w.queue.CompleteJob(ctx, job.ID, booking.ID)

	if w.hub != nil {
		w.hub.BroadcastSeatUpdate(booking.EventID, booking.SeatID, "sold")
	}

	log.Printf("Successfully processed purchase job %s for booking %s", job.ID, job.BookingID)
	return nil
}
