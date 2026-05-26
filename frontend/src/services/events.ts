/**
 * Events Service - Handles event-related API calls
 */

import api from './api';
import type { Event, SeatsResponse } from '@/types';

/**
 * Get all events
 * 
 * @returns Array of events
 */
export async function getEvents(): Promise<Event[]> {
  const response = await api.get<Event[]>('/events');
  return response.data;
}

/**
 * Get a single event by ID
 * 
 * @param id - Event UUID
 * @returns Event details
 */
export async function getEvent(id: string): Promise<Event> {
  const response = await api.get<Event>(`/events/${id}`);
  return response.data;
}

/**
 * Get seats for an event
 * 
 * @param eventId - Event UUID
 * @param status - Optional filter by seat status ('available', 'reserved', 'sold')
 * @returns Seats response with seats array and summary
 */
export async function getEventSeats(
  eventId: string, 
  status?: string
): Promise<SeatsResponse> {
  const params = status ? { status } : {};
  const response = await api.get<SeatsResponse>(`/events/${eventId}/seats`, { params });
  return response.data;
}
