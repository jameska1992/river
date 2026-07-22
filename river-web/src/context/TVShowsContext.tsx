import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from 'react'
import { api } from '../api'
import type {
  TVShow, Season, Episode,
  CreateTVShowRequest, UpdateTVShowRequest,
  CreateSeasonRequest, CreateEpisodeRequest,
} from '../api'

interface TVShowsState {
  shows: TVShow[]
  isLoading: boolean
  error: string | null
  fetch: (libraryId?: string) => Promise<void>
  getOne: (id: string) => Promise<TVShow>
  create: (data: CreateTVShowRequest) => Promise<TVShow>
  update: (id: string, data: UpdateTVShowRequest) => Promise<TVShow>
  remove: (id: string) => Promise<void>
  // Seasons
  fetchSeasons: (showId: string) => Promise<Season[]>
  createSeason: (showId: string, data: CreateSeasonRequest) => Promise<Season>
  // Episodes
  fetchEpisodes: (showId: string, seasonId: string) => Promise<Episode[]>
  createEpisode: (showId: string, seasonId: string, data: CreateEpisodeRequest) => Promise<Episode>
  episodeStreamUrl: (showId: string, seasonId: string, episodeId: string) => string
}

const TVShowsContext = createContext<TVShowsState | null>(null)

export function TVShowsProvider({ children }: { children: ReactNode }) {
  const [shows, setShows] = useState<TVShow[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(async (libraryId?: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await api.listTVShows(libraryId ? { library_id: libraryId } : undefined)
      setShows(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load TV shows')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const getOne = useCallback((id: string) => api.getTVShow(id), [])

  const create = useCallback(async (data: CreateTVShowRequest) => {
    const show = await api.createTVShow(data)
    setShows(prev => [...prev, show])
    return show
  }, [])

  const update = useCallback(async (id: string, data: UpdateTVShowRequest) => {
    const show = await api.updateTVShow(id, data)
    setShows(prev => prev.map(s => s.id === id ? show : s))
    return show
  }, [])

  const remove = useCallback(async (id: string) => {
    await api.deleteTVShow(id)
    setShows(prev => prev.filter(s => s.id !== id))
  }, [])

  const fetchSeasons = useCallback((showId: string) => api.listSeasons(showId), [])

  const createSeason = useCallback((showId: string, data: CreateSeasonRequest) =>
    api.createSeason(showId, data), [])

  const fetchEpisodes = useCallback((showId: string, seasonId: string) =>
    api.listEpisodes(showId, seasonId), [])

  const createEpisode = useCallback((showId: string, seasonId: string, data: CreateEpisodeRequest) =>
    api.createEpisode(showId, seasonId, data), [])

  const episodeStreamUrl = useCallback(
    (showId: string, seasonId: string, episodeId: string) =>
      api.episodeStreamUrl(showId, seasonId, episodeId),
    [],
  )

  return (
    <TVShowsContext.Provider value={{
      shows, isLoading, error,
      fetch, getOne, create, update, remove,
      fetchSeasons, createSeason,
      fetchEpisodes, createEpisode, episodeStreamUrl,
    }}>
      {children}
    </TVShowsContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components -- context hook colocated with its provider; separating adds no runtime value
export function useTVShows(): TVShowsState {
  const ctx = useContext(TVShowsContext)
  if (!ctx) throw new Error('useTVShows must be used within TVShowsProvider')
  return ctx
}
