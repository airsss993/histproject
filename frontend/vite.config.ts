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
        target: 'https://api.russia-heroes.ru',
        changeOrigin: true,
        secure: true,
      }
    }
  }
})