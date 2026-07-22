import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from 'react'
import { api } from '../api'
import type { Library, CreateLibraryRequest, UpdateLibraryRequest } from '../api'

interface LibrariesState {
  libraries: Library[]
  isLoading: boolean
  error: string | null
  fetch: () => Promise<void>
  create: (data: CreateLibraryRequest) => Promise<Library>
  update: (id: string, data: UpdateLibraryRequest) => Promise<Library>
  remove: (id: string) => Promise<void>
}

const LibrariesContext = createContext<LibrariesState | null>(null)

export function LibrariesProvider({ children }: { children: ReactNode }) {
  const [libraries, setLibraries] = useState<Library[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await api.listLibraries()
      setLibraries(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load libraries')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const create = useCallback(async (data: CreateLibraryRequest) => {
    const library = await api.createLibrary(data)
    setLibraries(prev => [...prev, library])
    return library
  }, [])

  const update = useCallback(async (id: string, data: UpdateLibraryRequest) => {
    const library = await api.updateLibrary(id, data)
    setLibraries(prev => prev.map(l => l.id === id ? library : l))
    return library
  }, [])

  const remove = useCallback(async (id: string) => {
    await api.deleteLibrary(id)
    setLibraries(prev => prev.filter(l => l.id !== id))
  }, [])

  return (
    <LibrariesContext.Provider value={{ libraries, isLoading, error, fetch, create, update, remove }}>
      {children}
    </LibrariesContext.Provider>
  )
}

export function useLibraries(): LibrariesState {
  const ctx = useContext(LibrariesContext)
  if (!ctx) throw new Error('useLibraries must be used within LibrariesProvider')
  return ctx
}
