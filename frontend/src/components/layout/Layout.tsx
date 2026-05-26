/**
 * Layout Component - Page wrapper
 * 
 * Uses CSS Modules for styling.
 */

import { Outlet } from 'react-router-dom';
import { Header } from './Header';
import styles from './Layout.module.css';

export function Layout() {
  return (
    <div className={styles.layout}>
      {/* Header */}
      <Header />
      
      {/* Main content area */}
      <main className={styles.main}>
        <Outlet />
      </main>
      
      {/* Footer */}
      <footer className={styles.footer}>
        <div className={styles.footerContainer}>
          <div className={styles.footerContent}>
            <p className={styles.copyright}>
              © 2026 TicketMaster. All rights reserved.
            </p>
            <div className={styles.footerLinks}>
              <a href="#" className={styles.footerLink}>
                Privacy Policy
              </a>
              <a href="#" className={styles.footerLink}>
                Terms of Service
              </a>
              <a href="#" className={styles.footerLink}>
                Contact
              </a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
