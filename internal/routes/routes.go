package routes

import (
	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/handlers"
	"event-ticketing-system/internal/middleware"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/services"
	"event-ticketing-system/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// SetupRoutes configures all API routes and returns the Gin engine.
func SetupRoutes(
	cfg *config.Config,
	redisClient *redis.Client,
	hub *websocket.Hub,
) *gin.Engine {
	gin.SetMode(cfg.Server.GinMode)
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	userRepo := repository.NewUserRepository()
	venueRepo := repository.NewVenueRepository()
	eventRepo := repository.NewEventRepository()
	seatRepo := repository.NewSeatRepository()
	bookingRepo := repository.NewBookingRepository()

	q := queue.NewQueue(redisClient)
	expiryQueue := queue.NewExpiryQueue(redisClient)

	authService := services.NewAuthService(userRepo, cfg)
	venueService := services.NewVenueService(venueRepo)
	eventService := services.NewEventService(eventRepo, venueRepo, seatRepo, redisClient, cfg)
	bookingService := services.NewBookingService(bookingRepo, eventRepo, seatRepo, cfg, expiryQueue)

	authHandler := handlers.NewAuthHandler(authService)
	venueHandler := handlers.NewVenueHandler(venueService)
	eventHandler := handlers.NewEventHandler(eventService)
	bookingHandler := handlers.NewBookingHandler(bookingService, q)
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

	protected := api.Group("")
	protected.Use(authMiddleware.RequireAuth())
	protected.Use(rateLimiter.SimpleRateLimit())

	protected.GET("/auth/me", authHandler.GetMe)

	venues := protected.Group("/venues")
	venues.Use(authMiddleware.RequireAdmin())
	venues.POST("", venueHandler.CreateVenue)
	venues.GET("", venueHandler.ListVenues)
	venues.GET("/:id", venueHandler.GetVenue)

	events := protected.Group("/events")
	events.POST("", authMiddleware.RequireAdmin(), eventHandler.CreateEvent)

	bookings := protected.Group("/bookings")
	bookings.POST("/reserve", rateLimiter.VirtualWaitingRoom(5, 60), bookingHandler.ReserveSeat)
	bookings.POST("/bulk-reserve", bookingHandler.BulkReserve)
	bookings.POST("/purchase", idempotencyMiddleware.IdempotencyKey(), bookingHandler.PurchaseBooking)
	bookings.GET("/my-bookings", bookingHandler.GetMyBookings)
	bookings.DELETE("/:id", bookingHandler.CancelBooking)

	protected.GET("/ws", wsHandler.HandleWebSocket)

	return router
}
