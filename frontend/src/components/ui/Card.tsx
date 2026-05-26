/**
 * Card Component - Container for content
 * 
 * Uses CSS Modules for styling.
 */

import type { HTMLAttributes, PropsWithChildren } from 'react';
import styles from './Card.module.css';

/**
 * Card - Main container
 */
export function Card({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={`${styles.card} ${className}`} {...props}>
      {children}
    </div>
  );
}

/**
 * CardHeader - Header section of a card
 */
export function CardHeader({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={`${styles.header} ${className}`} {...props}>
      {children}
    </div>
  );
}

/**
 * CardTitle - Title inside card header
 */
export function CardTitle({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLHeadingElement>>) {
  return (
    <h3 className={`${styles.title} ${className}`} {...props}>
      {children}
    </h3>
  );
}

/**
 * CardDescription - Subtitle/description inside card header
 */
export function CardDescription({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLParagraphElement>>) {
  return (
    <p className={`${styles.description} ${className}`} {...props}>
      {children}
    </p>
  );
}

/**
 * CardContent - Main content area of a card
 */
export function CardContent({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={`${styles.content} ${className}`} {...props}>
      {children}
    </div>
  );
}

/**
 * CardFooter - Footer section of a card
 */
export function CardFooter({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={`${styles.footer} ${className}`} {...props}>
      {children}
    </div>
  );
}
