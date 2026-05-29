package routes

import (
	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/handlers"
	"event-ticketing-system/internal/kafka"
	"event-ticketing-system/internal/middleware"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/services"
	"event-ticketing-system/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupRoutes configures all API routes and returns the Gin engine.
func SetupRoutes(
	cfg *config.Config,
	redisClient *redis.Client,
	hub *websocket.Hub,
) *gin.Engine {
	gin.SetMode(cfg.Server.GinMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.AccessLog())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})
	router.GET("/metrics", handlers.ServePrometheusMetrics)

	swaggerHandler := handlers.NewSwaggerHandler()
	router.GET("/docs", swaggerHandler.ServeSwaggerUI)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	userRepo := repository.NewUserRepository()
	venueRepo := repository.NewVenueRepository()
	eventRepo := repository.NewEventRepository()
	seatRepo := repository.NewSeatRepository()
	bookingRepo := repository.NewBookingRepository()

	q := queue.NewQueue(cfg, redisClient)
	q.SetMaxRetries(cfg.Queue.MaxRetries)
	var bookingQueue *queue.Queue
	if q.Enabled() {
		bookingQueue = q
	}
	expiryQueue := queue.NewExpiryQueue(redisClient)

	authService := services.NewAuthService(userRepo, cfg)
	venueService := services.NewVenueService(venueRepo)
	eventService := services.NewEventService(eventRepo, venueRepo, seatRepo, redisClient, cfg)
	paymentService := services.NewPaymentService(cfg)
	bookingService := services.NewBookingService(bookingRepo, eventRepo, seatRepo, cfg, expiryQueue, paymentService)

	bookingService.SetCacheInvalidator(eventService.InvalidateEventCache)
	kafkaProducer, err := kafka.NewProducer(cfg)
	if err == nil && kafkaProducer.Enabled() {
		bookingService.SetEventPublisher(services.NewKafkaEventPublisher(kafkaProducer))
	}

	authHandler := handlers.NewAuthHandler(authService)
	venueHandler := handlers.NewVenueHandler(venueService)
	eventHandler := handlers.NewEventHandler(eventService)
	bookingHandler := handlers.NewBookingHandler(bookingService, bookingQueue)
	wsHandler := handlers.NewWebSocketHandler(hub)

	authMiddleware := middleware.NewAuthMiddleware(cfg)
	rateLimiter := middleware.NewRateLimiter(redisClient, cfg)
	idempotencyMiddleware := middleware.NewIdempotencyMiddleware(redisClient, cfg)

	api := router.Group("/api")

	auth := api.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)

	api.GET("/events", rateLimiter.SimpleRateLimit(), eventHandler.ListEvents)
	api.GET("/events/:id", rateLimiter.SimpleRateLimit(), eventHandler.GetEvent)
	api.GET("/events/:id/seats", rateLimiter.SimpleRateLimit(), eventHandler.GetEventSeats)

	protected := api.Group("")
	protected.Use(authMiddleware.RequireAuth())
	protected.Use(rateLimiter.SimpleRateLimit())

	protected.GET("/auth/me", authHandler.GetMe)

	admin := protected.Group("/admin")
	admin.Use(authMiddleware.RequireAdmin())
	admin.GET("/users", authHandler.ListUsers)
	admin.POST("/users/:id/promote", authHandler.PromoteToAdmin)
	admin.POST("/users/:id/demote", authHandler.DemoteToUser)
	admin.GET("/queue-metrics", bookingHandler.GetQueueMetrics)

	venues := protected.Group("/venues")
	venues.Use(authMiddleware.RequireAdmin())
	venues.POST("", venueHandler.CreateVenue)
	venues.GET("", venueHandler.ListVenues)
	venues.GET("/:id", venueHandler.GetVenue)

	events := protected.Group("/events")
	events.POST("", authMiddleware.RequireAdmin(), eventHandler.CreateEvent)

	bookings := protected.Group("/bookings")
	// VirtualWaitingRoom: 100 requests per 60 seconds per IP (increased for load testing)
	bookings.POST("/reserve", rateLimiter.VirtualWaitingRoom(100, 60), bookingHandler.ReserveSeat)
	bookings.POST("/bulk-reserve", bookingHandler.BulkReserve)
	bookings.POST("/purchase", idempotencyMiddleware.IdempotencyKey(), bookingHandler.PurchaseBooking)
	bookings.GET("/my-bookings", bookingHandler.GetMyBookings)
	bookings.GET("/job/:job_id", bookingHandler.GetJobStatus)
	bookings.GET("/:id", bookingHandler.GetBooking)
	bookings.DELETE("/:id", bookingHandler.CancelBooking)

	protected.GET("/ws", wsHandler.HandleWebSocket)

	return router
}
