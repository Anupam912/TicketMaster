/**
 * Auth Service - Handles authentication API calls
 * 
 * This file contains all authentication-related API calls.
 * Separating services by domain makes code easier to maintain.
 */

import api from './api';
import type { AuthResponse, LoginRequest, RegisterRequest, User } from '@/types';

/**
 * Login user with email and password
 * 
 * @param credentials - Email and password
 * @returns AuthResponse with user data and JWT token
 */
export async function login(credentials: LoginRequest): Promise<AuthResponse> {
  const response = await api.post<AuthResponse>('/auth/login', credentials);
  return response.data;
}

/**
 * Register a new user
 * 
 * @param data - Registration data (email, password, full_name)
 * @returns AuthResponse with user data and JWT token
 */
export async function register(data: RegisterRequest): Promise<AuthResponse> {
  const response = await api.post<AuthResponse>('/auth/register', data);
  return response.data;
}

/**
 * Get current authenticated user
 * 
 * @returns User object of the currently logged in user
 */
export async function getCurrentUser(): Promise<{ user: User }> {
  const response = await api.get<{ user: User }>('/auth/me');
  return response.data;
}

/**
 * Logout user
 * 
 * Clears local storage and could call a logout endpoint if one exists
 */
export function logout(): void {
  localStorage.removeItem('token');
  localStorage.removeItem('user');
}
