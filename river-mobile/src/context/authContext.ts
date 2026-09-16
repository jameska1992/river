import { createContext, useContext } from 'react'
import type { User } from '../api'

// Single-account auth for the mobile MVP. The API client already supports a
// multi-account registry (ported from river-tv); wiring a mobile account
// picker on top is a follow-up (see the mobile epic).
export interface AuthState {
  user: User | null
  isLoading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

export const AuthContext = createContext<AuthState | null>(null)

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
