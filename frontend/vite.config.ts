import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    // Path aliases - import like '@/components/Button' instead of '../../components/Button'
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  css: {
    // CSS Modules configuration
    modules: {
      // Use camelCase for class names in JS
      localsConvention: 'camelCase',
    },
  },
  server: {
    port: 3000,
    // Proxy API requests to backend during development
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
