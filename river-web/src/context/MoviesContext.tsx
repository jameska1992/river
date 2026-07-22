import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from 'react'
import { api } from '../api'
import type { Movie, CreateMovieRequest, UpdateMovieRequest } from '../api'

interface MoviesState {
  movies: Movie[]
  isLoading: boolean
  error: string | null
  fetch: (libraryId?: string) => Promise<void>
  getOne: (id: string) => Promise<Movie>
  create: (data: CreateMovieRequest) => Promise<Movie>
  update: (id: string, data: UpdateMovieRequest) => Promise<Movie>
  remove: (id: string) => Promise<void>
  streamUrl: (id: string) => string
}

const MoviesContext = createContext<MoviesState | null>(null)

export function MoviesProvider({ children }: { children: ReactNode }) {
  const [movies, setMovies] = useState<Movie[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(async (libraryId?: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await api.listMovies(libraryId ? { library_id: libraryId } : undefined)
      setMovies(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load movies')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const getOne = useCallback((id: string) => api.getMovie(id), [])

  const create = useCallback(async (data: CreateMovieRequest) => {
    const movie = await api.createMovie(data)
    setMovies(prev => [...prev, movie])
    return movie
  }, [])

  const update = useCallback(async (id: string, data: UpdateMovieRequest) => {
    const movie = await api.updateMovie(id, data)
    setMovies(prev => prev.map(m => m.id === id ? movie : m))
    return movie
  }, [])

  const remove = useCallback(async (id: string) => {
    await api.deleteMovie(id)
    setMovies(prev => prev.filter(m => m.id !== id))
  }, [])

  const streamUrl = useCallback((id: string) => api.movieStreamUrl(id), [])

  return (
    <MoviesContext.Provider value={{ movies, isLoading, error, fetch, getOne, create, update, remove, streamUrl }}>
      {children}
    </MoviesContext.Provider>
  )
}

export function useMovies(): MoviesState {
  const ctx = useContext(MoviesContext)
  if (!ctx) throw new Error('useMovies must be used within MoviesProvider')
  return ctx
}
