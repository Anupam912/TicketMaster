/**
 * App Component - Main application entry point
 * 
 * This component sets up:
 * - React Router for navigation
 * - TanStack Query for data fetching
 * - Auth Context for authentication state
 */

import { BrowserRouter, Routes, Route, Link } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider } from '@/context/AuthContext';
import { Layout } from '@/components/layout';
import { 
  HomePage, 
  LoginPage, 
  RegisterPage, 
  EventsPage, 
  EventDetailPage,
  MyBookingsPage 
} from '@/pages';
import styles from './App.module.css';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 30 * 1000,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route index element={<HomePage />} />
              <Route path="events" element={<EventsPage />} />
              <Route path="events/:id" element={<EventDetailPage />} />
              <Route path="login" element={<LoginPage />} />
              <Route path="register" element={<RegisterPage />} />
              <Route path="my-bookings" element={<MyBookingsPage />} />
              <Route path="*" element={<NotFoundPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  );
}

function NotFoundPage() {
  return (
    <div className={styles.notFoundPage}>
      <h1 className={styles.notFoundCode}>404</h1>
      <h2 className={styles.notFoundTitle}>Page Not Found</h2>
      <p className={styles.notFoundText}>
        The page you're looking for doesn't exist or has been moved.
      </p>
      <Link to="/" className={styles.notFoundLink}>
        Go back home
      </Link>
    </div>
  );
}

export default App;
