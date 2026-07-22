import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from 'react'
import { api } from '../api'
import type {
  Artist, Album, Track,
  CreateArtistRequest, UpdateArtistRequest,
  CreateAlbumRequest, UpdateAlbumRequest,
  CreateTrackRequest,
} from '../api'

interface MusicState {
  artists: Artist[]
  albums: Album[]
  isLoading: boolean
  error: string | null
  fetchArtists: (libraryId?: string) => Promise<void>
  fetchAlbums: (libraryId?: string) => Promise<void>
  getArtist: (id: string) => Promise<Artist>
  createArtist: (data: CreateArtistRequest) => Promise<Artist>
  updateArtist: (id: string, data: UpdateArtistRequest) => Promise<Artist>
  removeArtist: (id: string) => Promise<void>
  getAlbum: (id: string) => Promise<Album>
  createAlbum: (data: CreateAlbumRequest) => Promise<Album>
  updateAlbum: (id: string, data: UpdateAlbumRequest) => Promise<Album>
  removeAlbum: (id: string) => Promise<void>
  // Tracks fetched on demand
  fetchTracks: (albumId: string) => Promise<Track[]>
  createTrack: (data: CreateTrackRequest) => Promise<Track>
  removeTrack: (id: string) => Promise<void>
  trackStreamUrl: (id: string) => string
}

const MusicContext = createContext<MusicState | null>(null)

export function MusicProvider({ children }: { children: ReactNode }) {
  const [artists, setArtists] = useState<Artist[]>([])
  const [albums, setAlbums] = useState<Album[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchArtists = useCallback(async (libraryId?: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await api.listArtists(libraryId ? { library_id: libraryId } : undefined)
      setArtists(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load artists')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const fetchAlbums = useCallback(async (libraryId?: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await api.listAlbums(libraryId ? { library_id: libraryId } : undefined)
      setAlbums(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load albums')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const getArtist = useCallback((id: string) => api.getArtist(id), [])

  const createArtist = useCallback(async (data: CreateArtistRequest) => {
    const artist = await api.createArtist(data)
    setArtists(prev => [...prev, artist])
    return artist
  }, [])

  const updateArtist = useCallback(async (id: string, data: UpdateArtistRequest) => {
    const artist = await api.updateArtist(id, data)
    setArtists(prev => prev.map(a => a.id === id ? artist : a))
    return artist
  }, [])

  const removeArtist = useCallback(async (id: string) => {
    await api.deleteArtist(id)
    setArtists(prev => prev.filter(a => a.id !== id))
  }, [])

  const getAlbum = useCallback((id: string) => api.getAlbum(id), [])

  const createAlbum = useCallback(async (data: CreateAlbumRequest) => {
    const album = await api.createAlbum(data)
    setAlbums(prev => [...prev, album])
    return album
  }, [])

  const updateAlbum = useCallback(async (id: string, data: UpdateAlbumRequest) => {
    const album = await api.updateAlbum(id, data)
    setAlbums(prev => prev.map(a => a.id === id ? album : a))
    return album
  }, [])

  const removeAlbum = useCallback(async (id: string) => {
    await api.deleteAlbum(id)
    setAlbums(prev => prev.filter(a => a.id !== id))
  }, [])

  const fetchTracks = useCallback((albumId: string) => api.listAlbumTracks(albumId), [])

  const createTrack = useCallback(async (data: CreateTrackRequest) => {
    const track = await api.createTrack(data)
    return track
  }, [])

  const removeTrack = useCallback(async (id: string) => {
    await api.deleteTrack(id)
  }, [])

  const trackStreamUrl = useCallback((id: string) => api.trackStreamUrl(id), [])

  return (
    <MusicContext.Provider value={{
      artists, albums, isLoading, error,
      fetchArtists, fetchAlbums,
      getArtist, createArtist, updateArtist, removeArtist,
      getAlbum, createAlbum, updateAlbum, removeAlbum,
      fetchTracks, createTrack, removeTrack, trackStreamUrl,
    }}>
      {children}
    </MusicContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components -- context hook colocated with its provider; separating adds no runtime value
export function useMusic(): MusicState {
  const ctx = useContext(MusicContext)
  if (!ctx) throw new Error('useMusic must be used within MusicProvider')
  return ctx
}
