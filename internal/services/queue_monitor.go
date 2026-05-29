package services

import (
	"context"
	"log"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/telemetry"
)

// QueueMonitor emits threshold-based operational warnings for queue pressure.
type QueueMonitor struct {
	queue         *queue.Queue
	config        *config.Config
	bookingGroup  string
	purchaseGroup string
}

func NewQueueMonitor(q *queue.Queue, cfg *config.Config, bookingGroup, purchaseGroup string) *QueueMonitor {
	return &QueueMonitor{
		queue:         q,
		config:        cfg,
		bookingGroup:  bookingGroup,
		purchaseGroup: purchaseGroup,
	}
}

func (m *QueueMonitor) Run(ctx context.Context) {
	interval := time.Duration(m.config.Queue.MonitorIntervalSec) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	m.checkOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Queue monitor shutting down...")
			return
		case <-ticker.C:
			m.checkOnce(ctx)
		}
	}
}

func (m *QueueMonitor) checkOnce(ctx context.Context) {
	metrics, err := m.queue.GetMetrics(ctx, m.bookingGroup, m.purchaseGroup)
	if err != nil {
		telemetry.IncQueueMonitorError()
		log.Printf("component=queue_monitor level=error message=\"failed to fetch queue metrics\" error=%q", err.Error())
		return
	}
	telemetry.UpdateQueueMetrics(metrics)

	if metrics.BookingQueueLength >= m.config.Queue.AlertBookingQueueLength {
		telemetry.IncQueueAlert("booking_queue_lag")
		log.Printf("component=queue_monitor level=warn signal=booking_queue_lag value=%d threshold=%d", metrics.BookingQueueLength, m.config.Queue.AlertBookingQueueLength)
	}
	if metrics.PurchaseQueueLength >= m.config.Queue.AlertPurchaseQueueLength {
		telemetry.IncQueueAlert("purchase_queue_lag")
		log.Printf("component=queue_monitor level=warn signal=purchase_queue_lag value=%d threshold=%d", metrics.PurchaseQueueLength, m.config.Queue.AlertPurchaseQueueLength)
	}
	if metrics.BookingPending >= m.config.Queue.AlertBookingPending {
		telemetry.IncQueueAlert("booking_pending")
		log.Printf("component=queue_monitor level=warn signal=booking_pending value=%d threshold=%d", metrics.BookingPending, m.config.Queue.AlertBookingPending)
	}
	if metrics.PurchasePending >= m.config.Queue.AlertPurchasePending {
		telemetry.IncQueueAlert("purchase_pending")
		log.Printf("component=queue_monitor level=warn signal=purchase_pending value=%d threshold=%d", metrics.PurchasePending, m.config.Queue.AlertPurchasePending)
	}
	if metrics.BookingDLQLength >= m.config.Queue.AlertBookingDLQ {
		telemetry.IncQueueAlert("booking_dlq")
		log.Printf("component=queue_monitor level=warn signal=booking_dlq value=%d threshold=%d", metrics.BookingDLQLength, m.config.Queue.AlertBookingDLQ)
	}
	if metrics.PurchaseDLQLength >= m.config.Queue.AlertPurchaseDLQ {
		telemetry.IncQueueAlert("purchase_dlq")
		log.Printf("component=queue_monitor level=warn signal=purchase_dlq value=%d threshold=%d", metrics.PurchaseDLQLength, m.config.Queue.AlertPurchaseDLQ)
	}
}
