package services

import (
	"context"
	"log"
	"sync"
	"time"

	"event-ticketing-system/internal/queue"

	"github.com/google/uuid"
)

const (
	releaseWorkerPollInterval = 10 * time.Second
	releaseWorkerConcurrency  = 4
)

type BatchReleaseWorker struct {
	bookingService *BookingService
	expiryQueue    *queue.ExpiryQueue
}

func NewBatchReleaseWorker(bookingService *BookingService, expiryQueue *queue.ExpiryQueue) *BatchReleaseWorker {
	return &BatchReleaseWorker{
		bookingService: bookingService,
		expiryQueue:    expiryQueue,
	}
}

func (w *BatchReleaseWorker) Run(ctx context.Context) {
	if w.expiryQueue == nil {
		log.Println("BatchReleaseWorker: Redis expiry queue not available, skipping")
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < releaseWorkerConcurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w.runWorker(ctx, workerID)
		}(i)
	}
	wg.Wait()
}

func (w *BatchReleaseWorker) runWorker(ctx context.Context, workerID int) {
	ticker := time.NewTicker(releaseWorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processDue(ctx, workerID)
		}
	}
}

func (w *BatchReleaseWorker) processDue(ctx context.Context, workerID int) {
	ids, err := w.expiryQueue.PollDue(ctx, time.Now(), queue.ExpiryPollBatchSize)
	if err != nil {
		log.Printf("BatchReleaseWorker[%d] PollDue error: %v", workerID, err)
		return
	}
	for _, idStr := range ids {
		bookingID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		if err := w.bookingService.ReleaseExpiredBookingByID(ctx, bookingID); err != nil {
			log.Printf("BatchReleaseWorker[%d] release booking %s: %v", workerID, idStr, err)
			continue
		}
		if err := w.expiryQueue.Remove(ctx, idStr); err != nil {
			log.Printf("BatchReleaseWorker[%d] ack expiry %s: %v", workerID, idStr, err)
		}
	}
}
