/**
 * Card Component - Container for content
 * 
 * Uses CSS Modules for styling.
 * Supports compound component pattern: Card.Header, Card.Content, etc.
 */

import type { HTMLAttributes, PropsWithChildren } from 'react';
import styles from './Card.module.css';

/**
 * CardHeader - Header section of a card
 */
function CardHeader({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={`${styles.header} ${className}`} {...props}>
      {children}
    </div>
  );
}

/**
 * CardTitle - Title inside card header
 */
function CardTitle({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLHeadingElement>>) {
  return (
    <h3 className={`${styles.title} ${className}`} {...props}>
      {children}
    </h3>
  );
}

/**
 * CardDescription - Subtitle/description inside card header
 */
function CardDescription({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLParagraphElement>>) {
  return (
    <p className={`${styles.description} ${className}`} {...props}>
      {children}
    </p>
  );
}

/**
 * CardContent - Main content area of a card
 */
function CardContent({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={`${styles.content} ${className}`} {...props}>
      {children}
    </div>
  );
}

/**
 * CardFooter - Footer section of a card
 */
function CardFooter({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={`${styles.footer} ${className}`} {...props}>
      {children}
    </div>
  );
}

/**
 * Card - Main container with compound components
 */
function CardBase({ children, className = '', ...props }: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={`${styles.card} ${className}`} {...props}>
      {children}
    </div>
  );
}

export const Card = Object.assign(CardBase, {
  Header: CardHeader,
  Title: CardTitle,
  Description: CardDescription,
  Content: CardContent,
  Footer: CardFooter,
});

export { CardHeader, CardTitle, CardDescription, CardContent, CardFooter };
