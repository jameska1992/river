import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Credits, ExternalPersonResult } from '../api'

const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    searchTMDBPeople: vi.fn(),
    getTMDBPerson: vi.fn(),
    setMovieCredits: vi.fn(),
    getMovieCredits: vi.fn(),
    setTVShowCredits: vi.fn(),
    getTVShowCredits: vi.fn(),
  },
}))
vi.mock('../api', () => ({ api: mockApi, ApiError: class extends Error { status = 0 } }))

import { CastEditorModal } from './CastEditorModal'

const EMPTY: Credits = { cast: [], crew: [] }
const NOLAN: ExternalPersonResult = {
  tmdb_id: 525, name: 'Christopher Nolan', profile_path: 'https://image.tmdb.org/t/p/original/n.jpg',
  known_for_department: 'Directing', known_for: ['Inception'], person_id: null,
}

function renderModal(type: 'movie' | 'tvshow' = 'movie') {
  const onSaved = vi.fn()
  render(<CastEditorModal type={type} mediaId="m1" credits={EMPTY} onSaved={onSaved} onClose={vi.fn()} />)
  return { onSaved }
}

beforeEach(() => {
  Object.values(mockApi).forEach(f => f.mockReset())
  mockApi.setMovieCredits.mockResolvedValue(undefined)
  mockApi.getMovieCredits.mockResolvedValue(EMPTY)
  mockApi.setTVShowCredits.mockResolvedValue(undefined)
  mockApi.getTVShowCredits.mockResolvedValue(EMPTY)
})

describe('CastEditorModal TMDB person search', () => {
  it('saves a picked cast member with their tmdb_id, photo and biography', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([NOLAN])
    mockApi.getTMDBPerson.mockResolvedValue({ ...NOLAN, biography: 'British-American filmmaker.' })
    const user = userEvent.setup()
    renderModal()

    await user.click(screen.getByRole('button', { name: /Add cast member/ }))
    await user.type(screen.getByRole('combobox', { name: 'Name' }), 'nolan')
    await user.click(await screen.findByRole('option', { name: /Christopher Nolan/ }))
    await user.type(screen.getByRole('textbox', { name: 'Character' }), 'Himself')
    await waitFor(() => expect(mockApi.getTMDBPerson).toHaveBeenCalledWith(525))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockApi.setMovieCredits).toHaveBeenCalled())
    const [, payload] = mockApi.setMovieCredits.mock.calls[0]
    expect(payload.cast).toEqual([{
      tmdb_id: 525,
      biography: 'British-American filmmaker.',
      name: 'Christopher Nolan',
      profile_path: NOLAN.profile_path,
      character: 'Himself',
      order: 0,
    }])
  })

  it('defaults a picked crew member’s department and tolerates a failed bio lookup', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([NOLAN])
    mockApi.getTMDBPerson.mockRejectedValue(new Error('tmdb down'))
    const user = userEvent.setup()
    renderModal('tvshow')

    await user.click(screen.getByRole('tab', { name: /Crew/ }))
    await user.click(screen.getByRole('button', { name: /Add crew member/ }))
    await user.type(screen.getByRole('combobox', { name: 'Name' }), 'nolan')
    await user.click(await screen.findByRole('option', { name: /Christopher Nolan/ }))
    expect(screen.getByRole('textbox', { name: 'Department' })).toHaveValue('Directing')
    await user.type(screen.getByRole('textbox', { name: 'Job' }), 'Director')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockApi.setTVShowCredits).toHaveBeenCalled())
    const [, payload] = mockApi.setTVShowCredits.mock.calls[0]
    expect(payload.crew).toEqual([{
      tmdb_id: 525, name: 'Christopher Nolan', profile_path: NOLAN.profile_path, job: 'Director', department: 'Directing',
    }])
  })

  it('still saves a free-text person without a tmdb_id', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([])
    const user = userEvent.setup()
    renderModal()

    await user.click(screen.getByRole('button', { name: /Add cast member/ }))
    await user.type(screen.getByRole('combobox', { name: 'Name' }), 'Jane Doe')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockApi.setMovieCredits).toHaveBeenCalled())
    const [, payload] = mockApi.setMovieCredits.mock.calls[0]
    expect(payload.cast).toEqual([{ name: 'Jane Doe', profile_path: '', character: '', order: 0 }])
  })
})
