import { useState } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ExternalPersonResult } from '../api'

const { mockApi, MockApiError } = vi.hoisted(() => {
  class MockApiError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
    }
  }
  return { mockApi: { searchTMDBPeople: vi.fn() }, MockApiError }
})
vi.mock('../api', () => ({ api: mockApi, ApiError: MockApiError }))

import { PersonSearchInput } from './PersonSearchInput'

const BRAD: ExternalPersonResult = {
  tmdb_id: 287, name: 'Brad Pitt', profile_path: '', known_for_department: 'Acting',
  known_for: ['Fight Club', 'Se7en'], person_id: 'p1',
}
const BRADLEY: ExternalPersonResult = {
  tmdb_id: 51329, name: 'Bradley Cooper', profile_path: '', known_for_department: 'Acting',
  known_for: ['The Hangover'], person_id: null,
}

// Harness that owns the row state the way CastEditorModal does.
function Harness({ onSelect = vi.fn() }: { onSelect?: (p: ExternalPersonResult) => void }) {
  const [name, setName] = useState('')
  const [tmdbId, setTmdbId] = useState<number | null>(null)
  return (
    <form onSubmit={e => { e.preventDefault(); submitted(name) }}>
      <PersonSearchInput
        value={name}
        tmdbId={tmdbId}
        onChange={n => { setName(n); setTmdbId(null) }}
        onSelect={p => { setName(p.name); setTmdbId(p.tmdb_id); onSelect(p) }}
        onUnlink={() => setTmdbId(null)}
      />
    </form>
  )
}
const submitted = vi.fn()

beforeEach(() => {
  mockApi.searchTMDBPeople.mockReset()
  submitted.mockReset()
})

describe('PersonSearchInput', () => {
  it('debounces typing into a single TMDB search and lists the hits', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([BRAD, BRADLEY])
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox', { name: 'Name' }), 'brad')

    expect(await screen.findByRole('option', { name: /Brad Pitt/ })).toBeInTheDocument()
    expect(mockApi.searchTMDBPeople).toHaveBeenCalledTimes(1)
    expect(mockApi.searchTMDBPeople).toHaveBeenCalledWith('brad')
    // Known-for line and the in-library marker.
    expect(screen.getByRole('option', { name: /Brad Pitt/ })).toHaveTextContent('Acting · Fight Club, Se7en')
    expect(screen.getByRole('option', { name: /Brad Pitt/ })).toHaveTextContent('In library')
    expect(screen.getByRole('option', { name: /Bradley Cooper/ })).not.toHaveTextContent('In library')
  })

  it('does not search for queries shorter than two characters', async () => {
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'b')
    await new Promise(r => setTimeout(r, 400))

    expect(mockApi.searchTMDBPeople).not.toHaveBeenCalled()
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('links the row when a hit is clicked', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([BRAD])
    const onSelect = vi.fn()
    const user = userEvent.setup()
    render(<Harness onSelect={onSelect} />)

    await user.type(screen.getByRole('combobox'), 'brad')
    await user.click(await screen.findByRole('option', { name: /Brad Pitt/ }))

    expect(onSelect).toHaveBeenCalledWith(BRAD)
    expect(screen.getByRole('combobox')).toHaveValue('Brad Pitt')
    expect(screen.getByText('TMDB')).toBeInTheDocument()
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('picks with the keyboard, and Enter with nothing highlighted submits the typed name', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([BRAD, BRADLEY])
    const onSelect = vi.fn()
    const user = userEvent.setup()
    render(<Harness onSelect={onSelect} />)
    const input = screen.getByRole('combobox')

    await user.type(input, 'brad')
    await screen.findByRole('option', { name: /Brad Pitt/ })

    // Nothing pre-selected: Enter is a normal form submit of the free text.
    await user.keyboard('{Enter}')
    expect(onSelect).not.toHaveBeenCalled()
    expect(submitted).toHaveBeenCalledWith('brad')

    await user.keyboard('{ArrowDown}{ArrowDown}')
    expect(screen.getByRole('option', { name: /Bradley Cooper/ })).toHaveAttribute('aria-selected', 'true')
    await user.keyboard('{Enter}')
    expect(onSelect).toHaveBeenCalledWith(BRADLEY)
    expect(submitted).toHaveBeenCalledTimes(1) // the pick didn't submit
  })

  it('unlinks when the name is edited or the unlink button is pressed', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([BRAD])
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'brad')
    await user.click(await screen.findByRole('option', { name: /Brad Pitt/ }))
    await user.click(screen.getByRole('button', { name: 'Unlink from TMDB' }))
    expect(screen.queryByText('TMDB')).toBeNull()
    expect(screen.getByRole('combobox')).toHaveValue('Brad Pitt')

    await user.type(screen.getByRole('combobox'), 'brad')
    await user.click(await screen.findByRole('option', { name: /Brad Pitt/ }))
    await user.type(screen.getByRole('combobox'), 'x')
    expect(screen.queryByText('TMDB')).toBeNull()
  })

  it('shows a configuration hint when TMDB search is unavailable, and keeps free text', async () => {
    mockApi.searchTMDBPeople.mockRejectedValue(new MockApiError(503, 'TMDB API key is not configured'))
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'jane doe')

    expect(await screen.findByText(/TMDB search isn’t available/)).toBeInTheDocument()
    expect(screen.getByRole('combobox')).toHaveValue('jane doe')
  })

  it('shows a generic error on other failures', async () => {
    mockApi.searchTMDBPeople.mockRejectedValue(new Error('boom'))
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'jane')

    expect(await screen.findByText(/TMDB search failed/)).toBeInTheDocument()
  })

  it('says so when there are no matches', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([])
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'zzzz')

    expect(await screen.findByText(/No TMDB matches/)).toBeInTheDocument()
  })

  it('ignores a slow response for a query the user has typed past', async () => {
    let resolveFirst!: (v: ExternalPersonResult[]) => void
    mockApi.searchTMDBPeople
      .mockImplementationOnce(() => new Promise(r => { resolveFirst = r }))
      .mockResolvedValueOnce([BRADLEY])
    const user = userEvent.setup()
    render(<Harness />)
    const input = screen.getByRole('combobox')

    await user.type(input, 'brad')
    await waitFor(() => expect(mockApi.searchTMDBPeople).toHaveBeenCalledTimes(1))
    await user.type(input, 'ley')
    await screen.findByRole('option', { name: /Bradley Cooper/ })

    resolveFirst([BRAD])
    await new Promise(r => setTimeout(r, 0))
    expect(screen.queryByRole('option', { name: /Brad Pitt/ })).toBeNull()
  })

  it('Escape closes the list without bubbling to the dialog', async () => {
    mockApi.searchTMDBPeople.mockResolvedValue([BRAD])
    const docEscape = vi.fn()
    document.addEventListener('keydown', docEscape)
    const user = userEvent.setup()
    render(<Harness />)

    await user.type(screen.getByRole('combobox'), 'brad')
    await screen.findByRole('option', { name: /Brad Pitt/ })
    docEscape.mockClear()
    await user.keyboard('{Escape}')

    expect(screen.queryByRole('listbox')).toBeNull()
    expect(docEscape).not.toHaveBeenCalled()
    document.removeEventListener('keydown', docEscape)
  })
})
