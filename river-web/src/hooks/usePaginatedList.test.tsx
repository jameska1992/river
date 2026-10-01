import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createElement, type ReactNode } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { usePaginatedList, type Page } from './usePaginatedList'

// usePaginatedList reads/writes ?page=N via react-router, so every render needs
// a Router. A factory lets each test seed the initial URL.
function wrapperFor(initialUrl: string) {
  return ({ children }: { children: ReactNode }) =>
    createElement(MemoryRouter, { initialEntries: [initialUrl] }, children)
}

function page<T>(items: T[], total: number): Page<T> {
  return { items, total }
}

beforeEach(() => {
  // jsdom has no scrollTo; goToPage calls it after navigation.
  window.scrollTo = vi.fn()
})

describe('usePaginatedList', () => {
  it('loads page 1 on mount: loading → data', async () => {
    const fetchPage = vi.fn().mockResolvedValue(page(['a', 'b'], 2))
    const { result } = renderHook(() => usePaginatedList(fetchPage, 10), {
      wrapper: wrapperFor('/'),
    })

    expect(result.current.isLoading).toBe(true)
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(fetchPage).toHaveBeenCalledWith(1, 10)
    expect(result.current.items).toEqual(['a', 'b'])
    expect(result.current.total).toBe(2)
    expect(result.current.page).toBe(1)
    expect(result.current.error).toBeNull()
  })

  it('restores the page from the URL on mount', async () => {
    const fetchPage = vi.fn().mockResolvedValue(page(['x'], 100))
    const { result } = renderHook(() => usePaginatedList(fetchPage, 10), {
      wrapper: wrapperFor('/?page=3'),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(fetchPage).toHaveBeenCalledWith(3, 10)
    expect(result.current.page).toBe(3)
  })

  it('computes totalPages from total and pageSize', async () => {
    const fetchPage = vi.fn().mockResolvedValue(page([], 25))
    const { result } = renderHook(() => usePaginatedList(fetchPage, 10), {
      wrapper: wrapperFor('/'),
    })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.totalPages).toBe(3) // ceil(25 / 10)
  })

  it('totalPages is at least 1 even with zero results', async () => {
    const fetchPage = vi.fn().mockResolvedValue(page([], 0))
    const { result } = renderHook(() => usePaginatedList(fetchPage, 10), {
      wrapper: wrapperFor('/'),
    })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.totalPages).toBe(1)
  })

  it('sets an error and clears data when the fetch rejects', async () => {
    const fetchPage = vi.fn().mockRejectedValue(new Error('boom'))
    const { result } = renderHook(() => usePaginatedList(fetchPage, 10), {
      wrapper: wrapperFor('/'),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.error).toBe('boom')
    expect(result.current.items).toEqual([])
    expect(result.current.total).toBe(0)
  })

  it('uses a fallback message for a non-Error rejection', async () => {
    const fetchPage = vi.fn().mockRejectedValue('nope')
    const { result } = renderHook(() => usePaginatedList(fetchPage, 10), {
      wrapper: wrapperFor('/'),
    })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.error).toBe('Failed to load')
  })

  it('goToPage clamps to [1, totalPages], loads and scrolls', async () => {
    const fetchPage = vi.fn().mockResolvedValue(page(['a'], 25)) // 3 pages
    const { result } = renderHook(() => usePaginatedList(fetchPage, 10), {
      wrapper: wrapperFor('/'),
    })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    fetchPage.mockClear()

    await act(async () => { result.current.goToPage(2) })
    expect(result.current.page).toBe(2)
    expect(fetchPage).toHaveBeenCalledWith(2, 10)
    expect(window.scrollTo).toHaveBeenCalled()

    // beyond the last page clamps to totalPages (3)
    await act(async () => { result.current.goToPage(99) })
    expect(result.current.page).toBe(3)

    // below 1 clamps to 1
    await act(async () => { result.current.goToPage(-5) })
    expect(result.current.page).toBe(1)
  })

  it('goToPage is a no-op when the target equals the current page', async () => {
    const fetchPage = vi.fn().mockResolvedValue(page(['a'], 25))
    const { result } = renderHook(() => usePaginatedList(fetchPage, 10), {
      wrapper: wrapperFor('/'),
    })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    fetchPage.mockClear()

    await act(async () => { result.current.goToPage(1) })
    expect(fetchPage).not.toHaveBeenCalled()
  })

  it('resets to page 1 and reloads when the query (fetchPage) changes', async () => {
    const fetchA = vi.fn().mockResolvedValue(page(['a'], 100))
    const fetchB = vi.fn().mockResolvedValue(page(['b'], 100))
    const { result, rerender } = renderHook(
      ({ fp }: { fp: (p: number, l: number) => Promise<Page<string>> }) =>
        usePaginatedList(fp, 10),
      { wrapper: wrapperFor('/?page=4'), initialProps: { fp: fetchA } },
    )
    await waitFor(() => expect(result.current.page).toBe(4))
    expect(fetchA).toHaveBeenCalledWith(4, 10)

    rerender({ fp: fetchB })
    await waitFor(() => expect(result.current.page).toBe(1))
    expect(fetchB).toHaveBeenCalledWith(1, 10)
  })
})
