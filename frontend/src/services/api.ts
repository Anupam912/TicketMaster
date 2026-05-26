/**
 * API Service - Handles all HTTP requests to the backend
 * 
 * Uses VITE_API_URL environment variable for production deployments.
 * Falls back to '/api' for local development (proxied by Vite).
 */

import axios from 'axios';
import type { AxiosError, InternalAxiosRequestConfig } from 'axios';
import type { ApiError } from '@/types';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api';

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

/**
 * Request Interceptor - Adds JWT token to every request
 */
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem('token');
    
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    
    return config;
  }
);

/**
 * Response Interceptor - Handles auth errors and formats error messages
 */
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ApiError>) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      window.location.href = '/login';
    }
    
    const message = error.response?.data?.error || 
                    error.response?.data?.message || 
                    'An unexpected error occurred';
    
    return Promise.reject(new Error(message));
  }
);

export default api;
