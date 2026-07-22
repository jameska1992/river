import { useState, useEffect, useCallback, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'

export interface Page<T> { items: T[]; total: number }

export function usePaginatedList<T>(
  fetchPage: (page: number, limit: number) => Promise<Page<T>>,
  pageSize: number,
) {
  const [searchParams, setSearchParams] = useSearchParams()

  const [items, setItems] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  // Seed the page from the URL so returning from a detail page (browser back)
  // restores the page the user was on instead of snapping to 1.
  const [page, setPage] = useState(() => Math.max(1, Number(searchParams.get('page')) || 1))
  // Capture the mount page so the first-load effect can restore it without
  // depending on `page` (which would refetch on every page change).
  const restorePageRef = useRef(page)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  // Mirror the current page into the URL (?page=N). Page 1 clears the param to
  // keep clean URLs. `replace` avoids stacking a history entry per page click,
  // so browser back goes straight from a detail page to the grid.
  const writePageParam = useCallback((p: number) => {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (p <= 1) next.delete('page')
      else next.set('page', String(p))
      return next
    }, { replace: true })
  }, [setSearchParams])

  const load = useCallback(async (p: number) => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await fetchPage(p, pageSize)
      setItems(data.items)
      setTotal(data.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load')
      setItems([])
      setTotal(0)
    } finally {
      setIsLoading(false)
    }
  }, [fetchPage, pageSize])

  // Restore the page from the URL on mount; snap back to page 1 only when the
  // query genuinely changes (library swap, sort change, page-size change) — the
  // previous index isn't meaningful against a re-sliced query.
  //
  // We key this on the identity of `load` rather than a first-run boolean so it
  // stays correct under React StrictMode, which double-invokes effects on mount
  // (setup → cleanup → setup). A boolean flag would flip on the first invoke
  // and make the second invoke wrongly reset to page 1; comparing `load`
  // identity, the StrictMode re-invoke sees the same value and doesn't reset.
  const loadedFor = useRef<typeof load | null>(null)
  useEffect(() => {
    if (loadedFor.current !== null && loadedFor.current !== load) {
      // Query changed — reset to page 1.
      loadedFor.current = load
      setPage(1)
      writePageParam(1)
      void load(1)
    } else {
      // Initial mount, or a StrictMode re-invoke of the same query — honour the
      // page restored from the URL.
      loadedFor.current = load
      void load(restorePageRef.current)
    }
  // writePageParam is stable (memoised on setSearchParams); intentionally
  // keyed only on load so URL updates don't retrigger a reset.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [load])

  const goToPage = useCallback((p: number) => {
    const clamped = Math.max(1, Math.min(p, totalPages))
    if (clamped === page) return
    setPage(clamped)
    writePageParam(clamped)
    void load(clamped)
    if (typeof window !== 'undefined') {
      window.scrollTo({ top: 0, behavior: 'smooth' })
    }
  }, [load, page, totalPages, writePageParam])

  return { items, total, page, totalPages, isLoading, error, goToPage }
}
