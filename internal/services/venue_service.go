package services

import (
	"errors"

	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/repository"

	"github.com/google/uuid"
)

type VenueService struct {
	venueRepo *repository.VenueRepository
}

func NewVenueService(venueRepo *repository.VenueRepository) *VenueService {
	return &VenueService{
		venueRepo: venueRepo,
	}
}

func (s *VenueService) CreateVenue(req *models.CreateVenueRequest) (*models.Venue, error) {
	venue := &models.Venue{
		Name:      req.Name,
		Address:   req.Address,
		Capacity:  req.Capacity,
		SeatLayout: req.SeatLayout,
	}

	if err := s.venueRepo.Create(venue); err != nil {
		return nil, err
	}

	return venue, nil
}

func (s *VenueService) GetVenueByID(idStr string) (*models.Venue, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errors.New("invalid venue ID")
	}

	venue, err := s.venueRepo.FindByID(id)
	if err != nil {
		if err == repository.ErrVenueNotFound {
			return nil, errors.New("venue not found")
		}
		return nil, err
	}

	return venue, nil
}

func (s *VenueService) ListVenues() ([]*models.Venue, error) {
	return s.venueRepo.ListAll()
}
