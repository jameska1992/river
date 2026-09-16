import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { PosterCard } from './PosterCard'

const wrap = (ui: React.ReactNode) => render(<MemoryRouter>{ui}</MemoryRouter>)

describe('PosterCard', () => {
  it('renders title, subtitle and a link to `to`', () => {
    wrap(<PosterCard to="/movies/1" title="Dune" subtitle="2021" kind="movie" />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('2021')).toBeInTheDocument()
    expect(screen.getByRole('link')).toHaveAttribute('href', '/movies/1')
  })

  it('falls back to an icon (no <img>) when no image is given', () => {
    const { container } = wrap(<PosterCard to="/albums/1" title="No Art" kind="album" />)
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('svg')).not.toBeNull()
  })

  // The progress track is the only element styled `height: 3px`.
  const track = (c: HTMLElement) => Array.from(c.querySelectorAll('div')).find(d => d.style.height === '3px')

  it('renders a progress fill for continue-watching items', () => {
    const { container } = wrap(
      <PosterCard to="/movies/1" title="Half" kind="movie" position={50} duration={100} />,
    )
    const bar = track(container)
    expect(bar).toBeTruthy()
    expect((bar!.firstElementChild as HTMLElement).style.width).toBe('50%')
  })

  it('omits the progress bar when there is no progress', () => {
    const { container } = wrap(<PosterCard to="/movies/1" title="Fresh" kind="movie" position={0} duration={100} />)
    expect(track(container)).toBeUndefined()
  })
})
