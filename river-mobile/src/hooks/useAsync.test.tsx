import { describe, it, expect } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { useAsync } from './useAsync'

describe('useAsync', () => {
  it('starts loading, then resolves with data', async () => {
    const { result } = renderHook(() => useAsync(() => Promise.resolve(42), []))
    expect(result.current.loading).toBe(true)
    expect(result.current.data).toBeNull()

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.data).toBe(42)
    expect(result.current.error).toBeNull()
  })

  it('captures a rejection as an error message', async () => {
    const { result } = renderHook(() => useAsync(() => Promise.reject(new Error('boom')), []))
    await waitFor(() => expect(result.current.error).toBe('boom'))
    expect(result.current.data).toBeNull()
    expect(result.current.loading).toBe(false)
  })

  it('reload() re-runs the async fn', async () => {
    let n = 0
    const { result } = renderHook(() => useAsync(() => Promise.resolve(++n), []))
    await waitFor(() => expect(result.current.data).toBe(1))

    act(() => result.current.reload())
    await waitFor(() => expect(result.current.data).toBe(2))
  })

  it('ignores a stale result after deps change', async () => {
    let resolveFirst: (v: string) => void = () => {}
    const first = new Promise<string>(r => { resolveFirst = r })

    const { result, rerender } = renderHook(
      ({ id }: { id: number }) => useAsync(() => (id === 1 ? first : Promise.resolve('second')), [id]),
      { initialProps: { id: 1 } },
    )

    // Switch deps before the first run resolves.
    rerender({ id: 2 })
    await waitFor(() => expect(result.current.data).toBe('second'))

    // Late resolution of the stale run must not clobber the current data.
    act(() => resolveFirst('first'))
    await waitFor(() => expect(result.current.data).toBe('second'))
  })
})
