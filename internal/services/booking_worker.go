package services

import (
	"context"
	"encoding/json"
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
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Booking worker shutting down...")
			return
		default:
			job, err := w.queue.DequeueBookingJob(ctx, consumerGroup)
			if err != nil {
				log.Printf("Error dequeuing booking job: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if job == nil {
				continue
			}

			if err := w.processBookingJob(ctx, job); err != nil {
				log.Printf("Error processing booking job %s: %v", job.ID, err)
			}
		}
	}
}

func (w *BookingWorker) processBookingJob(ctx context.Context, job *queue.BookingJob) error {
	req := &models.ReserveSeatRequest{
		EventID:    job.EventID,
		SeatNumber: job.SeatNumber,
	}

	_, err := w.bookingService.ReserveSeat(job.UserID, req)
	if err != nil {
		return fmt.Errorf("failed to reserve seat: %w", err)
	}

	if w.hub != nil {
		seatRepo := repository.NewSeatRepository()
		seat, err := seatRepo.FindByEventAndSeatNumber(job.EventID, job.SeatNumber)
		if err == nil && seat != nil {
			w.hub.BroadcastSeatUpdate(job.EventID, seat.ID, "reserved")
		}
	}

	log.Printf("Successfully processed booking job %s for user %s", job.ID, job.UserID)
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
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Purchase worker shutting down...")
			return
		default:
			job, err := w.queue.DequeuePurchaseJob(ctx, consumerGroup)
			if err != nil {
				log.Printf("Error dequeuing purchase job: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if job == nil {
				continue
			}

			if err := w.processPurchaseJob(ctx, job); err != nil {
				log.Printf("Error processing purchase job %s: %v", job.ID, err)
			}
		}
	}
}

func (w *PurchaseWorker) processPurchaseJob(ctx context.Context, job *queue.PurchaseJob) error {
	booking, err := w.bookingService.PurchaseBooking(job.UserID, job.BookingID)
	if err != nil {
		return fmt.Errorf("failed to purchase booking: %w", err)
	}

	if w.hub != nil {
		w.hub.BroadcastSeatUpdate(booking.EventID, booking.SeatID, "sold")
	}

	log.Printf("Successfully processed purchase job %s for booking %s", job.ID, job.BookingID)
	return nil
}
