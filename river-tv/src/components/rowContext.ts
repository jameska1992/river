import { createContext, useContext } from 'react'

interface RowCtx {
  ensureVisible: () => void
}

export const RowCtx = createContext<RowCtx | null>(null)

/** Cards inside a Row call this to scroll the entire section (title +
 *  cards + a bit of margin) into view when they gain focus. */
export function useRowEnsureVisible() {
  return useContext(RowCtx)?.ensureVisible
}
