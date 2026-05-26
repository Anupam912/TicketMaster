/**
 * Header Component - Main navigation bar
 */

import { Link, useNavigate } from 'react-router-dom';
import { Ticket, User, LogOut, Menu, X } from 'lucide-react';
import { useState, useCallback } from 'react';
import { useAuth } from '@/context/AuthContext';
import { Button } from '@/components/ui';
import styles from './Header.module.css';

export function Header() {
  const { user, isAuthenticated, logout } = useAuth();
  const navigate = useNavigate();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  const closeMobileMenu = useCallback(() => {
    setIsMobileMenuOpen(false);
  }, []);

  const toggleMobileMenu = useCallback(() => {
    setIsMobileMenuOpen(prev => !prev);
  }, []);

  const handleLogout = useCallback(() => {
    logout();
    navigate('/');
  }, [logout, navigate]);

  const handleMobileLogout = useCallback(() => {
    handleLogout();
    closeMobileMenu();
  }, [handleLogout, closeMobileMenu]);

  return (
    <header className={styles.header}>
      <div className={styles.container}>
        <div className={styles.headerContent}>
          <Link to="/" className={styles.logo}>
            <Ticket className={styles.logoIcon} />
            <span className={styles.logoText}>TicketMaster</span>
          </Link>

          <nav className={styles.desktopNav}>
            <Link to="/events" className={styles.navLink}>
              Events
            </Link>
            
            {isAuthenticated ? (
              <>
                <Link to="/my-bookings" className={styles.navLink}>
                  My Bookings
                </Link>
                
                <div className={styles.userSection}>
                  <div className={styles.userInfo}>
                    <User className={styles.userIcon} />
                    <span className={styles.userName}>{user?.full_name}</span>
                  </div>
                  
                  <Button 
                    variant="ghost" 
                    size="sm"
                    onClick={handleLogout}
                    leftIcon={<LogOut />}
                  >
                    Logout
                  </Button>
                </div>
              </>
            ) : (
              <div className={styles.authButtons}>
                <Link to="/login">
                  <Button variant="ghost">Login</Button>
                </Link>
                <Link to="/register">
                  <Button variant="primary">Sign Up</Button>
                </Link>
              </div>
            )}
          </nav>

          <button
            type="button"
            className={styles.mobileMenuButton}
            onClick={toggleMobileMenu}
            aria-label="Toggle menu"
            aria-expanded={isMobileMenuOpen}
          >
            {isMobileMenuOpen ? (
              <X className={styles.menuIcon} />
            ) : (
              <Menu className={styles.menuIcon} />
            )}
          </button>
        </div>

        {isMobileMenuOpen && (
          <div className={styles.mobileNav}>
            <nav className={styles.mobileNavContent}>
              <Link 
                to="/events" 
                className={styles.mobileNavLink}
                onClick={closeMobileMenu}
              >
                Events
              </Link>
              
              {isAuthenticated ? (
                <>
                  <Link 
                    to="/my-bookings" 
                    className={styles.mobileNavLink}
                    onClick={closeMobileMenu}
                  >
                    My Bookings
                  </Link>
                  <div className={styles.mobileUserSection}>
                    <p className={styles.mobileUserEmail}>
                      Signed in as {user?.email}
                    </p>
                    <Button 
                      variant="outline" 
                      size="sm"
                      onClick={handleMobileLogout}
                    >
                      Logout
                    </Button>
                  </div>
                </>
              ) : (
                <div className={styles.mobileAuthButtons}>
                  <Link to="/login" onClick={closeMobileMenu}>
                    <Button variant="outline">Login</Button>
                  </Link>
                  <Link to="/register" onClick={closeMobileMenu}>
                    <Button variant="primary">Sign Up</Button>
                  </Link>
                </div>
              )}
            </nav>
          </div>
        )}
      </div>
    </header>
  );
}
