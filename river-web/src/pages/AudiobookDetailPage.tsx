import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  RiArrowLeftLine, RiHeadphoneLine, RiPlayFill, RiTimeLine, RiDownloadLine,
  RiBookmarkLine, RiBookmarkFill,
} from 'react-icons/ri'
import { useAudiobooks } from '../context/AudiobooksContext'
import { useAuth } from '../context/AuthContext'
import { useWatchlist } from '../context/WatchlistContext'
import type { Audiobook, AudiobookChapter } from '../api'
import { imageUrl } from '../util/imageUrl'
import { api } from '../api'
import { AdminMediaMenu } from '../components/AdminMediaMenu'
import { MetadataModal } from '../components/MetadataModal'
import { MediaDetailsModal } from '../components/MediaDetailsModal'
import { SimilarCarousel } from '../components/SimilarCarousel'
import { useBackTo } from '../hooks/useBackTo'
import styles from './AudiobookDetailPage.module.css'

export function AudiobookDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { getOne, fetchChapters } = useAudiobooks()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const { isInWatchlist, toggle } = useWatchlist()

  const [book, setBook] = useState<Audiobook | null>(null)
  const goBack = useBackTo(book ? `/library/${book.library_id}` : '/')
  const [chapters, setChapters] = useState<AudiobookChapter[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)

  useEffect(() => {
    if (!id) return
    setIsLoading(true)
    setError(null)
    Promise.all([getOne(id), fetchChapters(id)])
      .then(([b, chs]) => {
        setBook(b)
        setChapters([...chs].sort((a, b) => a.number - b.number))
      })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load audiobook'))
      .finally(() => setIsLoading(false))
  }, [id, getOne, fetchChapters])

  if (isLoading) return <LoadingSkeleton />

  if (error) {
    return (
      <div className={styles.errorPage}>
        <p className="body-md" style={{ color: 'var(--color-error)' }}>{error}</p>
      </div>
    )
  }

  if (!book) return null

  const firstChapter = chapters[0]

  return (
    <div className={styles.page}>
      {/* Hero */}
      <div className={styles.hero}>
        {book.cover_path ? (
          <img src={imageUrl(book.cover_path, 'backdrop')} alt="" className={styles.heroImage} loading="eager" />
        ) : (
          <div className={styles.heroFallback}>
            <RiHeadphoneLine className={styles.heroFallbackIcon} />
          </div>
        )}
        <div className={styles.heroGradient} />
        <button onClick={goBack} className={`btn btn-icon ${styles.backBtn}`} aria-label="Go back">
          <RiArrowLeftLine size={20} />
        </button>
      </div>

      {/* Content row */}
      <div className={`container ${styles.content}`}>
        {/* Cover */}
        <div className={`card card-portrait ${styles.cover}`}>
          {book.cover_path ? (
            <img src={imageUrl(book.cover_path)} alt={book.title} loading="eager" />
          ) : (
            <div className={styles.coverFallback}>
              <RiHeadphoneLine size={48} />
            </div>
          )}
        </div>

        {/* Metadata */}
        <div className={styles.meta}>
          <div className={styles.titleRow}>
            <h1 className={`headline-lg ${styles.title}`}>{book.title}</h1>
            {isAdmin && (
              <AdminMediaMenu
                onRefresh={() => api.refreshAudiobookMetadata(id!)}
                onEdit={() => setEditOpen(true)}
                onShowDetails={() => setDetailsOpen(true)}
              />
            )}
          </div>

          {book.author && (
            <p className={`label-md ${styles.author}`}>{book.author}</p>
          )}

          <div className={styles.attrs}>
            {book.year > 0 && (
              <span className="label-md" style={{ color: 'var(--color-on-surface-variant)' }}>
                {book.year}
              </span>
            )}
            {book.duration > 0 && (
              <span className={styles.attrItem}>
                <RiTimeLine size={14} />
                <span className="label-md">{formatDuration(book.duration)}</span>
              </span>
            )}
            {chapters.length > 0 && (
              <span className="label-md" style={{ color: 'var(--color-on-surface-variant)' }}>
                {chapters.length} chapter{chapters.length !== 1 ? 's' : ''}
              </span>
            )}
          </div>

          {book.genre && (
            <div className={styles.genres}>
              <span className="badge badge-primary">{book.genre}</span>
            </div>
          )}

          {book.narrator && (
            <p className={`label-sm ${styles.narrator}`}>
              <span style={{ color: 'var(--color-on-surface-variant)' }}>Narrated by </span>
              {book.narrator}
            </p>
          )}

          {book.description && (
            <p className={`body-md ${styles.description}`}>{book.description}</p>
          )}

          <div className={styles.mediaActions}>
            {firstChapter?.file_path && (
              <Link
                to={`/audiobook/${id}/listen`}
                className={`btn btn-primary ${styles.playBtn}`}
              >
                <RiPlayFill size={18} />
                Play
              </Link>
            )}
            <button
              className={`btn btn-icon ${isInWatchlist('audiobook', id!) ? styles.watchlistBtnActive : ''}`}
              onClick={() => toggle('audiobook', id!)}
              title={isInWatchlist('audiobook', id!) ? 'Remove from watchlist' : 'Add to watchlist'}
              aria-label={isInWatchlist('audiobook', id!) ? 'Remove from watchlist' : 'Add to watchlist'}
            >
              {isInWatchlist('audiobook', id!) ? <RiBookmarkFill size={18} /> : <RiBookmarkLine size={18} />}
            </button>
          </div>
        </div>
      </div>

      {/* Chapters */}
      {chapters.length > 0 && (
        <div className={`container ${styles.chaptersSection}`}>
          <h2 className={`headline-md ${styles.chaptersTitle}`}>
            {chapters.length === 1 ? '1 Chapter' : `${chapters.length} Chapters`}
          </h2>
          <div className={styles.chapterList}>
            {chapters.map(ch => (
              <ChapterRow key={ch.id} audiobookId={id!} chapter={ch} />
            ))}
          </div>
        </div>
      )}

      {book && <SimilarCarousel sourceId={book.id} type="audiobook" />}

      {editOpen && book && (
        <MetadataModal
          type="audiobook"
          item={book}
          onSave={updated => setBook(updated)}
          onClose={() => setEditOpen(false)}
        />
      )}

      {detailsOpen && book && (
        <MediaDetailsModal
          title="Audiobook details"
          item={book as unknown as Record<string, unknown>}
          primaryKeys={['id', 'title', 'author', 'year']}
          onClose={() => setDetailsOpen(false)}
        />
      )}
    </div>
  )
}

