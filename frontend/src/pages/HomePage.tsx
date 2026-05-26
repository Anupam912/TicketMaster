/**
 * Home Page - Landing page with hero section and featured events
 */

import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Calendar, MapPin, Shield, ArrowRight } from 'lucide-react';
import { Button, Card, CardContent } from '@/components/ui';
import { getEvents } from '@/services/events';
import type { Event } from '@/types';
import styles from './HomePage.module.css';

export function HomePage() {
  // Fetch events using TanStack Query
  // useQuery handles loading, caching, and error states automatically
  const { data: events, isLoading, error } = useQuery({
    queryKey: ['events'],  // Unique key for caching
    queryFn: getEvents,    // Function to fetch data
  });

  return (
    <div>
      {/* Hero Section */}
      <section className={styles.hero}>
        <div className={styles.heroContainer}>
          <h1 className={styles.heroTitle}>
            Find & Book Amazing Events
          </h1>
          <p className={styles.heroSubtitle}>
            Discover concerts, sports, theater, and more. 
            Secure your tickets with our fast and reliable booking system.
          </p>
          <div className={styles.heroButtons}>
            <Link to="/events">
              <Button variant="secondary" size="lg">
                Browse Events
              </Button>
            </Link>
            <Link to="/register">
              <Button variant="secondary" size="lg">
                Create Account
              </Button>
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
              <MapPin className={styles.featureIcon} />
              <h3 className={styles.featureTitle}>Choose Your Seat</h3>
              <p className={styles.featureDescription}>
                Select your preferred seats with our interactive 
                seat selection system.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Featured Events Section */}
      <section className={styles.eventsSection}>
        <div className={styles.featuresContainer}>
          <div className={styles.eventsHeader}>
            <h2 className={styles.sectionTitle}>Upcoming Events</h2>
            <Link to="/events" className={styles.viewAllLink}>
              View All <ArrowRight size={16} />
            </Link>
          </div>

          {isLoading && (
            <p className={styles.loadingText}>Loading events...</p>
          )}

          {error && (
            <p className={styles.errorText}>
              Failed to load events. Please try again later.
            </p>
          )}

          {events && (
            <div className={styles.eventsGrid}>
              {events.slice(0, 6).map((event) => (
                <EventCard key={event.id} event={event} />
              ))}
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

// Event Card Component
interface EventCardProps {
  event: Event;
}

function EventCard({ event }: EventCardProps) {
  const eventDate = new Date(event.event_date);
  const formattedDate = eventDate.toLocaleDateString('en-US', {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  });
  const formattedTime = eventDate.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
  });

  return (
    <Link to={`/events/${event.id}`} className={styles.eventCard}>
      <Card>
        <CardContent>
          <p className={styles.eventDate}>
            {formattedDate} • {formattedTime}
          </p>
          <h3 className={styles.eventTitle}>
            {event.title}
          </h3>
          <p className={styles.eventDescription}>
            {event.description}
          </p>
          <div className={styles.eventFooter}>
            <span className={styles.eventPrice}>
              ${event.ticket_price.toFixed(2)}
            </span>
            <span className={event.available_seats > 0 ? styles.seatsAvailable : styles.soldOut}>
              {event.available_seats > 0 
                ? `${event.available_seats} seats left` 
                : 'Sold out'}
            </span>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
