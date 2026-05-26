/**
 * Input Component - Reusable text input with label and error states
 * 
 * Uses CSS Modules for styling.
 */

import { forwardRef, useId } from 'react';
import type { InputHTMLAttributes } from 'react';
import styles from './Input.module.css';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

/**
 * Reusable Input component with label, error, and helper text support.
 */
export const Input = forwardRef<HTMLInputElement, InputProps>(
  function Input({ label, error, helperText, className = '', id, ...props }, ref) {
    const generatedId = useId();
    const inputId = id || generatedId;

    const inputClasses = [
      styles.input,
      error ? styles.inputError : '',
      className,
    ].filter(Boolean).join(' ');

    return (
      <div className={styles.inputWrapper}>
        {label && (
          <label htmlFor={inputId} className={styles.label}>
            {label}
          </label>
        )}

        <input
          ref={ref}
          id={inputId}
          className={inputClasses}
          aria-invalid={error ? 'true' : 'false'}
          aria-describedby={error ? `${inputId}-error` : undefined}
          {...props}
        />

        {error && (
          <p id={`${inputId}-error`} className={styles.errorMessage} role="alert">
            {error}
          </p>
        )}

        {!error && helperText && (
          <p className={styles.helperText}>{helperText}</p>
        )}
      </div>
    );
  }
);