function ChapterRow({ audiobookId, chapter }: { audiobookId: string; chapter: AudiobookChapter }) {
  const { chapterStreamUrl } = useAudiobooks()
  const listenTo = `/audiobook/${audiobookId}/listen?chapter=${chapter.id}`
  const downloadUrl = chapterStreamUrl(audiobookId, chapter.id)

  return (
    <div className={styles.chapter}>
      <span className={`label-md ${styles.chNum}`}>{String(chapter.number).padStart(2, '0')}</span>

      <div className={styles.chInfo}>
        <span className="label-md">{chapter.title || `Chapter ${chapter.number}`}</span>
      </div>

      {chapter.duration > 0 && (
        <div className={styles.chMeta}>
          <span className={styles.chMetaItem}>
            <RiTimeLine size={12} />
            <span className="label-sm">{formatDuration(chapter.duration)}</span>
          </span>
        </div>
      )}

      <div className={styles.chActions}>
        {chapter.file_path ? (
          <Link to={listenTo} className={`btn btn-icon ${styles.chPlay}`} aria-label="Play chapter">
            <RiPlayFill size={16} />
          </Link>
        ) : (
          <button className={`btn btn-icon ${styles.chPlay}`} disabled aria-label="Not available">
            <RiPlayFill size={16} />
          </button>
        )}
        {chapter.file_path && (
          <a
            href={downloadUrl}
            download
            className={`btn btn-icon ${styles.chDownload}`}
            aria-label="Download chapter"
            title="Download"
          >
            <RiDownloadLine size={16} />
          </a>
        )}
      </div>
    </div>
  )
}

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function LoadingSkeleton() {
  return (
    <div className={styles.page}>
      <div className={`${styles.hero} ${styles.skeleton} ${styles.skeletonHero}`} />
      <div className={`container ${styles.content}`}>
        <div className={`card card-portrait ${styles.cover} ${styles.skeleton}`} />
        <div className={styles.meta}>
          <div className={`${styles.skeleton} ${styles.skeletonTitle}`} />
          <div className={`${styles.skeleton} ${styles.skeletonLine}`} />
          <div className={`${styles.skeleton} ${styles.skeletonLine} ${styles.skeletonLineShort}`} />
        </div>
      </div>
    </div>
  )
}
