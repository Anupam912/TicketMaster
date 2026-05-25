package handlers

import (
	"net/http"
	"strings"

	"event-ticketing-system/internal/middleware"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/queue"
	"event-ticketing-system/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BookingHandler struct {
	bookingService *services.BookingService
	queue          *queue.Queue
}

func NewBookingHandler(bookingService *services.BookingService, q *queue.Queue) *BookingHandler {
	return &BookingHandler{
		bookingService: bookingService,
		queue:          q,
	}
}

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
		statusCode := http.StatusInternalServerError
		if err.Error() == "seat is currently being booked by another user" ||
			err.Error() == "seat is not available" ||
			err.Error() == "no available seats" {
			statusCode = http.StatusConflict
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, booking)
}

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
			UserID:        userID,
			BookingID:     req.BookingID,
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
		statusCode := http.StatusInternalServerError
		if err.Error() == "booking has expired" ||
			err.Error() == "booking is not in reserved status" {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, booking)
}

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
		statusCode := http.StatusInternalServerError
		if err.Error() == "no seats requested" ||
			err.Error() == "maximum 500 seats per bulk request" ||
			err.Error() == "event not found" ||
			err.Error() == "no available seats" {
			statusCode = http.StatusBadRequest
		}
		if strings.HasPrefix(err.Error(), "bulk reserve failed:") {
			statusCode = http.StatusConflict
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "bulk reservation completed",
		"count":    len(bookings),
		"bookings": bookings,
	})
}

func (h *BookingHandler) GetMyBookings(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	bookings, err := h.bookingService.GetUserBookings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) CancelBooking(c *gin.Context) {
	idStr := c.Param("id")
	bookingID, err := uuid.Parse(idStr)
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
		statusCode := http.StatusInternalServerError
		if err.Error() == "booking not found" {
			statusCode = http.StatusNotFound
		} else if err.Error() == "only reserved bookings can be cancelled" {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "booking cancelled successfully"})
}
