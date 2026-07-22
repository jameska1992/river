import { useCallback } from 'react'
import { useNavigate } from 'react-router-dom'

// useBackTo returns a handler for detail-page "back" buttons. When there's a
// previous in-app history entry it goes back to it (navigate(-1)), preserving
// that entry's URL — e.g. a paginated /library/:id?page=3 — so the user lands
// on the page they came from instead of page 1. When the page was opened
// directly (deep link, no history) it falls back to `fallback` so the user
// isn't stranded.
//
// React Router stores a monotonic `idx` in history.state; idx > 0 means there
// is an earlier entry to return to.
export function useBackTo(fallback: string) {
  const navigate = useNavigate()
  return useCallback(() => {
    const idx = (window.history.state?.idx as number | undefined) ?? 0
    if (idx > 0) navigate(-1)
    else navigate(fallback)
  }, [navigate, fallback])
}
