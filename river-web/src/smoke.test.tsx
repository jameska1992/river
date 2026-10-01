import { render, screen } from '@testing-library/react'

// Smoke test for the test harness itself (issue #211), not for app code.
// It proves the full stack is wired: Vitest executes, jsdom provides a DOM,
// @testing-library/react can render, and the @testing-library/jest-dom
// matchers are registered. Real unit tests land in the follow-up issues.
describe('test harness', () => {
  it('runs Vitest', () => {
    expect(1 + 1).toBe(2)
  })

  it('renders into jsdom with jest-dom matchers available', () => {
    render(<p>river</p>)
    expect(screen.getByText('river')).toBeInTheDocument()
  })
})
