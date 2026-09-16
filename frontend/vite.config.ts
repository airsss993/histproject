import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': './src',
    }
  },
  server: {
    port: 3000, // 2005
    open: true,
    host: true,
    proxy: {
      '/api': {
        // Caddy из backend/docker-compose.dev.yml
        target: process.env.VITE_API_PROXY ?? 'http://127.0.0.1:8080',
        changeOrigin: true,
      }
    }
  }
})