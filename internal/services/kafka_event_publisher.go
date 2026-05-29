package services

import (
	"context"
	"time"

	"event-ticketing-system/internal/kafka"
	"event-ticketing-system/internal/models"
)

type KafkaEventPublisher struct {
	producer *kafka.Producer
}

func NewKafkaEventPublisher(producer *kafka.Producer) *KafkaEventPublisher {
	return &KafkaEventPublisher{producer: producer}
}

func (p *KafkaEventPublisher) PublishBookingEvent(ctx context.Context, eventType string, booking *models.Booking, metadata map[string]interface{}) error {
	if p == nil || p.producer == nil || !p.producer.Enabled() || booking == nil {
		return nil
	}

	occurredAt := time.Now().UTC()
	if !booking.UpdatedAt.IsZero() {
		occurredAt = booking.UpdatedAt.UTC()
	}

	event := &kafka.BookingEvent{
		EventType:   eventType,
		BookingID:   booking.ID.String(),
		UserID:      booking.UserID.String(),
		EventID:     booking.EventID.String(),
		SeatID:      booking.SeatID.String(),
		Status:      booking.Status,
		TotalAmount: booking.TotalAmount,
		OccurredAt:  occurredAt,
		Metadata:    metadata,
	}

	return p.producer.PublishBookingEvent(ctx, event)
}
