package handlers

import (
	"net/http"

	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/services"

	"github.com/gin-gonic/gin"
)

// VenueHandler handles venue-related HTTP requests.
type VenueHandler struct {
	venueService *services.VenueService
}

// NewVenueHandler creates a new VenueHandler instance.
func NewVenueHandler(venueService *services.VenueService) *VenueHandler {
	return &VenueHandler{
		venueService: venueService,
	}
}

// CreateVenue godoc
// @Summary      Create a new venue (Admin only)
// @Description  Creates a new venue with name, address, and capacity
// @Tags         venues
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateVenueRequest true "Venue details"
// @Success      201 {object} models.Venue
// @Failure      400 {object} map[string]string "Invalid request"
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      403 {object} map[string]string "Forbidden - Admin only"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /venues [post]
func (h *VenueHandler) CreateVenue(c *gin.Context) {
	var req models.CreateVenueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	venue, err := h.venueService.CreateVenue(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, venue)
}

// GetVenue godoc
// @Summary      Get venue by ID
// @Description  Returns details of a specific venue
// @Tags         venues
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Venue ID" format(uuid)
// @Success      200 {object} models.Venue
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      404 {object} map[string]string "Venue not found"
// @Router       /venues/{id} [get]
func (h *VenueHandler) GetVenue(c *gin.Context) {
	id := c.Param("id")
	venue, err := h.venueService.GetVenueByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "venue not found"})
		return
	}

	c.JSON(http.StatusOK, venue)
}

// ListVenues godoc
// @Summary      List all venues
// @Description  Returns a list of all venues
// @Tags         venues
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.Venue
// @Failure      401 {object} map[string]string "Unauthorized"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /venues [get]
func (h *VenueHandler) ListVenues(c *gin.Context) {
	venues, err := h.venueService.ListVenues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, venues)
}
