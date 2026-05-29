package telemetry

import (
	"fmt"
	"strings"
	"sync/atomic"

	"event-ticketing-system/internal/queue"
)

var (
	bookingQueueLength  atomic.Int64
	purchaseQueueLength atomic.Int64
	bookingDLQLength    atomic.Int64
	purchaseDLQLength   atomic.Int64
	bookingPending      atomic.Int64
	purchasePending     atomic.Int64

	queueMonitorErrors atomic.Uint64
	alertBookingQueue  atomic.Uint64
	alertPurchaseQueue atomic.Uint64
	alertBookingPend   atomic.Uint64
	alertPurchasePend  atomic.Uint64
	alertBookingDLQ    atomic.Uint64
	alertPurchaseDLQ   atomic.Uint64
)

func UpdateQueueMetrics(m *queue.QueueMetrics) {
	if m == nil {
		return
	}
	bookingQueueLength.Store(m.BookingQueueLength)
	purchaseQueueLength.Store(m.PurchaseQueueLength)
	bookingDLQLength.Store(m.BookingDLQLength)
	purchaseDLQLength.Store(m.PurchaseDLQLength)
	bookingPending.Store(m.BookingPending)
	purchasePending.Store(m.PurchasePending)
}

func IncQueueMonitorError() {
	queueMonitorErrors.Add(1)
}

func IncQueueAlert(signal string) {
	switch signal {
	case "booking_queue_lag":
		alertBookingQueue.Add(1)
	case "purchase_queue_lag":
		alertPurchaseQueue.Add(1)
	case "booking_pending":
		alertBookingPend.Add(1)
	case "purchase_pending":
		alertPurchasePend.Add(1)
	case "booking_dlq":
		alertBookingDLQ.Add(1)
	case "purchase_dlq":
		alertPurchaseDLQ.Add(1)
	}
}

func RenderPrometheus() string {
	var b strings.Builder

	b.WriteString("# HELP ticketmaster_queue_booking_length Current booking queue stream length.\n")
	b.WriteString("# TYPE ticketmaster_queue_booking_length gauge\n")
	b.WriteString(fmt.Sprintf("ticketmaster_queue_booking_length %d\n", bookingQueueLength.Load()))

	b.WriteString("# HELP ticketmaster_queue_purchase_length Current purchase queue stream length.\n")
	b.WriteString("# TYPE ticketmaster_queue_purchase_length gauge\n")
	b.WriteString(fmt.Sprintf("ticketmaster_queue_purchase_length %d\n", purchaseQueueLength.Load()))

	b.WriteString("# HELP ticketmaster_queue_booking_pending Current booking pending entries for consumer group.\n")
	b.WriteString("# TYPE ticketmaster_queue_booking_pending gauge\n")
	b.WriteString(fmt.Sprintf("ticketmaster_queue_booking_pending %d\n", bookingPending.Load()))

	b.WriteString("# HELP ticketmaster_queue_purchase_pending Current purchase pending entries for consumer group.\n")
	b.WriteString("# TYPE ticketmaster_queue_purchase_pending gauge\n")
	b.WriteString(fmt.Sprintf("ticketmaster_queue_purchase_pending %d\n", purchasePending.Load()))

	b.WriteString("# HELP ticketmaster_queue_booking_dlq_length Current booking DLQ stream length.\n")
	b.WriteString("# TYPE ticketmaster_queue_booking_dlq_length gauge\n")
	b.WriteString(fmt.Sprintf("ticketmaster_queue_booking_dlq_length %d\n", bookingDLQLength.Load()))

	b.WriteString("# HELP ticketmaster_queue_purchase_dlq_length Current purchase DLQ stream length.\n")
	b.WriteString("# TYPE ticketmaster_queue_purchase_dlq_length gauge\n")
	b.WriteString(fmt.Sprintf("ticketmaster_queue_purchase_dlq_length %d\n", purchaseDLQLength.Load()))

	b.WriteString("# HELP ticketmaster_queue_monitor_errors_total Total queue monitor metric collection errors.\n")
	b.WriteString("# TYPE ticketmaster_queue_monitor_errors_total counter\n")
	b.WriteString(fmt.Sprintf("ticketmaster_queue_monitor_errors_total %d\n", queueMonitorErrors.Load()))

	b.WriteString("# HELP ticketmaster_queue_alerts_total Total threshold alert evaluations by signal.\n")
	b.WriteString("# TYPE ticketmaster_queue_alerts_total counter\n")
	b.WriteString(fmt.Sprintf("ticketmaster_queue_alerts_total{signal=\"booking_queue_lag\"} %d\n", alertBookingQueue.Load()))
	b.WriteString(fmt.Sprintf("ticketmaster_queue_alerts_total{signal=\"purchase_queue_lag\"} %d\n", alertPurchaseQueue.Load()))
	b.WriteString(fmt.Sprintf("ticketmaster_queue_alerts_total{signal=\"booking_pending\"} %d\n", alertBookingPend.Load()))
	b.WriteString(fmt.Sprintf("ticketmaster_queue_alerts_total{signal=\"purchase_pending\"} %d\n", alertPurchasePend.Load()))
	b.WriteString(fmt.Sprintf("ticketmaster_queue_alerts_total{signal=\"booking_dlq\"} %d\n", alertBookingDLQ.Load()))
	b.WriteString(fmt.Sprintf("ticketmaster_queue_alerts_total{signal=\"purchase_dlq\"} %d\n", alertPurchaseDLQ.Load()))

	return b.String()
}
