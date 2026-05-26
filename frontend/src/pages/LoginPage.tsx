/**
 * LoginPage - User authentication page
 */

import { useState } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/context/AuthContext';
import { Button, Input, Card } from '@/components/ui';
import styles from './LoginPage.module.css';

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { login, isAuthenticated } = useAuth();
  
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  // Redirect if already logged in
  if (isAuthenticated) {
    const from = (location.state as { from?: string })?.from || '/';
    navigate(from, { replace: true });
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      await login({ email, password });
      const from = (location.state as { from?: string })?.from || '/';
      navigate(from, { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className={styles.container}>
      <Card className={styles.card}>
        <Card.Header>
          <Card.Title>Welcome Back</Card.Title>
          <Card.Description>
            Sign in to your account to continue
          </Card.Description>
        </Card.Header>
        
        <Card.Content>
          <form onSubmit={handleSubmit} className={styles.form}>
            {error && (
              <div className={styles.error}>
                {error}
              </div>
            )}
            
            <Input
              label="Email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              required
              autoComplete="email"
            />
            
            <Input
              label="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              required
              autoComplete="current-password"
            />
            
            <Button 
              type="submit" 
              variant="primary" 
              isLoading={isLoading}
              className={styles.submitButton}
            >
              Sign In
            </Button>
          </form>
        </Card.Content>
        
        <Card.Footer className={styles.footer}>
          <p>
            Don't have an account?{' '}
            <Link to="/register" className={styles.link}>
              Sign up
            </Link>
          </p>
        </Card.Footer>
      </Card>
    </div>
  );
}
