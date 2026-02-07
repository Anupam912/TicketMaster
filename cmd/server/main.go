package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/database"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/redis"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/routes"
	"event-ticketing-system/internal/services"
	"event-ticketing-system/internal/websocket"

	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	var redisClient *redis.Client
	redisClient, err = redis.Connect(cfg)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v. Continuing without caching/rate limiting.", err)
		redisClient = nil
	} else {
		defer redis.Close()
		log.Println("Redis connected successfully")
	}

	hub := websocket.NewHub()
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bookingRepo := repository.NewBookingRepository()
	eventRepo := repository.NewEventRepository()
	seatRepo := repository.NewSeatRepository()
	expiryQueue := queue.NewExpiryQueue(redisClient)
	bookingService := services.NewBookingService(bookingRepo, eventRepo, seatRepo, cfg, expiryQueue)

	if redisClient != nil {
		go services.NewBatchReleaseWorker(bookingService, expiryQueue).Run(ctx)
		log.Println("Batch release worker started")
	} else {
		go startCleanupJob(ctx, bookingService)
	}

	if redisClient != nil {
		q := queue.NewQueue(redisClient)

		bookingWorker := services.NewBookingWorker(bookingService, q, hub, cfg)
		go bookingWorker.StartBookingWorker(ctx)
		log.Println("Booking worker started")

		purchaseWorker := services.NewPurchaseWorker(bookingService, q, hub)
		go purchaseWorker.StartPurchaseWorker(ctx)
		log.Println("Purchase worker started")
	}

	router := routes.SetupRoutes(cfg, redisClient, hub)

	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Server starting on %s", addr)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := router.Run(addr); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-sigChan
	log.Println("Shutting down server...")
	cancel()
	time.Sleep(2 * time.Second)
}

func startCleanupJob(ctx context.Context, bookingService *services.BookingService) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := bookingService.CleanupExpiredReservations(ctx); err != nil {
				log.Printf("Error cleaning up expired reservations: %v", err)
			}
		}
	}
}
