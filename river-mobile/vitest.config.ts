import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Separate from vite.config.ts so the production build stays vitest-free.
export default defineConfig({
  plugins: [react()],
  // Test files are excluded from tsconfig.app.json (so `tsc -b` ignores them),
  // which leaves esbuild without a jsx setting and defaulting to the classic
  // runtime. Force the automatic runtime so test JSX needs no React import.
  esbuild: { jsx: 'automatic', jsxImportSource: 'react' },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/setupTests.ts'],
    css: false,
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
