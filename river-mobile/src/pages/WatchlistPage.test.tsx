import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'

vi.mock('../api', () => ({
  api: {
    getWatchlist: vi.fn(),
    removeFromWatchlist: vi.fn(),
  },
}))

import { api } from '../api'
import type { WatchlistItem } from '../api'
import WatchlistPage from './WatchlistPage'

const item: WatchlistItem = {
  id: 'w1',
  media_type: 'movie',
  media_id: 'm1',
  title: 'Dune',
  year: 2021,
  poster_path: '',
  added_at: '2026-01-01T00:00:00Z',
}

describe('WatchlistPage', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders fetched items and removes one optimistically', async () => {
    vi.mocked(api.getWatchlist).mockResolvedValue([item])
    vi.mocked(api.removeFromWatchlist).mockResolvedValue(undefined)

    render(<MemoryRouter><WatchlistPage /></MemoryRouter>)
    expect(await screen.findByText('Dune')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: /remove dune from watchlist/i }))

    await waitFor(() => expect(api.removeFromWatchlist).toHaveBeenCalledWith('w1'))
    await waitFor(() => expect(screen.queryByText('Dune')).toBeNull())
  })

  it('shows the empty state when the watchlist is empty', async () => {
    vi.mocked(api.getWatchlist).mockResolvedValue([])
    render(<MemoryRouter><WatchlistPage /></MemoryRouter>)
    expect(await screen.findByText(/your watchlist is empty/i)).toBeInTheDocument()
  })
})
