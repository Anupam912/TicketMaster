package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/queue"

	"github.com/google/uuid"
)

const releaseWorkerPollInterval = 10 * time.Second

type expiryReleasePartitionPlan struct {
	concurrency int
	count       int
	offset      int
}

type BatchReleaseWorker struct {
	bookingService *BookingService
	expiryQueue    *queue.ExpiryQueue
	partitions     expiryReleasePartitionPlan
}

func NewBatchReleaseWorker(
	bookingService *BookingService,
	expiryQueue *queue.ExpiryQueue,
	cfg *config.Config,
) (*BatchReleaseWorker, error) {
	plan, err := expiryReleasePartitionPlanFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &BatchReleaseWorker{
		bookingService: bookingService,
		expiryQueue:    expiryQueue,
		partitions:     plan,
	}, nil
}

func expiryReleasePartitionPlanFromConfig(cfg *config.Config) (expiryReleasePartitionPlan, error) {
	if cfg == nil {
		return expiryReleasePartitionPlan{}, fmt.Errorf("config is required")
	}

	concurrency := cfg.Booking.ExpiryReleaseConcurrency
	if concurrency < 1 {
		concurrency = 4
	}

	count := cfg.Booking.ExpiryReleasePartitionCount
	if count <= 0 {
		count = concurrency
	}

	offset := cfg.Booking.ExpiryReleasePartitionOffset
	if offset < 0 {
		return expiryReleasePartitionPlan{}, fmt.Errorf("EXPIRY_RELEASE_PARTITION_OFFSET must be >= 0")
	}
	if offset+concurrency > count {
		return expiryReleasePartitionPlan{}, fmt.Errorf(
			"partition offset %d + concurrency %d exceeds partition count %d",
			offset, concurrency, count,
		)
	}

	return expiryReleasePartitionPlan{
		concurrency: concurrency,
		count:       count,
		offset:      offset,
	}, nil
}

func (w *BatchReleaseWorker) Run(ctx context.Context) {
	if w.expiryQueue == nil {
		log.Println("BatchReleaseWorker: Redis expiry queue not available, skipping")
		return
	}

	log.Printf(
		"BatchReleaseWorker started: %d workers, partitions [%d,%d) of %d",
		w.partitions.concurrency,
		w.partitions.offset,
		w.partitions.offset+w.partitions.concurrency,
		w.partitions.count,
	)

	var wg sync.WaitGroup
	for i := 0; i < w.partitions.concurrency; i++ {
		partitionIndex := w.partitions.offset + i
		wg.Add(1)
		go func(workerID int, partitionIndex int) {
			defer wg.Done()
			w.runWorker(ctx, workerID, partitionIndex)
		}(i, partitionIndex)
	}
	wg.Wait()
}

func (w *BatchReleaseWorker) runWorker(ctx context.Context, workerID int, partitionIndex int) {
	ticker := time.NewTicker(releaseWorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processDue(ctx, workerID, partitionIndex)
		}
	}
}

func (w *BatchReleaseWorker) processDue(ctx context.Context, workerID int, partitionIndex int) {
	ids, err := w.expiryQueue.PollDue(
		ctx,
		time.Now(),
		queue.ExpiryPollBatchSize,
		partitionIndex,
		w.partitions.count,
	)
	if err != nil {
		log.Printf("BatchReleaseWorker[%d] partition %d PollDue error: %v", workerID, partitionIndex, err)
		return
	}
	for _, idStr := range ids {
		bookingID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		if err := w.bookingService.ReleaseExpiredBookingByID(ctx, bookingID); err != nil {
			log.Printf(
				"BatchReleaseWorker[%d] partition %d release booking %s: %v",
				workerID, partitionIndex, idStr, err,
			)
			continue
		}
		if err := w.expiryQueue.Remove(ctx, idStr); err != nil {
			log.Printf(
				"BatchReleaseWorker[%d] partition %d ack expiry %s: %v",
				workerID, partitionIndex, idStr, err,
			)
		}
	}
}
