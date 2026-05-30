package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"event-ticketing-system/internal/kafka"
	"event-ticketing-system/internal/websocket"

	"github.com/google/uuid"
)

const bookingEventConsumerBackoff = time.Second

type BookingEventConsumer struct {
	consumer         *kafka.BookingEventConsumer
	cacheInvalidator CacheInvalidator
	hub              *websocket.Hub
}

func NewBookingEventConsumer(
	consumer *kafka.BookingEventConsumer,
	cacheInvalidator CacheInvalidator,
	hub *websocket.Hub,
) *BookingEventConsumer {
	return &BookingEventConsumer{
		consumer:         consumer,
		cacheInvalidator: cacheInvalidator,
		hub:              hub,
	}
}

func (c *BookingEventConsumer) Run(ctx context.Context) {
	if c == nil || c.consumer == nil || !c.consumer.Enabled() {
		return
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Booking event consumer shutting down...")
			return
		default:
			consumed, err := c.consumer.Fetch(ctx)
			if err != nil {
				if consumed != nil {
					_ = c.consumer.DeadLetter(ctx, consumed, err.Error())
					_ = c.consumer.Ack(ctx, consumed)
					log.Printf("Booking event moved to DLQ: %v", err)
					continue
				}
				log.Printf("Error fetching booking event: %v", err)
				time.Sleep(bookingEventConsumerBackoff)
				continue
			}
			if consumed == nil {
				continue
			}

			if err := c.handle(ctx, consumed.Event); err != nil {
				_ = c.consumer.DeadLetter(ctx, consumed, err.Error())
				_ = c.consumer.Ack(ctx, consumed)
				log.Printf("Booking event handler failed and event moved to DLQ: %v", err)
				continue
			}

			if err := c.consumer.Ack(ctx, consumed); err != nil {
				log.Printf("Error acking booking event: %v", err)
			}
		}
	}
}

func (c *BookingEventConsumer) handle(ctx context.Context, event *kafka.BookingEvent) error {
	if event == nil {
		return fmt.Errorf("booking event is nil")
	}

	eventID, err := uuid.Parse(event.EventID)
	if err != nil {
		return fmt.Errorf("invalid event_id %q: %w", event.EventID, err)
	}
	seatID, err := uuid.Parse(event.SeatID)
	if err != nil {
		return fmt.Errorf("invalid seat_id %q: %w", event.SeatID, err)
	}

	if c.cacheInvalidator != nil {
		c.cacheInvalidator(eventID)
	}

	if c.hub != nil {
		status, ok := seatStatusForBookingEvent(event.EventType)
		if !ok {
			return fmt.Errorf("unsupported booking event type %q", event.EventType)
		}
		c.hub.BroadcastSeatUpdate(eventID, seatID, status)
	}

	log.Printf(
		"Consumed booking event type=%s booking_id=%s event_id=%s seat_id=%s",
		event.EventType,
		event.BookingID,
		event.EventID,
		event.SeatID,
	)

	return nil
}

func seatStatusForBookingEvent(eventType string) (string, bool) {
	switch eventType {
	case "booking.reserved":
		return "reserved", true
	case "booking.purchased":
		return "sold", true
	case "booking.cancelled", "booking.expired":
		return "available", true
	default:
		return "", false
	}
}
