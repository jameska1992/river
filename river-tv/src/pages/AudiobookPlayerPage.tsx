import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  api,
  type Audiobook,
  type AudiobookChapter,
} from '../api'
import { PlayerScreen, type UpNext } from '../components/PlayerScreen'

export default function AudiobookPlayerPage() {
  const { audiobookId = '', chapterId = '' } = useParams<{
    audiobookId: string
    chapterId: string
  }>()
  const navigate = useNavigate()
  const location = useLocation()
  // Same flag the EpisodePlayerPage uses — set by in-player skip-next /
  // skip-prev / up-next so the new chapter opens at 0 instead of
  // resuming the saved position from a previous listen.
  const startFromBeginning = (location.state as { fresh?: boolean } | null)?.fresh === true

  const [book, setBook] = useState<Audiobook | null>(null)
  const [chapters, setChapters] = useState<AudiobookChapter[]>([])

  useEffect(() => {
    if (!audiobookId) return
    let alive = true
    Promise.all([
      api.getAudiobook(audiobookId),
      api.listChapters(audiobookId),
    ])
      .then(([b, ch]) => {
        if (!alive) return
        setBook(b)
        setChapters([...ch].sort((a, b) => a.number - b.number))
      })
      .catch(() => {})
    return () => { alive = false }
  }, [audiobookId])

  const current = useMemo(
    () => chapters.find(c => c.id === chapterId),
    [chapters, chapterId],
  )

  const nextCh = useMemo(
    () => current ? chapters.find(c => c.number > current.number && c.file_path) : undefined,
    [current, chapters],
  )
  const prevCh = useMemo(() => {
    if (!current) return undefined
    let pick: AudiobookChapter | undefined
    for (const c of chapters) {
      if (c.number < current.number && c.file_path) pick = c
    }
    return pick
  }, [current, chapters])

  const nextUrl = nextCh
    ? `/audiobooks/${audiobookId}/chapters/${nextCh.id}/listen`
    : null
  const prevUrl = prevCh
    ? `/audiobooks/${audiobookId}/chapters/${prevCh.id}/listen`
    : null

  const upNext: UpNext | undefined = useMemo(() => {
    if (!current || !nextCh || !nextUrl) return undefined
    return {
      title: nextCh.title || `Chapter ${nextCh.number}`,
      subtitle: book?.title,
      posterUrl: book?.cover_path || undefined,
      onPlay: () => navigate(nextUrl, { state: { fresh: true } }),
    }
  }, [current, nextCh, nextUrl, book, navigate])

  const onNext = nextUrl ? () => navigate(nextUrl, { state: { fresh: true } }) : undefined
  const onPrev = prevUrl ? () => navigate(prevUrl, { state: { fresh: true } }) : undefined

  if (!audiobookId || !chapterId) return null

  const chapterLabel = current ? `Chapter ${current.number}${current.title ? ` · ${current.title}` : ''}` : ''

  return (
    <PlayerScreen
      streamUrl={api.chapterStreamUrl(audiobookId, chapterId)}
      title={book?.title ?? ''}
      subtitle={chapterLabel || undefined}
      progressKind="chapter"
      progressId={chapterId}
      audioOnly
      coverUrl={book?.cover_path || undefined}
      upNext={upNext}
      onPrev={onPrev}
      onNext={onNext}
      startFromBeginning={startFromBeginning}
      onExit={() => navigate(`/audiobooks/${audiobookId}`, { replace: true })}
    />
  )
}
