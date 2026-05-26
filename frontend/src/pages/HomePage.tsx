/**
 * Home Page - Landing page with hero section and featured events
 */

import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Calendar, MapPin, Shield, Zap } from 'lucide-react';
import { getEvents } from '@/services/events';
import type { Event } from '@/types';
import styles from './HomePage.module.css';

export function HomePage() {
  const { data: events, isLoading, error } = useQuery({
    queryKey: ['events'],
    queryFn: getEvents,
  });

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('en-US', {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
    });
  };

  const formatPrice = (price: number): string => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(price);
  };

  return (
    <div>
      {/* Hero Section */}
      <section className={styles.hero}>
        <div className={styles.heroContent}>
          <h1 className={styles.heroTitle}>
            Find & Book Amazing Events
          </h1>
          <p className={styles.heroSubtitle}>
            Discover concerts, sports, theater, and more. 
            Secure your tickets with our fast and reliable booking system.
          </p>
          <div className={styles.heroButtons}>
            <Link to="/events" className={styles.heroButtonPrimary}>
              Browse Events
            </Link>
            <Link to="/register" className={styles.heroButtonSecondary}>
              Create Account
            </Link>
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section className={styles.features}>
        <div className={styles.featuresContainer}>
          <h2 className={styles.sectionTitle}>Why Choose TicketMaster?</h2>
          <div className={styles.featuresGrid}>
            <div className={styles.featureCard}>
              <Calendar className={styles.featureIcon} />
              <h3 className={styles.featureTitle}>Wide Selection</h3>
              <p className={styles.featureDescription}>
                Access thousands of events from concerts to sports games, 
                all in one place.
              </p>
            </div>
            <div className={styles.featureCard}>
              <Shield className={styles.featureIcon} />
              <h3 className={styles.featureTitle}>Secure Booking</h3>
              <p className={styles.featureDescription}>
                Your transactions are protected with industry-leading 
                security measures.
              </p>
            </div>
            <div className={styles.featureCard}>
              <Zap className={styles.featureIcon} />
              <h3 className={styles.featureTitle}>Instant Confirmation</h3>
              <p className={styles.featureDescription}>
                Get your tickets confirmed instantly with real-time 
                seat availability.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Featured Events Section */}
      <section className={styles.events}>
        <div className={styles.eventsContainer}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>Upcoming Events</h2>
            <Link to="/events" className={styles.viewAllLink}>
              View All →
            </Link>
          </div>

          {isLoading && (
            <div className={styles.loading}>
              <div className={styles.spinner} />
              <p>Loading events...</p>
            </div>
          )}

          {error && (
            <div className={styles.error}>
              <p>Failed to load events. Please try again later.</p>
            </div>
          )}

          {events && events.length > 0 ? (
            <div className={styles.eventsGrid}>
              {events.slice(0, 6).map((event: Event) => (
                <Link 
                  key={event.id} 
                  to={`/events/${event.id}`} 
                  className={styles.eventCardLink}
                >
                  <article className={styles.eventCard}>
                    <div className={styles.eventImage}>
                      <span className={styles.eventEmoji}>🎫</span>
                    </div>
                    <div className={styles.eventContent}>
                      <h3 className={styles.eventTitle}>{event.title}</h3>
                      <div className={styles.eventMeta}>
                        <div className={styles.eventMetaItem}>
                          <Calendar className={styles.eventMetaIcon} />
                          <span>{formatDate(event.event_date)}</span>
                        </div>
                        {event.venue && (
                          <div className={styles.eventMetaItem}>
                            <MapPin className={styles.eventMetaIcon} />
                            <span>{event.venue.name}</span>
                          </div>
                        )}
                      </div>
                      <div className={styles.eventPrice}>
                        {formatPrice(event.ticket_price)}
                      </div>
                    </div>
                  </article>
                </Link>
              ))}
            </div>
          ) : !isLoading && (
            <div className={styles.emptyState}>
              <p>No events available at the moment. Check back soon!</p>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
