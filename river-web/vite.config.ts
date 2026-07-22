import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // Pin the CSS minifier target to modern browsers so Lightning CSS keeps the
  // unprefixed `backdrop-filter` declarations. Without this it strips the
  // unprefixed form, leaving only `-webkit-backdrop-filter` which Chrome
  // treats as a legacy alias and refuses to apply.
  build: {
    cssTarget: ['chrome111', 'safari18', 'firefox128', 'edge111'],
  },
  server: {
    proxy: {
      // Proxy /api (and its WebSocket endpoints) to river-api on the LAN.
      // Override with RIVER_API_TARGET to point elsewhere, e.g.
      // RIVER_API_TARGET=http://localhost:8080 npm run dev.
      '/api': {
        target: process.env.RIVER_API_TARGET || 'http://localhost:8080',
        ws: true,
      },
    },
  },
})
