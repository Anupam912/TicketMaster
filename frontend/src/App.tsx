/**
 * App Component - Main application entry point
 * 
 * This component sets up:
 * - React Router for navigation
 * - TanStack Query for data fetching
 * - Auth Context for authentication state
 */

import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider } from '@/context/AuthContext';
import { Layout } from '@/components/layout';
import { HomePage } from '@/pages/HomePage';
import styles from './App.module.css';

// Create a QueryClient instance for TanStack Query
// This manages caching, refetching, and state for all queries
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Don't refetch on window focus by default
      refetchOnWindowFocus: false,
      // Retry failed requests once
      retry: 1,
      // Data is considered fresh for 30 seconds
      staleTime: 30 * 1000,
    },
  },
});

function App() {
  return (
    // QueryClientProvider makes TanStack Query available throughout the app
    <QueryClientProvider client={queryClient}>
      {/* AuthProvider makes auth state available throughout the app */}
      <AuthProvider>
        {/* BrowserRouter enables client-side routing */}
        <BrowserRouter>
          <Routes>
            {/* Layout wraps all routes with header and footer */}
            <Route path="/" element={<Layout />}>
              {/* Index route - shown at "/" */}
              <Route index element={<HomePage />} />
              
              {/* Placeholder routes - we'll build these next */}
              <Route path="events" element={<PlaceholderPage title="Events" />} />
              <Route path="events/:id" element={<PlaceholderPage title="Event Details" />} />
              <Route path="login" element={<PlaceholderPage title="Login" />} />
              <Route path="register" element={<PlaceholderPage title="Register" />} />
              <Route path="my-bookings" element={<PlaceholderPage title="My Bookings" />} />
              
              {/* 404 - catch all unmatched routes */}
              <Route path="*" element={<NotFoundPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  );
}

// Temporary placeholder for routes we haven't built yet
function PlaceholderPage({ title }: { title: string }) {
  return (
    <div className={styles.placeholderPage}>
      <h1 className={styles.placeholderTitle}>{title}</h1>
      <p className={styles.placeholderText}>
        This page is coming soon. We're building it step by step!
      </p>
    </div>
  );
}

// 404 Not Found page
function NotFoundPage() {
  return (
    <div className={styles.notFoundPage}>
      <h1 className={styles.notFoundCode}>404</h1>
      <h2 className={styles.notFoundTitle}>Page Not Found</h2>
      <p className={styles.notFoundText}>
        The page you're looking for doesn't exist or has been moved.
      </p>
      <a href="/" className={styles.notFoundLink}>
        Go back home
      </a>
    </div>
  );
}

export default App;
