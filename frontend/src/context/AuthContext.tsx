/**
 * Auth Context - Global Authentication State
 * 
 * React Context provides a way to pass data through the component tree
 * without having to pass props manually at every level.
 * 
 * This context provides:
 * - Current user state
 * - Login/logout functions
 * - Loading state
 * - isAuthenticated check
 */

import { createContext, useContext, useState, useEffect, useCallback } from 'react';
import type { ReactNode } from 'react';
import type { User, LoginRequest, RegisterRequest } from '@/types';
import * as authService from '@/services/auth';

// Define what the context will provide
interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (credentials: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => void;
}

// Create the context with undefined as default
// We'll throw an error if someone tries to use it outside a provider
const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Props for the provider component
interface AuthProviderProps {
  children: ReactNode;  // ReactNode = any valid React child (elements, strings, etc.)
}

/**
 * AuthProvider Component
 * 
 * Wrap your app with this to provide auth state to all children.
 * 
 * Usage:
 * <AuthProvider>
 *   <App />
 * </AuthProvider>
 */
export function AuthProvider({ children }: AuthProviderProps) {
  // State for the current user
  const [user, setUser] = useState<User | null>(null);
  
  // Loading state - true while we're checking if user is logged in
  const [isLoading, setIsLoading] = useState(true);

  // Check if user is logged in on initial load
  useEffect(() => {
    const initAuth = async () => {
      const token = localStorage.getItem('token');
      const storedUser = localStorage.getItem('user');
      
      if (token && storedUser) {
        try {
          // Verify token is still valid by fetching current user
          const { user } = await authService.getCurrentUser();
          setUser(user);
          localStorage.setItem('user', JSON.stringify(user));
        } catch {
          // Token invalid, clear storage
          localStorage.removeItem('token');
          localStorage.removeItem('user');
        }
      }
      
      setIsLoading(false);
    };

    initAuth();
  }, []);

  // Login function - useCallback prevents unnecessary re-renders
  const login = useCallback(async (credentials: LoginRequest) => {
    const response = await authService.login(credentials);
    
    // Store token and user in localStorage
    localStorage.setItem('token', response.token);
    localStorage.setItem('user', JSON.stringify(response.user));
    
    // Update state
    setUser(response.user);
  }, []);

  // Register function
  const register = useCallback(async (data: RegisterRequest) => {
    const response = await authService.register(data);
    
    localStorage.setItem('token', response.token);
    localStorage.setItem('user', JSON.stringify(response.user));
    
    setUser(response.user);
  }, []);

  // Logout function
  const logout = useCallback(() => {
    authService.logout();
    setUser(null);
  }, []);

  // The value that will be available to consumers of this context
  const value: AuthContextType = {
    user,
    isAuthenticated: !!user,  // Convert to boolean
    isLoading,
    login,
    register,
    logout,
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

/**
 * useAuth Hook - Easy way to access auth context
 * 
 * Usage:
 * const { user, login, logout } = useAuth();
 */
export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  
  return context;
}
