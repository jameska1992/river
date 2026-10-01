import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Test config is kept separate from vite.config.ts so the production build
// (tsc -b && vite build) stays untouched. The react plugin is re-declared
// here because Vitest loads this file independently of the dev/build config.
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/setupTests.ts'],
    css: false,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      // Only measure the app source. Config, entrypoints, and generated or
      // compile-time-only files carry no runtime behaviour worth covering.
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/**/*.test.{ts,tsx}',
        'src/setupTests.ts',
        'src/main.tsx',
        'src/vite-env.d.ts',
        'src/api/types.ts',
      ],
    },
  },
})
