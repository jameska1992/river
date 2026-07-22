import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    cssTarget: ['chrome111', 'safari18', 'firefox128', 'edge111'],
  },
  server: {
    host: true,
    port: 5174,
    strictPort: true,
    proxy: {
      // Proxy /api to river-api. Override with RIVER_API_TARGET to point
      // elsewhere, e.g. RIVER_API_TARGET=http://192.168.1.10:8080 npm run dev.
      '/api': {
        target: process.env.RIVER_API_TARGET || 'http://localhost:8080',
        ws: true,
      },
    },
  },
})
