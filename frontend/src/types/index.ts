/**
 * TypeScript Types for the TicketMaster API
 * 
 * Types define the "shape" of data. TypeScript checks that your code
 * uses data correctly at compile time, catching bugs early.
 * 
 * Example: If you try to access `user.nam` (typo), TypeScript will error
 */

// User represents an authenticated user
export interface User {
  id: string;
  email: string;
  full_name: string;
  role: 'user' | 'admin';  // Union type - can only be these two values
  created_at: string;
  updated_at: string;
}

// AuthResponse is what we get back from login/register
export interface AuthResponse {
  user: User;
  token: string;
}

// LoginRequest is what we send to login
export interface LoginRequest {
  email: string;
  password: string;
}

// RegisterRequest is what we send to register
export interface RegisterRequest {
  email: string;
  password: string;
  full_name: string;
}

// Venue represents an event location
export interface Venue {
  id: string;
  name: string;
  address: string;
  city: string;
  capacity: number;
  created_at: string;
}

// Event represents a ticketed event
export interface Event {
  id: string;
  title: string;
  description: string;
  venue_id: string;
  venue?: Venue;  // Optional - may be included in response
  event_date: string;
  ticket_price: number;
  total_seats: number;
  available_seats: number;
  created_at: string;
  updated_at: string;
}

// Seat status enum - using const assertion for type safety
export const SeatStatus = {
  AVAILABLE: 'available',
  RESERVED: 'reserved',
  SOLD: 'sold',
} as const;

// Create a type from the enum values
export type SeatStatusType = typeof SeatStatus[keyof typeof SeatStatus];

// Seat represents a seat in an event
export interface Seat {
  id: string;
  event_id: string;
  seat_number: string;
  row_number: string;
  section: string;
  status: SeatStatusType;
  reserved_at?: string;
  reserved_until?: string;
  created_at: string;
}

// SeatsResponse is what we get from the seats endpoint
export interface SeatsResponse {
  event_id: string;
  seats: Seat[];
  summary: {
    total: number;
    available: number;
    reserved: number;
    sold: number;
  };
}

// Booking status
export const BookingStatus = {
  PENDING: 'pending',
  CONFIRMED: 'confirmed',
  CANCELLED: 'cancelled',
  EXPIRED: 'expired',
} as const;

export type BookingStatusType = typeof BookingStatus[keyof typeof BookingStatus];

// Booking represents a ticket booking
export interface Booking {
  id: string;
  user_id: string;
  event_id: string;
  seat_id: string;
  status: BookingStatusType;
  total_amount: number;
  reserved_at: string;
  expires_at: string;
  confirmed_at?: string;
  created_at: string;
  updated_at: string;
  event?: Event;
  seat?: Seat;
}

// ReserveSeatRequest is what we send to reserve a seat
export interface ReserveSeatRequest {
  event_id: string;
  seat_number: string;
}

// ReserveResponse is what we get back (async job)
export interface ReserveResponse {
  job_id: string;
  message: string;
}

// JobStatus is what we get when checking async job status
export interface JobStatus {
  job_id: string;
  status: 'pending' | 'processing' | 'completed' | 'failed';
  booking_id?: string;
  error?: string;
  created_at: string;
  updated_at: string;
}

// PurchaseRequest is what we send to complete a purchase
export interface PurchaseRequest {
  booking_id: string;
  payment_method: string;
}

// API Error response
export interface ApiError {
  error: string;
  message?: string;
}
