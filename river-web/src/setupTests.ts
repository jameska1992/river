// Registers @testing-library/jest-dom matchers (e.g. toBeInTheDocument) on
// Vitest's expect and augments its types. Loaded via setupFiles in
// vitest.config.ts, so every test file gets the matchers automatically.
import '@testing-library/jest-dom/vitest'
