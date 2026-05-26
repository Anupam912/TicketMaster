/**
 * EventDetailPage - View event details and select seats
 */

import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Calendar, MapPin, Users, Clock, CreditCard } from 'lucide-react';
import { getEvent, getEventSeats } from '@/services/events';
import { reserveSeat, waitForJobCompletion } from '@/services/bookings';
import { useAuth } from '@/context/AuthContext';
import { Button, Card } from '@/components/ui';
import type { Seat, SeatStatusType } from '@/types';
import styles from './EventDetailPage.module.css';

export function EventDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { isAuthenticated } = useAuth();
  
  const [selectedSeat, setSelectedSeat] = useState<Seat | null>(null);
  const [reservationError, setReservationError] = useState('');

  const { data: event, isLoading: eventLoading } = useQuery({
    queryKey: ['event', id],
    queryFn: () => getEvent(id!),
    enabled: !!id,
  });

  const { data: seatsData, isLoading: seatsLoading } = useQuery({
    queryKey: ['seats', id],
    queryFn: () => getEventSeats(id!),
    enabled: !!id,
    refetchInterval: 10000, // Refresh seats every 10 seconds
  });

  const reserveMutation = useMutation({
    mutationFn: async () => {
      if (!selectedSeat || !id) throw new Error('No seat selected');
      
      const response = await reserveSeat({
        event_id: id,
        seat_number: selectedSeat.seat_number,
      });
      
      const jobResult = await waitForJobCompletion(response.job_id);
      
      if (jobResult.status === 'failed') {
        throw new Error(jobResult.error || 'Reservation failed');
      }
      
      return jobResult;
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['seats', id] });
      if (result.booking_id) {
        navigate(`/my-bookings`);
      }
    },
    onError: (error) => {
      setReservationError(error instanceof Error ? error.message : 'Reservation failed');
    },
  });

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('en-US', {
      weekday: 'long',
      month: 'long',
      day: 'numeric',
      year: 'numeric',
    });
  };

  const formatTime = (dateString: string): string => {
    return new Date(dateString).toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const formatPrice = (price: number): string => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(price);
  };

  const getSeatColor = (status: SeatStatusType): string => {
    switch (status) {
      case 'available':
        return styles.seatAvailable;
      case 'reserved':
        return styles.seatReserved;
      case 'sold':
        return styles.seatSold;
      default:
        return '';
    }
  };

  const handleSeatClick = (seat: Seat) => {
    if (seat.status !== 'available') return;
    setSelectedSeat(seat.id === selectedSeat?.id ? null : seat);
    setReservationError('');
  };

  const handleReserve = () => {
    if (!isAuthenticated) {
      navigate('/login', { state: { from: `/events/${id}` } });
      return;
    }
    reserveMutation.mutate();
  };

  if (eventLoading || seatsLoading) {
    return (
      <div className={styles.container}>
        <div className={styles.loading}>
          <div className={styles.spinner} />
          <p>Loading event details...</p>
        </div>
      </div>
    );
  }

  if (!event) {
    return (
      <div className={styles.container}>
        <div className={styles.error}>
          <h2>Event not found</h2>
          <p>The event you're looking for doesn't exist.</p>
          <Button onClick={() => navigate('/events')}>
            Browse Events
          </Button>
        </div>
      </div>
    );
  }

  const seats = seatsData?.seats || [];
  const seatsBySection = seats.reduce((acc: Record<string, Seat[]>, seat: Seat) => {
    const section = seat.section || 'General';
    if (!acc[section]) acc[section] = [];
    acc[section].push(seat);
    return acc;
  }, {});

  return (
    <div className={styles.container}>
      <div className={styles.content}>
        <div className={styles.mainContent}>
          <div className={styles.eventHeader}>
            <h1 className={styles.eventTitle}>{event.title}</h1>
            <p className={styles.eventDescription}>{event.description}</p>
            
            <div className={styles.eventMeta}>
              <div className={styles.metaItem}>
                <Calendar className={styles.metaIcon} />
                <span>{formatDate(event.event_date)}</span>
              </div>
              <div className={styles.metaItem}>
                <Clock className={styles.metaIcon} />
                <span>{formatTime(event.event_date)}</span>
              </div>
              {event.venue && (
                <div className={styles.metaItem}>
                  <MapPin className={styles.metaIcon} />
                  <span>{event.venue.name}, {event.venue.city}</span>
                </div>
              )}
              <div className={styles.metaItem}>
                <Users className={styles.metaIcon} />
                <span>{event.available_seats} of {event.total_seats} seats available</span>
              </div>
            </div>
          </div>

          <div className={styles.seatSelection}>
            <h2 className={styles.sectionTitle}>Select Your Seat</h2>
            
            <div className={styles.legend}>
              <div className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.seatAvailable}`} />
                <span>Available</span>
              </div>
              <div className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.seatReserved}`} />
                <span>Reserved</span>
              </div>
              <div className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.seatSold}`} />
                <span>Sold</span>
              </div>
              <div className={styles.legendItem}>
                <span className={`${styles.legendDot} ${styles.seatSelected}`} />
                <span>Selected</span>
              </div>
            </div>

            <div className={styles.stage}>STAGE</div>

            {Object.entries(seatsBySection).map(([section, sectionSeats]) => (
              <div key={section} className={styles.seatSection}>
                <h3 className={styles.sectionName}>{section}</h3>
                <div className={styles.seatGrid}>
                  {sectionSeats.map((seat: Seat) => (
                    <button
                      key={seat.id}
                      className={`${styles.seat} ${getSeatColor(seat.status)} ${
                        selectedSeat?.id === seat.id ? styles.seatSelected : ''
                      }`}
                      onClick={() => handleSeatClick(seat)}
                      disabled={seat.status !== 'available'}
                      title={`Seat ${seat.seat_number}${seat.row_number ? `, Row ${seat.row_number}` : ''}`}
                    >
                      {seat.seat_number}
                    </button>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className={styles.sidebar}>
          <Card className={styles.bookingCard}>
            <Card.Header>
              <Card.Title>Booking Summary</Card.Title>
            </Card.Header>
            
            <Card.Content>
              <div className={styles.summaryItem}>
                <span>Event</span>
                <span className={styles.summaryValue}>{event.title}</span>
              </div>
              
              <div className={styles.summaryItem}>
                <span>Date</span>
                <span className={styles.summaryValue}>{formatDate(event.event_date)}</span>
              </div>
              
              <div className={styles.summaryItem}>
                <span>Time</span>
                <span className={styles.summaryValue}>{formatTime(event.event_date)}</span>
              </div>
              
              {selectedSeat && (
                <>
                  <div className={styles.divider} />
                  <div className={styles.summaryItem}>
                    <span>Seat</span>
                    <span className={styles.summaryValue}>
                      {selectedSeat.seat_number}
                      {selectedSeat.row_number && `, Row ${selectedSeat.row_number}`}
                    </span>
                  </div>
                  <div className={styles.summaryItem}>
                    <span>Section</span>
                    <span className={styles.summaryValue}>
                      {selectedSeat.section || 'General'}
                    </span>
                  </div>
                </>
              )}
              
              <div className={styles.divider} />
              
              <div className={styles.totalRow}>
                <span>Total</span>
                <span className={styles.totalPrice}>
                  {selectedSeat ? formatPrice(event.ticket_price) : '$0.00'}
                </span>
              </div>

              {reservationError && (
                <div className={styles.reservationError}>
                  {reservationError}
                </div>
              )}
            </Card.Content>
            
            <Card.Footer>
              <Button
                variant="primary"
                className={styles.reserveButton}
                onClick={handleReserve}
                disabled={!selectedSeat || reserveMutation.isPending}
                isLoading={reserveMutation.isPending}
                leftIcon={<CreditCard />}
              >
                {!isAuthenticated 
                  ? 'Login to Reserve' 
                  : selectedSeat 
                    ? 'Reserve Seat' 
                    : 'Select a Seat'}
              </Button>
            </Card.Footer>
          </Card>
        </div>
      </div>
    </div>
  );
}
