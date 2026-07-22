import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from 'react'
import { api } from '../api'
import type {
  Audiobook, AudiobookChapter,
  CreateAudiobookRequest, UpdateAudiobookRequest,
  CreateChapterRequest,
} from '../api'

interface AudiobooksState {
  audiobooks: Audiobook[]
  isLoading: boolean
  error: string | null
  fetch: (libraryId?: string) => Promise<void>
  getOne: (id: string) => Promise<Audiobook>
  create: (data: CreateAudiobookRequest) => Promise<Audiobook>
  update: (id: string, data: UpdateAudiobookRequest) => Promise<Audiobook>
  remove: (id: string) => Promise<void>
  // Chapters fetched on demand
  fetchChapters: (audiobookId: string) => Promise<AudiobookChapter[]>
  createChapter: (audiobookId: string, data: CreateChapterRequest) => Promise<AudiobookChapter>
  chapterStreamUrl: (audiobookId: string, chapterId: string) => string
}

const AudiobooksContext = createContext<AudiobooksState | null>(null)

export function AudiobooksProvider({ children }: { children: ReactNode }) {
  const [audiobooks, setAudiobooks] = useState<Audiobook[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(async (libraryId?: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await api.listAudiobooks(libraryId ? { library_id: libraryId } : undefined)
      setAudiobooks(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load audiobooks')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const getOne = useCallback((id: string) => api.getAudiobook(id), [])

  const create = useCallback(async (data: CreateAudiobookRequest) => {
    const audiobook = await api.createAudiobook(data)
    setAudiobooks(prev => [...prev, audiobook])
    return audiobook
  }, [])

  const update = useCallback(async (id: string, data: UpdateAudiobookRequest) => {
    const audiobook = await api.updateAudiobook(id, data)
    setAudiobooks(prev => prev.map(a => a.id === id ? audiobook : a))
    return audiobook
  }, [])

  const remove = useCallback(async (id: string) => {
    await api.deleteAudiobook(id)
    setAudiobooks(prev => prev.filter(a => a.id !== id))
  }, [])

  const fetchChapters = useCallback((audiobookId: string) =>
    api.listChapters(audiobookId), [])

  const createChapter = useCallback((audiobookId: string, data: CreateChapterRequest) =>
    api.createChapter(audiobookId, data), [])

  const chapterStreamUrl = useCallback(
    (audiobookId: string, chapterId: string) =>
      api.chapterStreamUrl(audiobookId, chapterId),
    [],
  )

  return (
    <AudiobooksContext.Provider value={{
      audiobooks, isLoading, error,
      fetch, getOne, create, update, remove,
      fetchChapters, createChapter, chapterStreamUrl,
    }}>
      {children}
    </AudiobooksContext.Provider>
  )
}

export function useAudiobooks(): AudiobooksState {
  const ctx = useContext(AudiobooksContext)
  if (!ctx) throw new Error('useAudiobooks must be used within AudiobooksProvider')
  return ctx
}
