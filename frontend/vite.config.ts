import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [
    react(),
    // Общий лимит поднят под MapLibre, остальные чанки держим в прежних 500 КБ.
    {
      name: 'app-chunk-size',
      generateBundle(_, bundle) {
        for (const c of Object.values(bundle)) {
          if (c.type === 'chunk' && c.name !== 'maplibre' && c.code.length > 500_000) {
            this.warn(`${c.fileName}: ${Math.round(c.code.length / 1000)} kB, больше 500 kB`)
          }
        }
      },
    },
  ],
  build: {
    // MapLibre весит ~1 МБ сам по себе и обновляется редко: отдельный чанк остаётся в кеше браузера между деплоями.
    chunkSizeWarningLimit: 1100,
    rollupOptions: {
      output: { manualChunks: { maplibre: ['maplibre-gl'] } },
    },
  },
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