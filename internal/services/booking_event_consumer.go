package services

import (
	"context"
	"errors"
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
				if errors.Is(err, context.Canceled) {
					return
				}
				if consumed != nil {
					c.deadLetterThenAck(ctx, consumed, err.Error(), "booking event fetch/decode")
					continue
				}
				log.Printf("Error fetching booking event: %v", err)
				if !waitWithContext(ctx, bookingEventConsumerBackoff) {
					return
				}
				continue
			}
			if consumed == nil {
				if !waitWithContext(ctx, bookingEventConsumerBackoff) {
					return
				}
				continue
			}

			if err := c.handle(ctx, consumed.Event); err != nil {
				c.deadLetterThenAck(ctx, consumed, err.Error(), "booking event handler")
				continue
			}

			if err := c.consumer.Ack(ctx, consumed); err != nil {
				log.Printf(
					"Booking event ack failed (partition=%d offset=%d): %v",
					consumed.Partition,
					consumed.Offset,
					err,
				)
			}
		}
	}
}

// deadLetterThenAck publishes a failed message to the DLQ and commits the offset only when publish succeeds.
func (c *BookingEventConsumer) deadLetterThenAck(
	ctx context.Context,
	consumed *kafka.ConsumedBookingEvent,
	reason string,
	stage string,
) {
	if err := c.consumer.DeadLetter(ctx, consumed, reason); err != nil {
		log.Printf(
			"%s: DLQ publish failed (partition=%d offset=%d): %v; message not acked",
			stage,
			consumed.Partition,
			consumed.Offset,
			err,
		)
		return
	}

	if err := c.consumer.Ack(ctx, consumed); err != nil {
		log.Printf(
			"%s: ack failed after DLQ (partition=%d offset=%d): %v",
			stage,
			consumed.Partition,
			consumed.Offset,
			err,
		)
		return
	}

	log.Printf(
		"%s: moved to DLQ and acked (partition=%d offset=%d): %s",
		stage,
		consumed.Partition,
		consumed.Offset,
		reason,
	)
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
		c.cacheInvalidator(ctx, eventID)
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

// waitWithContext sleeps for d or until ctx is cancelled.
func waitWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
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
