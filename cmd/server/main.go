package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/database"
	"event-ticketing-system/internal/kafka"
	"event-ticketing-system/internal/queue"
	appredis "event-ticketing-system/internal/redis"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/routes"
	"event-ticketing-system/internal/services"
	"event-ticketing-system/internal/websocket"

	_ "event-ticketing-system/docs"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// @title          Event Ticketing System API
// @version        1.0
// @description    A high-concurrency event ticketing system with seat booking, Stripe payments, real-time WebSocket updates, and async processing via Redis Streams.

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your JWT token (Bearer prefix added automatically)

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
	redisClient, err = appredis.Connect(cfg)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v. Continuing without caching/rate limiting.", err)
		redisClient = nil
	} else {
		defer appredis.Close()
		log.Println("Redis connected successfully")
	}

	hub := websocket.NewHubWithConfig(cfg)
	hub.SetRedis(redisClient)
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.StartFanout(ctx)

	bookingRepo := repository.NewBookingRepository()
	eventRepo := repository.NewEventRepository()
	seatRepo := repository.NewSeatRepository()
	venueRepo := repository.NewVenueRepository()
	expiryQueue := queue.NewExpiryQueue(redisClient)

	paymentService := services.NewPaymentService(cfg)
	if paymentService.IsEnabled() {
		log.Println("Stripe payment processing enabled")
	} else {
		log.Println("Stripe not configured, using simulated payments")
	}

	eventService := services.NewEventService(eventRepo, venueRepo, seatRepo, redisClient, cfg)
	bookingService := services.NewBookingService(bookingRepo, eventRepo, seatRepo, cfg, expiryQueue, paymentService)
	bookingService.SetCacheInvalidator(eventService.InvalidateEventCache)

	kafkaProducer, err := kafka.NewProducer(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize Kafka producer: %v. Continuing without Kafka events.", err)
	} else if kafkaProducer.Enabled() {
		defer kafkaProducer.Close()
		bookingService.SetEventPublisher(services.NewKafkaEventPublisher(kafkaProducer))
		log.Println("Kafka booking event publisher enabled")
	}

	if redisClient != nil {
		go services.NewBatchReleaseWorker(bookingService, expiryQueue).Run(ctx)
		log.Println("Batch release worker started")
	}
	go startCleanupJob(ctx, bookingService)

	q := queue.NewQueue(cfg, redisClient)
	q.SetMaxRetries(cfg.Queue.MaxRetries)
	if q.Enabled() {
		bookingWorker := services.NewBookingWorker(bookingService, q, hub, cfg)
		go bookingWorker.StartBookingWorker(ctx)
		log.Println("Booking worker started")

		purchaseWorker := services.NewPurchaseWorker(bookingService, q, hub)
		go purchaseWorker.StartPurchaseWorker(ctx)
		log.Println("Purchase worker started")

		queueMonitor := services.NewQueueMonitor(q, cfg, "booking-workers", "purchase-workers")
		go queueMonitor.Run(ctx)
		log.Println("Queue monitor started")
	} else {
		log.Println("Kafka command queue not configured; async booking queue disabled")
	}

	router := routes.SetupRoutes(cfg, redisClient, hub)

	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSec) * time.Second,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-sigChan
	log.Println("Shutting down server...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSec)*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
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
