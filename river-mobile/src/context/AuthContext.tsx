import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { api, type User } from '../api'
import { AuthContext } from './authContext'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  // Only an authenticated session needs an async me() fetch; without a token
  // we're immediately settled.
  const [isLoading, setIsLoading] = useState(() => api.isAuthenticated)

  useEffect(() => {
    if (!api.isAuthenticated) return
    api.me()
      .then(setUser)
      .catch(() => api.clearAuth())
      .finally(() => setIsLoading(false))
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.login(username, password)
    setUser(res.user)
  }, [])

  const logout = useCallback(async () => {
    await api.logout()
    setUser(null)
  }, [])

  return (
    <AuthContext.Provider value={{ user, isLoading, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}
