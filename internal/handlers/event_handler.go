package handlers

import (
	"errors"
	"net/http"

	"event-ticketing-system/internal/middleware"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Valid seat status filter values.
var validSeatStatuses = map[string]bool{
	"available": true,
	"reserved":  true,
	"sold":      true,
}

// EventHandler handles event-related HTTP requests.
type EventHandler struct {
	eventService *services.EventService
}

// NewEventHandler creates a new EventHandler instance.
func NewEventHandler(eventService *services.EventService) *EventHandler {
	return &EventHandler{
		eventService: eventService,
	}
}

// CreateEvent godoc
// @Summary      Create a new event (Admin only)
// @Description  Creates a new event with venue, date, pricing, and seats
// @Tags         events
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateEventRequest true "Event details"
// @Success      201 {object} models.Event
// @Failure      400 {object} map[string]string "Invalid request or venue not found"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      403 {object} map[string]string "Forbidden - Admin only"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /events [post]
func (h *EventHandler) CreateEvent(c *gin.Context) {
	var req models.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	event, err := h.eventService.CreateEvent(c.Request.Context(), &req, userID)
	if err != nil {
		if errors.Is(err, services.ErrVenueNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "venue not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create event"})
		return
	}

	c.JSON(http.StatusCreated, event)
}

// GetEvent godoc
// @Summary      Get event by ID
// @Description  Returns details of a specific event
// @Tags         events
// @Accept       json
// @Produce      json
// @Param        id path string true "Event ID" format(uuid)
// @Success      200 {object} models.Event
// @Failure      400 {object} map[string]string "Invalid event ID"
// @Failure      404 {object} map[string]string "Event not found"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /events/{id} [get]
func (h *EventHandler) GetEvent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})
		return
	}

	event, err := h.eventService.GetEventByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrEventNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get event"})
		return
	}

	c.JSON(http.StatusOK, event)
}

// ListEvents godoc
// @Summary      List all events
// @Description  Returns a list of all available events
// @Tags         events
// @Accept       json
// @Produce      json
// @Success      200 {array} models.Event
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /events [get]
func (h *EventHandler) ListEvents(c *gin.Context) {
	events, err := h.eventService.ListEvents(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list events"})
		return
	}

	c.JSON(http.StatusOK, events)
}

// GetEventSeats godoc
// @Summary      Get seats for an event
// @Description  Returns all seats for an event with optional status filter and summary
// @Tags         events
// @Accept       json
// @Produce      json
// @Param        id path string true "Event ID" format(uuid)
// @Param        status query string false "Filter by status" Enums(available, reserved, sold)
// @Success      200 {object} services.SeatsResponse
// @Failure      400 {object} map[string]string "Invalid event ID or status filter"
// @Failure      404 {object} map[string]string "Event not found"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /events/{id}/seats [get]
func (h *EventHandler) GetEventSeats(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})
		return
	}

	statusFilter := c.Query("status")
	if statusFilter != "" && !validSeatStatuses[statusFilter] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status filter, must be: available, reserved, or sold"})
		return
	}

	response, err := h.eventService.GetEventSeats(c.Request.Context(), id, statusFilter)
	if err != nil {
		if errors.Is(err, services.ErrEventNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get seats"})
		return
	}

	c.JSON(http.StatusOK, response)
}
