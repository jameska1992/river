import { useCallback, useEffect, useState } from 'react'

export interface AsyncState<T> {
  data: T | null
  loading: boolean
  error: string | null
  reload: () => void
}

// useAsync runs an async function and tracks {data, loading, error}, with a
// reload() to re-run on demand. Re-runs when `deps` change; ignores results
// from a stale run if deps change (or the component unmounts) mid-flight.
export function useAsync<T>(fn: () => Promise<T>, deps: unknown[]): AsyncState<T> {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [nonce, setNonce] = useState(0)

  const reload = useCallback(() => setNonce(n => n + 1), [])

  useEffect(() => {
    let active = true
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reset async state before (re)fetching when deps change / on reload
    setLoading(true)
    setError(null)
    fn()
      .then(res => { if (active) setData(res) })
      .catch(err => { if (active) setError(err instanceof Error ? err.message : 'Something went wrong') })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
    // fn is intentionally not a dep — callers pass an inline closure; deps
    // control when we re-run. nonce drives reload().
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce])

  return { data, loading, error, reload }
}
