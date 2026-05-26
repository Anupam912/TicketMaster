/**
 * MyBookingsPage - View user's bookings
 */

import { Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Calendar, MapPin, Ticket, XCircle, CreditCard } from 'lucide-react';
import { getMyBookings, cancelBooking, purchaseBooking } from '@/services/bookings';
import { useAuth } from '@/context/AuthContext';
import { Button, Card } from '@/components/ui';
import type { Booking, BookingStatusType } from '@/types';
import styles from './MyBookingsPage.module.css';

export function MyBookingsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { isAuthenticated } = useAuth();

  const { data: bookings, isLoading, error } = useQuery({
    queryKey: ['myBookings'],
    queryFn: getMyBookings,
    enabled: isAuthenticated,
  });

  const cancelMutation = useMutation({
    mutationFn: cancelBooking,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['myBookings'] });
    },
  });

  const purchaseMutation = useMutation({
    mutationFn: (bookingId: string) => 
      purchaseBooking({ booking_id: bookingId, payment_method: 'card' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['myBookings'] });
    },
  });

  if (!isAuthenticated) {
    return (
      <div className={styles.container}>
        <div className={styles.notLoggedIn}>
          <Ticket className={styles.notLoggedInIcon} />
          <h2>Please log in</h2>
          <p>You need to be logged in to view your bookings.</p>
          <Button onClick={() => navigate('/login', { state: { from: '/my-bookings' } })}>
            Log In
          </Button>
        </div>
      </div>
    );
  }

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('en-US', {
      weekday: 'short',
      month: 'short',
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

  const getStatusBadge = (status: BookingStatusType) => {
    const statusConfig: Record<string, { class: string; label: string }> = {
      pending: { class: styles.statusPending, label: 'Pending Payment' },
      confirmed: { class: styles.statusConfirmed, label: 'Confirmed' },
      cancelled: { class: styles.statusCancelled, label: 'Cancelled' },
      expired: { class: styles.statusExpired, label: 'Expired' },
    };
    
    const config = statusConfig[status] || { class: '', label: status || 'Unknown' };
    return (
      <span className={`${styles.statusBadge} ${config.class}`}>
        {config.label}
      </span>
    );
  };

  const getRemainingTime = (expiresAt: string): string | null => {
    const now = new Date();
    const expires = new Date(expiresAt);
    const diff = expires.getTime() - now.getTime();
    
    if (diff <= 0) return null;
    
    const minutes = Math.floor(diff / 60000);
    if (minutes < 60) return `${minutes} min remaining`;
    
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m remaining`;
  };

  if (isLoading) {
    return (
      <div className={styles.container}>
        <div className={styles.loading}>
          <div className={styles.spinner} />
          <p>Loading your bookings...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.container}>
        <div className={styles.error}>
          <h2>Error loading bookings</h2>
          <p>{error instanceof Error ? error.message : 'Something went wrong'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>My Bookings</h1>
        <p className={styles.subtitle}>
          Manage your event tickets and reservations
        </p>
      </div>

      {bookings && bookings.length > 0 ? (
        <div className={styles.bookingsList}>
          {bookings.map((booking: Booking) => (
            <Card key={booking.id} className={styles.bookingCard}>
              <div className={styles.bookingContent}>
                <div className={styles.bookingMain}>
                  <div className={styles.bookingHeader}>
                    <h3 className={styles.eventTitle}>
                      {booking.event?.title || 'Event'}
                    </h3>
                    {getStatusBadge(booking.status)}
                  </div>

                  {booking.event && (
                    <div className={styles.bookingMeta}>
                      <div className={styles.metaItem}>
                        <Calendar className={styles.metaIcon} />
                        <span>
                          {formatDate(booking.event.event_date)} at{' '}
                          {formatTime(booking.event.event_date)}
                        </span>
                      </div>
                      {booking.event.venue && (
                        <div className={styles.metaItem}>
                          <MapPin className={styles.metaIcon} />
                          <span>
                            {booking.event.venue.name}, {booking.event.venue.city}
                          </span>
                        </div>
                      )}
                      {booking.seat && (
                        <div className={styles.metaItem}>
                          <Ticket className={styles.metaIcon} />
                          <span>
                            Seat {booking.seat.seat_number}
                            {booking.seat.row_number && `, Row ${booking.seat.row_number}`}
                            {booking.seat.section && ` - ${booking.seat.section}`}
                          </span>
                        </div>
                      )}
                    </div>
                  )}

                  {booking.status === 'pending' && (
                    <div className={styles.expiryWarning}>
                      {getRemainingTime(booking.expires_at) || 'Expiring soon'}
                    </div>
                  )}
                </div>

                <div className={styles.bookingActions}>
                  <div className={styles.priceSection}>
                    <span className={styles.priceLabel}>Total</span>
                    <span className={styles.price}>
                      {formatPrice(booking.total_amount)}
                    </span>
                  </div>

                  <div className={styles.actionButtons}>
                    {booking.status === 'pending' && (
                      <>
                        <Button
                          variant="primary"
                          size="sm"
                          leftIcon={<CreditCard />}
                          onClick={() => purchaseMutation.mutate(booking.id)}
                          isLoading={purchaseMutation.isPending}
                        >
                          Complete Purchase
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          leftIcon={<XCircle />}
                          onClick={() => cancelMutation.mutate(booking.id)}
                          isLoading={cancelMutation.isPending}
                        >
                          Cancel
                        </Button>
                      </>
                    )}
                    
                    {booking.status === 'confirmed' && (
                      <Link to={`/events/${booking.event_id}`}>
                        <Button variant="outline" size="sm">
                          View Event
                        </Button>
                      </Link>
                    )}
                  </div>
                </div>
              </div>
            </Card>
          ))}
        </div>
      ) : (
        <div className={styles.empty}>
          <Ticket className={styles.emptyIcon} />
          <h3>No bookings yet</h3>
          <p>You haven't booked any tickets yet. Browse events to get started!</p>
          <Link to="/events">
            <Button variant="primary">Browse Events</Button>
          </Link>
        </div>
      )}
    </div>
  );
}
