import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Mock only useNavigate; keep the rest of react-router-dom intact.
const navigate = vi.fn()
vi.mock('react-router-dom', async (orig) => ({
  ...(await orig<typeof import('react-router-dom')>()),
  useNavigate: () => navigate,
}))

import { useBackTo } from './useBackTo'

// Restore the history entry index the hook keys off of.
function setIdx(idx: number | undefined) {
  window.history.replaceState(idx === undefined ? {} : { idx }, '')
}

beforeEach(() => {
  navigate.mockReset()
})
afterEach(() => {
  setIdx(undefined)
})

describe('useBackTo', () => {
  it('goes back one entry when there is in-app history (idx > 0)', () => {
    setIdx(3)
    const { result } = renderHook(() => useBackTo('/library'))
    result.current()
    expect(navigate).toHaveBeenCalledWith(-1)
  })

  it('navigates to the fallback when there is no earlier entry (idx === 0)', () => {
    setIdx(0)
    const { result } = renderHook(() => useBackTo('/library'))
    result.current()
    expect(navigate).toHaveBeenCalledWith('/library')
  })

  it('navigates to the fallback when idx is absent (deep link)', () => {
    setIdx(undefined)
    const { result } = renderHook(() => useBackTo('/home'))
    result.current()
    expect(navigate).toHaveBeenCalledWith('/home')
  })
})
