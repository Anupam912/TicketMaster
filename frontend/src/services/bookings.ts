/**
 * Bookings Service - Handles booking-related API calls
 */

import api from './api';
import type { 
  Booking, 
  ReserveSeatRequest, 
  ReserveResponse, 
  JobStatus,
  PurchaseRequest 
} from '@/types';

/**
 * Reserve a seat for an event
 * Returns a job ID for async processing
 */
export async function reserveSeat(data: ReserveSeatRequest): Promise<ReserveResponse> {
  const response = await api.post<ReserveResponse>('/bookings/reserve', data);
  return response.data;
}

/**
 * Check the status of a reservation job
 */
export async function getJobStatus(jobId: string): Promise<JobStatus> {
  const response = await api.get<JobStatus>(`/bookings/job/${jobId}`);
  return response.data;
}

/**
 * Get all bookings for the current user
 */
export async function getMyBookings(): Promise<Booking[]> {
  const response = await api.get<Booking[] | { bookings: Booking[] }>('/bookings/my-bookings');
  // Handle both array and object responses
  const data = response.data;
  if (Array.isArray(data)) {
    return data;
  }
  if (data && 'bookings' in data) {
    return data.bookings;
  }
  return [];
}

/**
 * Get a single booking by ID
 */
export async function getBooking(id: string): Promise<Booking> {
  const response = await api.get<Booking>(`/bookings/${id}`);
  return response.data;
}

/**
 * Complete a purchase for a booking
 */
export async function purchaseBooking(data: PurchaseRequest): Promise<Booking> {
  const response = await api.post<Booking>('/bookings/purchase', data);
  return response.data;
}

/**
 * Cancel a booking
 */
export async function cancelBooking(id: string): Promise<void> {
  await api.delete(`/bookings/${id}`);
}

/**
 * Poll job status until completion or timeout
 */
export async function waitForJobCompletion(
  jobId: string, 
  maxAttempts: number = 10,
  intervalMs: number = 1000
): Promise<JobStatus> {
  let attempts = 0;
  
  while (attempts < maxAttempts) {
    const status = await getJobStatus(jobId);
    
    if (status.status === 'completed' || status.status === 'failed') {
      return status;
    }
    
    attempts++;
    await new Promise(resolve => setTimeout(resolve, intervalMs));
  }
  
  throw new Error('Job timed out');
}
