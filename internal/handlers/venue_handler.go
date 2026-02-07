package handlers

import (
	"net/http"

	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/services"

	"github.com/gin-gonic/gin"
)

type VenueHandler struct {
	venueService *services.VenueService
}

func NewVenueHandler(venueService *services.VenueService) *VenueHandler {
	return &VenueHandler{
		venueService: venueService,
	}
}

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

func (h *VenueHandler) GetVenue(c *gin.Context) {
	id := c.Param("id")
	venue, err := h.venueService.GetVenueByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "venue not found"})
		return
	}

	c.JSON(http.StatusOK, venue)
}

func (h *VenueHandler) ListVenues(c *gin.Context) {
	venues, err := h.venueService.ListVenues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, venues)
}
