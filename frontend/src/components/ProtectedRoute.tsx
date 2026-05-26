/**
 * ProtectedRoute - Wrapper for routes that require authentication
 * 
 * Redirects unauthenticated users to login page,
 * preserving the intended destination for post-login redirect.
 */

import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/context/AuthContext';
import type { PropsWithChildren } from 'react';

interface ProtectedRouteProps extends PropsWithChildren {
  /** If true, only admins can access this route */
  requireAdmin?: boolean;
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const { isAuthenticated, user, isLoading } = useAuth();
  const location = useLocation();

  // Show nothing while checking auth status
  if (isLoading) {
    return null;
  }

  // Redirect to login if not authenticated
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  }

  // Check admin requirement
  if (requireAdmin && user?.role !== 'admin') {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}
