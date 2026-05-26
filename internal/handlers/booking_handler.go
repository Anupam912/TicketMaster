package handlers

import (
	"errors"
	"net/http"

	"event-ticketing-system/internal/middleware"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/repository"
	"event-ticketing-system/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BookingHandler handles booking-related HTTP requests.
type BookingHandler struct {
	bookingService *services.BookingService
	queue          *queue.Queue
}

// NewBookingHandler creates a new BookingHandler instance.
func NewBookingHandler(bookingService *services.BookingService, q *queue.Queue) *BookingHandler {
	return &BookingHandler{
		bookingService: bookingService,
		queue:          q,
	}
}

// ReserveSeat godoc
// @Summary      Reserve a seat
// @Description  Reserves a seat for the authenticated user. Returns immediately with job ID if async processing is enabled.
// @Tags         bookings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.ReserveSeatRequest true "Seat reservation details"
// @Success      201 {object} models.Booking "Seat reserved successfully"
// @Success      202 {object} map[string]interface{} "Request accepted for async processing"
// @Failure      400 {object} map[string]string "Invalid request"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      404 {object} map[string]string "Event not found"
// @Failure      409 {object} map[string]string "Seat not available or already booked"
// @Failure      429 {object} map[string]string "Too many requests (rate limited)"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /bookings/reserve [post]
func (h *BookingHandler) ReserveSeat(c *gin.Context) {
	var req models.ReserveSeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.queue != nil {
		job := &queue.BookingJob{
			UserID:     userID,
			EventID:    req.EventID,
			SeatNumber: req.SeatNumber,
		}

		if err := h.queue.EnqueueBookingJob(c.Request.Context(), job); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue booking request"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"message": "booking request received, processing...",
			"job_id":  job.ID,
		})
		return
	}

	booking, err := h.bookingService.ReserveSeat(userID, &req)
	if err != nil {
		h.handleReservationError(c, err)
		return
	}

	c.JSON(http.StatusCreated, booking)
}

func (h *BookingHandler) handleReservationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrSeatBeingBooked),
		errors.Is(err, services.ErrSeatNotAvailable),
		errors.Is(err, services.ErrNoAvailableSeats):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrEventNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reservation failed"})
	}
}

// PurchaseBooking godoc
// @Summary      Purchase a booking
// @Description  Completes the purchase of a reserved booking. Processes payment via Stripe.
// @Tags         bookings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key header string false "Idempotency key to prevent duplicate charges"
// @Param        request body models.PurchaseBookingRequest true "Purchase details"
// @Success      200 {object} models.Booking "Purchase completed successfully"
// @Success      202 {object} map[string]interface{} "Request accepted for async processing"
// @Failure      400 {object} map[string]string "Booking expired or invalid status"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      403 {object} map[string]string "Not authorized to purchase this booking"
// @Failure      404 {object} map[string]string "Booking not found"
// @Failure      500 {object} map[string]string "Internal server error or payment failed"
// @Router       /bookings/purchase [post]
func (h *BookingHandler) PurchaseBooking(c *gin.Context) {
	var req models.PurchaseBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" && req.IdempotencyKey != "" {
		idempotencyKey = req.IdempotencyKey
	}

	if h.queue != nil {
		job := &queue.PurchaseJob{
			UserID:         userID,
			BookingID:      req.BookingID,
			IdempotencyKey: idempotencyKey,
		}

		if err := h.queue.EnqueuePurchaseJob(c.Request.Context(), job); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue purchase request"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"message": "purchase request received, processing...",
			"job_id":  job.ID,
		})
		return
	}

	booking, err := h.bookingService.PurchaseBooking(userID, req.BookingID)
	if err != nil {
		h.handlePurchaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func (h *BookingHandler) handlePurchaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrBookingExpired),
		errors.Is(err, services.ErrNotReserved):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrBookingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
	case errors.Is(err, services.ErrUnauthorized):
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})
	}
}

