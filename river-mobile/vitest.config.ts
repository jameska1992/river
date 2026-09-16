import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Separate from vite.config.ts so the production build stays vitest-free.
export default defineConfig({
  plugins: [react()],
  // JSX is handled by @vitejs/plugin-react + vitest's transformer (oxc as of
  // vitest 5), both of which default to the automatic runtime, so test JSX
  // needs no React import even though test files are excluded from
  // tsconfig.app.json.
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/setupTests.ts'],
    css: false,
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
