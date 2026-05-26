/**
 * EventsPage - Browse all available events
 */

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Calendar, MapPin, Users, Search } from 'lucide-react';
import { getEvents } from '@/services/events';
import { Input, Card } from '@/components/ui';
import type { Event } from '@/types';
import styles from './EventsPage.module.css';

export function EventsPage() {
  const [searchTerm, setSearchTerm] = useState('');
  
  const { data: events, isLoading, error } = useQuery({
    queryKey: ['events'],
    queryFn: getEvents,
  });

  const filteredEvents = events?.filter((event: Event) =>
    event.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
    event.description.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('en-US', {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
      year: 'numeric',
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

  if (isLoading) {
    return (
      <div className={styles.container}>
        <div className={styles.loading}>
          <div className={styles.spinner} />
          <p>Loading events...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.container}>
        <div className={styles.error}>
          <h2>Error loading events</h2>
          <p>{error instanceof Error ? error.message : 'Something went wrong'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Upcoming Events</h1>
        <p className={styles.subtitle}>
          Discover and book tickets for amazing events
        </p>
      </div>

      <div className={styles.searchContainer}>
        <div className={styles.searchWrapper}>
          <Search className={styles.searchIcon} />
          <Input
            type="text"
            placeholder="Search events..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className={styles.searchInput}
          />
        </div>
      </div>

      {filteredEvents && filteredEvents.length > 0 ? (
        <div className={styles.grid}>
          {filteredEvents.map((event: Event) => (
            <Link 
              key={event.id} 
              to={`/events/${event.id}`}
              className={styles.cardLink}
            >
              <Card className={styles.eventCard}>
                <div className={styles.eventImage}>
                  <span className={styles.eventEmoji}>🎫</span>
                </div>
                
                <Card.Content className={styles.eventContent}>
                  <h3 className={styles.eventTitle}>{event.title}</h3>
                  
                  <p className={styles.eventDescription}>
                    {event.description.length > 100
                      ? `${event.description.substring(0, 100)}...`
                      : event.description}
                  </p>
                  
                  <div className={styles.eventMeta}>
                    <div className={styles.metaItem}>
                      <Calendar className={styles.metaIcon} />
                      <span>{formatDate(event.event_date)}</span>
                    </div>
                    
                    {event.venue && (
                      <div className={styles.metaItem}>
                        <MapPin className={styles.metaIcon} />
                        <span>{event.venue.name}, {event.venue.city}</span>
                      </div>
                    )}
                    
                    <div className={styles.metaItem}>
                      <Users className={styles.metaIcon} />
                      <span>{event.available_seats} seats available</span>
                    </div>
                  </div>
                  
                  <div className={styles.eventFooter}>
                    <span className={styles.price}>
                      {formatPrice(event.ticket_price)}
                    </span>
                    <span className={styles.perTicket}>per ticket</span>
                  </div>
                </Card.Content>
              </Card>
            </Link>
          ))}
        </div>
      ) : (
        <div className={styles.empty}>
          <h3>No events found</h3>
          <p>
            {searchTerm
              ? 'Try adjusting your search terms'
              : 'Check back later for upcoming events'}
          </p>
        </div>
      )}
    </div>
  );
}