// BulkReserve godoc
// @Summary      Reserve multiple seats
// @Description  Reserves multiple seats for the authenticated user in a single operation
// @Tags         bookings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.BulkReserveRequest true "Bulk reservation details (max 500 seats)"
// @Success      201 {object} map[string]interface{} "Bulk reservation completed"
// @Failure      400 {object} map[string]string "Invalid request or too many seats"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      409 {object} map[string]string "Some seats not available"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /bookings/bulk-reserve [post]
func (h *BookingHandler) BulkReserve(c *gin.Context) {
	var req models.BulkReserveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	bookings, err := h.bookingService.BulkReserve(userID, &req)
	if err != nil {
		h.handleBulkReserveError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "bulk reservation completed",
		"count":    len(bookings),
		"bookings": bookings,
	})
}

func (h *BookingHandler) handleBulkReserveError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNoSeatsRequested),
		errors.Is(err, services.ErrTooManySeats),
		errors.Is(err, repository.ErrEventNotFound),
		errors.Is(err, services.ErrNoAvailableSeats):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	}
}

// GetMyBookings godoc
// @Summary      Get user's bookings
// @Description  Returns all bookings for the authenticated user
// @Tags         bookings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.BookingWithDetails
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /bookings/my-bookings [get]
func (h *BookingHandler) GetMyBookings(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	bookings, err := h.bookingService.GetUserBookings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get bookings"})
		return
	}

	c.JSON(http.StatusOK, bookings)
}

// CancelBooking godoc
// @Summary      Cancel a booking
// @Description  Cancels a reserved booking and releases the seat
// @Tags         bookings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Booking ID" format(uuid)
// @Success      200 {object} map[string]string "Booking cancelled successfully"
// @Failure      400 {object} map[string]string "Invalid booking ID or cannot cancel"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      403 {object} map[string]string "Not authorized to cancel this booking"
// @Failure      404 {object} map[string]string "Booking not found"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /bookings/{id} [delete]
func (h *BookingHandler) CancelBooking(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking ID"})
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.bookingService.CancelBooking(userID, bookingID); err != nil {
		switch {
		case errors.Is(err, services.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
		case errors.Is(err, services.ErrOnlyReservedCancel):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cancellation failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "booking cancelled successfully"})
}

// GetBooking godoc
// @Summary      Get booking by ID
// @Description  Returns a specific booking by ID for the authenticated user
// @Tags         bookings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Booking ID" format(uuid)
// @Success      200 {object} models.Booking
// @Failure      400 {object} map[string]string "Invalid booking ID"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      404 {object} map[string]string "Booking not found"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /bookings/{id} [get]
func (h *BookingHandler) GetBooking(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking ID"})
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	booking, err := h.bookingService.GetBookingByID(userID, bookingID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrBookingNotFound),
			errors.Is(err, services.ErrUnauthorized):
			c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get booking"})
		}
		return
	}

	c.JSON(http.StatusOK, booking)
}

// GetJobStatus godoc
// @Summary      Get async job status
// @Description  Returns the status of an async booking/purchase job
// @Tags         bookings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        job_id path string true "Job ID" format(uuid)
// @Success      200 {object} queue.JobStatus
// @Failure      400 {object} map[string]string "Invalid job ID"
// @Failure      404 {object} map[string]string "Job not found"
// @Failure      500 {object} map[string]string "Internal server error"
// @Failure      503 {object} map[string]string "Async processing not available"
// @Router       /bookings/job/{job_id} [get]
func (h *BookingHandler) GetJobStatus(c *gin.Context) {
	jobID, err := uuid.Parse(c.Param("job_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job ID"})
		return
	}

	if h.queue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "async processing not available"})
		return
	}

	status, err := h.queue.GetJobStatus(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, queue.ErrJobNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get job status"})
		return
	}

	c.JSON(http.StatusOK, status)
}
