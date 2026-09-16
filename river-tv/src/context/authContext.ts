import { createContext, useContext } from 'react'
import type { SavedAccount, User } from '../api'

export interface AuthState {
  user: User | null
  isLoading: boolean
  // accounts remembered on the currently-selected server (for the picker).
  accounts: SavedAccount[]
  login: (username: string, password: string) => Promise<void>
  // addAccount logs in and remembers the account — same as login; named for
  // the "Add account" flow so call sites read clearly.
  addAccount: (username: string, password: string) => Promise<void>
  // switchAccount signs in as a remembered account with no password.
  switchAccount: (id: string) => Promise<void>
  // removeAccount forgets + revokes a remembered account.
  removeAccount: (id: string) => Promise<void>
  // signOutActive revokes + forgets only the active account.
  signOutActive: () => Promise<void>
  logout: () => Promise<void>
}

export const AuthContext = createContext<AuthState | null>(null)

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
