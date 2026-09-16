import {
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from 'react'
import { api, type SavedAccount, type User } from '../api'
import { AuthContext } from './authContext'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  // Only authenticated sessions need an async "me()" fetch; if there's no
  // token we're immediately settled, so start loading=false in that case.
  const [isLoading, setIsLoading] = useState(() => api.isAuthenticated)
  const [accounts, setAccounts] = useState<SavedAccount[]>(() => api.getAccountsForServer())

  // Re-read the per-server account list from the client after any change or
  // a server switch.
  const syncAccounts = useCallback(() => {
    setAccounts(api.getAccountsForServer())
  }, [])

  useEffect(() => {
    if (!api.isAuthenticated) return
    api.me()
      .then(u => {
        setUser(u)
        // Upgrade path: record a pre-existing single-account session so it
        // appears in the picker / Change-account flow.
        api.ensureActiveAccount(u.username)
        syncAccounts()
      })
      .catch(() => api.clearAuth())
      .finally(() => setIsLoading(false))
  }, [syncAccounts])

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.login(username, password)
    setUser(res.user)
    syncAccounts()
  }, [syncAccounts])

  const switchAccount = useCallback(async (id: string) => {
    const u = await api.switchToAccount(id)
    setUser(u)
    syncAccounts()
  }, [syncAccounts])

  const removeAccount = useCallback(async (id: string) => {
    await api.removeAccount(id)
    if (!api.isAuthenticated) setUser(null)
    syncAccounts()
  }, [syncAccounts])

  const signOutActive = useCallback(async () => {
    await api.signOutActive()
    setUser(null)
    syncAccounts()
  }, [syncAccounts])

  return (
    <AuthContext.Provider value={{
      user, isLoading, accounts,
      login, addAccount: login,
      switchAccount, removeAccount, signOutActive,
      logout: signOutActive,
    }}>
      {children}
    </AuthContext.Provider>
  )
}
