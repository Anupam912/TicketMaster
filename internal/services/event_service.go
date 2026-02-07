package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"event-ticketing-system/internal/config"
	"event-ticketing-system/internal/models"
	"event-ticketing-system/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type EventService struct {
	eventRepo *repository.EventRepository
	venueRepo *repository.VenueRepository
	seatRepo  *repository.SeatRepository
	redis     *redis.Client
	config    *config.Config
}

func NewEventService(
	eventRepo *repository.EventRepository,
	venueRepo *repository.VenueRepository,
	seatRepo *repository.SeatRepository,
	redis *redis.Client,
	cfg *config.Config,
) *EventService {
	return &EventService{
		eventRepo: eventRepo,
		venueRepo: venueRepo,
		seatRepo:  seatRepo,
		redis:     redis,
		config:    cfg,
	}
}

func (s *EventService) CreateEvent(req *models.CreateEventRequest, createdBy uuid.UUID) (*models.Event, error) {
	_, err := s.venueRepo.FindByID(req.VenueID)
	if err != nil {
		if err == repository.ErrVenueNotFound {
			return nil, errors.New("venue not found")
		}
		return nil, err
	}

	event := &models.Event{
		VenueID:     req.VenueID,
		Title:       req.Title,
		Description: req.Description,
		EventDate:   req.EventDate,
		TicketPrice: req.TicketPrice,
		TotalSeats:  req.TotalSeats,
		CreatedBy:   createdBy,
	}

	if err := s.eventRepo.Create(event); err != nil {
		return nil, err
	}

	if err := s.seatRepo.CreateBulkSeats(event.ID, req.TotalSeats); err != nil {
		return nil, fmt.Errorf("failed to create seats: %w", err)
	}

	s.invalidateEventCache(event.ID)

	return event, nil
}

func (s *EventService) GetEventByID(id uuid.UUID) (*models.Event, error) {
	cached, err := s.getEventFromCache(id)
	if err == nil && cached != nil {
		return cached, nil
	}

	event, err := s.eventRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	s.cacheEvent(event)

	return event, nil
}

func (s *EventService) ListEvents() ([]*models.Event, error) {
	return s.eventRepo.ListAll()
}

func (s *EventService) getEventFromCache(id uuid.UUID) (*models.Event, error) {
	if s.redis == nil {
		return nil, errors.New("redis not available")
	}

	ctx := context.Background()
	key := fmt.Sprintf("event:%s", id.String())
	data, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var event models.Event
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, err
	}

	return &event, nil
}

func (s *EventService) cacheEvent(event *models.Event) {
	if s.redis == nil {
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("event:%s", event.ID.String())
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.redis.Set(ctx, key, data, time.Hour)
}

func (s *EventService) invalidateEventCache(id uuid.UUID) {
	if s.redis == nil {
		return
	}

	ctx := context.Background()
	key := fmt.Sprintf("event:%s", id.String())
	s.redis.Del(ctx, key)
}
