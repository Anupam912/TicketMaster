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

// Cache configuration constants.
const (
	eventCacheKeyPrefix = "event:"
	eventCacheTTL       = time.Hour
)

// Sentinel errors for event service operations.
var (
	ErrEventNotFound = errors.New("event not found")
	ErrVenueNotFound = errors.New("venue not found")
)

// EventService handles event-related business logic.
type EventService struct {
	eventRepo *repository.EventRepository
	venueRepo *repository.VenueRepository
	seatRepo  *repository.SeatRepository
	redis     *redis.Client
	config    *config.Config
}

// NewEventService creates a new EventService instance.
func NewEventService(
	eventRepo *repository.EventRepository,
	venueRepo *repository.VenueRepository,
	seatRepo *repository.SeatRepository,
	redisClient *redis.Client,
	cfg *config.Config,
) *EventService {
	return &EventService{
		eventRepo: eventRepo,
		venueRepo: venueRepo,
		seatRepo:  seatRepo,
		redis:     redisClient,
		config:    cfg,
	}
}

// CreateEvent creates a new event with the specified details and generates seats.
func (s *EventService) CreateEvent(ctx context.Context, req *models.CreateEventRequest, createdBy uuid.UUID) (*models.Event, error) {
	_, err := s.venueRepo.FindByID(req.VenueID)
	if err != nil {
		if errors.Is(err, repository.ErrVenueNotFound) {
			return nil, ErrVenueNotFound
		}
		return nil, fmt.Errorf("find venue: %w", err)
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
		return nil, fmt.Errorf("create event: %w", err)
	}

	if err := s.seatRepo.CreateBulkSeats(event.ID, req.TotalSeats); err != nil {
		return nil, fmt.Errorf("create seats: %w", err)
	}

	s.invalidateEventCache(ctx, event.ID)

	return event, nil
}

// GetEventByID retrieves an event by its UUID, using cache when available.
func (s *EventService) GetEventByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	if cached, err := s.getEventFromCache(ctx, id); err == nil && cached != nil {
		return cached, nil
	}

	event, err := s.eventRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrEventNotFound) {
			return nil, ErrEventNotFound
		}
		return nil, fmt.Errorf("find event: %w", err)
	}

	s.cacheEvent(ctx, event)

	return event, nil
}

// ListEvents retrieves all events.
func (s *EventService) ListEvents(ctx context.Context) ([]*models.Event, error) {
	events, err := s.eventRepo.ListAll()
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return events, nil
}

// SeatsSummary contains aggregated counts of seats by status.
type SeatsSummary struct {
	Total     int `json:"total"`
	Available int `json:"available"`
	Reserved  int `json:"reserved"`
	Sold      int `json:"sold"`
}

// SeatsResponse contains the seats list and summary for an event.
type SeatsResponse struct {
	EventID uuid.UUID      `json:"event_id"`
	Summary SeatsSummary   `json:"summary"`
	Seats   []*models.Seat `json:"seats"`
}

// GetEventSeats returns all seats for an event with optional status filter.
// The summary always reflects totals across all seats regardless of filter.
func (s *EventService) GetEventSeats(ctx context.Context, eventID uuid.UUID, statusFilter string) (*SeatsResponse, error) {
	_, err := s.eventRepo.FindByID(eventID)
	if err != nil {
		if errors.Is(err, repository.ErrEventNotFound) {
			return nil, ErrEventNotFound
		}
		return nil, fmt.Errorf("find event: %w", err)
	}

	allSeats, err := s.seatRepo.FindByEventID(ctx, eventID, "")
	if err != nil {
		return nil, fmt.Errorf("find all seats: %w", err)
	}

	summary := s.calculateSeatsSummary(allSeats)

	var seats []*models.Seat
	if statusFilter != "" {
		seats, err = s.seatRepo.FindByEventID(ctx, eventID, statusFilter)
		if err != nil {
			return nil, fmt.Errorf("find filtered seats: %w", err)
		}
	} else {
		seats = allSeats
	}

	return &SeatsResponse{
		EventID: eventID,
		Summary: summary,
		Seats:   seats,
	}, nil
}

// calculateSeatsSummary aggregates seat counts by status.
func (s *EventService) calculateSeatsSummary(seats []*models.Seat) SeatsSummary {
	summary := SeatsSummary{Total: len(seats)}
	for _, seat := range seats {
		switch seat.Status {
		case models.SeatStatusAvailable:
			summary.Available++
		case models.SeatStatusReserved:
			summary.Reserved++
		case models.SeatStatusSold:
			summary.Sold++
		}
	}
	return summary
}

// InvalidateEventCache removes the cached event data for the given event ID.
// This is exposed for use by other services (e.g., BookingService) when seat
// availability changes.
func (s *EventService) InvalidateEventCache(eventID uuid.UUID) {
	s.invalidateEventCache(context.Background(), eventID)
}

func (s *EventService) getEventFromCache(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	if s.redis == nil {
		return nil, errors.New("redis not available")
	}

	key := eventCacheKeyPrefix + id.String()
	data, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var event models.Event
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, fmt.Errorf("unmarshal cached event: %w", err)
	}

	return &event, nil
}

func (s *EventService) cacheEvent(ctx context.Context, event *models.Event) {
	if s.redis == nil {
		return
	}

	key := eventCacheKeyPrefix + event.ID.String()
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	_ = s.redis.Set(ctx, key, data, eventCacheTTL).Err()
}

func (s *EventService) invalidateEventCache(ctx context.Context, id uuid.UUID) {
	if s.redis == nil {
		return
	}

	key := eventCacheKeyPrefix + id.String()
	_ = s.redis.Del(ctx, key).Err()
}
