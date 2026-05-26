/**
 * Application Entry Point
 * 
 * This is where React mounts to the DOM.
 * StrictMode enables additional development checks.
 */

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './index.css';

// Get the root element from index.html
const rootElement = document.getElementById('root');

if (!rootElement) {
  throw new Error('Root element not found');
}

// Create React root and render the app
createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>
);
