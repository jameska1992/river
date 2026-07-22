import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { RiArrowLeftLine, RiFilmLine, RiPlayFill, RiStarLine, RiTimeLine, RiDownloadLine, RiUserLine, RiBookmarkLine, RiBookmarkFill, RiGroupLine, RiAlertFill, RiEyeLine, RiEyeOffLine } from 'react-icons/ri'
import { useMovies } from '../context/MoviesContext'
import { useAuth } from '../context/AuthContext'
import { imageUrl } from '../util/imageUrl'
import { useWatchlist } from '../context/WatchlistContext'
import type { Movie, Credits, WatchProgress } from '../api'
import { api } from '../api'
import { AdminMediaMenu } from '../components/AdminMediaMenu'
import { MetadataModal } from '../components/MetadataModal'
import { IdentifyMovieModal } from '../components/IdentifyMovieModal'
import { MediaDetailsModal } from '../components/MediaDetailsModal'
import { DeleteMediaModal } from '../components/DeleteMediaModal'
import { SimilarCarousel } from '../components/SimilarCarousel'
import { useBackTo } from '../hooks/useBackTo'
import styles from './MovieDetailPage.module.css'

export function MovieDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { getOne } = useMovies()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const { isInWatchlist, toggle } = useWatchlist()
  const navigate = useNavigate()

  const [movie, setMovie] = useState<Movie | null>(null)
  const goBack = useBackTo(movie ? `/library/${movie.library_id}` : '/')
  const [credits, setCredits] = useState<Credits | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [identifyOpen, setIdentifyOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [progress, setProgress] = useState<WatchProgress | null>(null)
  const [watchedSaving, setWatchedSaving] = useState(false)

  const startParty = async () => {
    if (!movie) return
    const room = await api.createWatchParty({ media_type: 'movie', media_id: movie.id })
    navigate(`/movie/${movie.id}/watch?party=${room.id}`)
  }

  useEffect(() => {
    if (!id) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resets loading state before refetching when the route id changes
    setIsLoading(true)
    setError(null)
    Promise.all([getOne(id), api.getMovieCredits(id).catch(() => null)])
      .then(([m, c]) => { setMovie(m); setCredits(c) })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load movie'))
      .finally(() => setIsLoading(false))
  }, [id, getOne])

  useEffect(() => {
    if (!id) return
    api.getProgress('movie', id).then(setProgress).catch(() => setProgress(null))
  }, [id])

  const isWatched = progress?.completed ?? false

  async function toggleWatched() {
    if (!id || watchedSaving) return
    const next = !isWatched
    setWatchedSaving(true)
    // Optimistic: collapse to either a synthetic "completed" record or
    // null. A real refresh-from-server would be nicer but adds a round
    // trip; if the call fails we revert.
    const previous = progress
    setProgress(next ? { ...(progress ?? {
      id: '', user_id: '', media_type: 'movie', media_id: id,
      position: 0, duration: 0, created_at: '', updated_at: '',
    }), completed: true } : null)
    try {
      await api.setProgressCompleted('movie', id, next)
    } catch {
      setProgress(previous)
    } finally {
      setWatchedSaving(false)
    }
  }

  if (isLoading) return <LoadingSkeleton />

  if (error) {
    return (
      <div className={styles.errorPage}>
        <p className="body-md" style={{ color: 'var(--color-error)' }}>{error}</p>
      </div>
    )
  }

  if (!movie) return null

  const runtime = movie.runtime > 0
    ? `${Math.floor(movie.runtime / 60)}h ${movie.runtime % 60}m`
    : null

  // Wall-clock estimate for when a "start now" play would finish. Computed
  // once on render rather than ticking live — by the time a stale value
  // would matter (more than a few minutes idle) the user has either
  // started the movie or moved on. movie.runtime is in minutes.
  const endsAt = movie.runtime > 0
    // eslint-disable-next-line react-hooks/purity -- intentional one-shot wall-clock read for a display-only "ends at" estimate (see comment above)
    ? new Date(Date.now() + movie.runtime * 60_000)
        .toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
    : null

  return (
    <div className={styles.page}>
        {/* Backdrop */}
        <div className={styles.hero}>
          {movie.backdrop_path ? (
            <img
              src={imageUrl(movie.backdrop_path, 'backdrop')}
              alt=""
              className={styles.heroImage}
              loading="eager"
            />
          ) : (
            <div className={styles.heroFallback}>
              <RiFilmLine className={styles.heroFallbackIcon} />
            </div>
          )}
          <div className={styles.heroGradient} />
          <button onClick={goBack} className={`btn btn-icon ${styles.backBtn}`} aria-label="Go back">
            <RiArrowLeftLine size={20} />
          </button>
        </div>

        {/* Content row */}
        <div className={`container ${styles.content}`}>
          {/* Poster */}
          <div className={`card card-portrait ${styles.poster}`}>
            {movie.poster_path ? (
              <img src={imageUrl(movie.poster_path)} alt={movie.title} loading="eager" />
            ) : (
              <div className={styles.posterFallback}>
                <RiFilmLine size={48} />
              </div>
            )}
          </div>

          {/* Metadata */}
          <div className={styles.meta}>
            <div className={styles.titleRow}>
              <h1 className={`headline-lg ${styles.title}`}>{movie.title}</h1>
              {isAdmin && (
                <AdminMediaMenu
                  onRefresh={() => api.refreshMovieMetadata(id!)}
                  onEdit={() => setEditOpen(true)}
                  onIdentify={() => setIdentifyOpen(true)}
                  onShowDetails={() => setDetailsOpen(true)}
                  onDelete={() => setDeleteOpen(true)}
                  identifyLabel="Identify movie"
                />
              )}
            </div>

            {movie.original_title && movie.original_title !== movie.title && (
              <p className={`label-sm ${styles.originalTitle}`}>{movie.original_title}</p>
            )}

            <div className={styles.attrs}>
              {movie.year > 0 && (
                <span className="label-md" style={{ color: 'var(--color-on-surface-variant)' }}>
                  {movie.year}
                </span>
              )}
              {runtime && (
                <span className={styles.attrItem}>
                  <RiTimeLine size={14} />
                  <span className="label-md">{runtime}</span>
                </span>
              )}
              {endsAt && (
                <span className={styles.attrItem}>
                  <span className="label-md">Ends at {endsAt}</span>
                </span>
              )}
              {movie.rating > 0 && (
                <span className={styles.attrItem}>
                  <RiStarLine size={14} style={{ color: 'var(--color-tertiary)' }} />
                  <span className="label-md" style={{ color: 'var(--color-tertiary)' }}>
                    {movie.rating.toFixed(1)}
                  </span>
                </span>
              )}
            </div>

            {movie.genres.length > 0 && (
              <div className={styles.genres}>
                {movie.genres.map(g => (
                  <Link key={g} to={`/search?genre=${encodeURIComponent(g)}`} className="badge badge-primary">{g}</Link>
                ))}
              </div>
            )}

            <div className={styles.contentBody}>
              <div className={styles.contentLeft}>
                {movie.description && (
                  <p className={`body-md ${styles.description}`}>{movie.description}</p>
                )}
                <div className={styles.mediaActions}>
                  <Link to={`/movie/${id}/watch`} className={`btn btn-primary ${styles.playBtn}`}>
                    <RiPlayFill size={18} />
                    Play
                    {!movie.file_path && (
                      <span
                        className={styles.notReadyInline}
                        title="Not transcoded yet — playing from original source"
                        aria-label="Not transcoded yet — playing from original source"
                      >
                        <RiAlertFill size={16} />
                      </span>
                    )}
                  </Link>
                  <button className="btn btn-icon" onClick={startParty} title="Start Watch Party" aria-label="Start Watch Party">
                    <RiGroupLine size={18} />
                  </button>
                  <button
                    className={`btn btn-icon ${isInWatchlist('movie', id!) ? styles.watchlistBtnActive : ''}`}
                    onClick={() => toggle('movie', id!)}
                    title={isInWatchlist('movie', id!) ? 'Remove from watchlist' : 'Add to watchlist'}
                    aria-label={isInWatchlist('movie', id!) ? 'Remove from watchlist' : 'Add to watchlist'}
                  >
                    {isInWatchlist('movie', id!) ? <RiBookmarkFill size={18} /> : <RiBookmarkLine size={18} />}
                  </button>
                  <button
                    className={`btn btn-icon ${isWatched ? styles.watchedBtnActive : ''}`}
                    onClick={toggleWatched}
                    disabled={watchedSaving}
                    title={isWatched ? 'Mark as unwatched' : 'Mark as watched'}
                    aria-label={isWatched ? 'Mark as unwatched' : 'Mark as watched'}
                    aria-pressed={isWatched}
                  >
                    {isWatched ? <RiEyeOffLine size={18} /> : <RiEyeLine size={18} />}
                  </button>
                  {movie.file_path && (
                    <a
                      href={api.movieDownloadUrl(id!)}
                      className="btn btn-icon"
                      title="Download"
                      aria-label="Download movie"
                    >
                      <RiDownloadLine size={18} />
                    </a>
                  )}
                </div>
                {credits && (credits.cast.length > 0 || credits.crew.length > 0) && (
                  <CreditsSection credits={credits} />
                )}
              </div>
              {movie.trailer_url && (
                <div className={styles.trailerEmbed}>
                  <iframe
                    src={`${movie.trailer_url}?rel=0`}
                    title="Trailer"
                    allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
                    allowFullScreen
                  />
                </div>
              )}
            </div>
          </div>

        </div>

        {movie && <SimilarCarousel sourceId={movie.id} type="movie" />}

      {editOpen && movie && (
        <MetadataModal
          type="movie"
          item={movie}
          onSave={updated => setMovie(updated)}
          onClose={() => setEditOpen(false)}
        />
      )}

      {identifyOpen && movie && (
        <IdentifyMovieModal
          movie={movie}
          onClose={() => setIdentifyOpen(false)}
          onSubmitted={() => {
            // Re-fetch the movie shortly after so the new title/year and
            // post-enrich poster/description show up without a page reload.
            setTimeout(() => {
              api.getMovie(movie.id).then(setMovie).catch(() => {})
            }, 1500)
          }}
        />
      )}

      {detailsOpen && movie && (
        <MediaDetailsModal
          title="Movie details"
          item={movie as unknown as Record<string, unknown>}
          primaryKeys={['id', 'title', 'year', 'file_path', 'source_path']}
          onClose={() => setDetailsOpen(false)}
        />
      )}

      {deleteOpen && movie && (
        <DeleteMediaModal
          mediaLabel="movie"
          title={movie.year > 0 ? `${movie.title} (${movie.year})` : movie.title}
          onConfirm={async deleteFiles => {
            await api.deleteMovie(movie.id, deleteFiles)
            // Back to the library so the user isn't stranded on a 404
            // when they navigate to the (now-deleted) movie again.
            navigate(`/library/${movie.library_id}`)
          }}
          onClose={() => setDeleteOpen(false)}
        />
      )}
    </div>
  )
}

function CreditsSection({ credits }: { credits: Credits }) {
  const directors = credits.crew.filter(c => c.job === 'Director')
  const writers = credits.crew.filter(c => c.department === 'Writing')

  return (
    <div className={styles.creditsSection}>
      {directors.length > 0 && (
        <dl className={styles.credits}>
          <dt className="label-sm">Director</dt>
          <dd className="body-md">{directors.map(c => c.name).join(', ')}</dd>
        </dl>
      )}
      {writers.length > 0 && (
        <dl className={styles.credits}>
          <dt className="label-sm">Writers</dt>
          <dd className="body-md">{writers.map(c => c.name).join(', ')}</dd>
        </dl>
      )}
      {credits.cast.length > 0 && (
        <div className={styles.castGrid}>
          {credits.cast.map(c => (
            <Link key={c.person_id} to={`/person/${c.person_id}`} className={styles.castCard}>
              <div className={styles.castPhoto}>
                {c.profile_path ? (
                  <img src={imageUrl(c.profile_path)} alt={c.name} />
                ) : (
                  <RiUserLine size={24} />
                )}
              </div>
              <span className={`label-sm ${styles.castName}`}>{c.name}</span>
              {c.character && <span className={`label-sm ${styles.castCharacter}`}>{c.character}</span>}
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}

function LoadingSkeleton() {
  return (
    <div className={styles.page}>
      <div className={`${styles.hero} ${styles.skeletonHero}`} />
      <div className={`container ${styles.content}`}>
        <div className={`card card-portrait ${styles.poster} ${styles.skeleton}`} />
        <div className={styles.meta}>
          <div className={`${styles.skeleton} ${styles.skeletonTitle}`} />
          <div className={`${styles.skeleton} ${styles.skeletonLine}`} />
          <div className={`${styles.skeleton} ${styles.skeletonLine} ${styles.skeletonLineShort}`} />
        </div>
      </div>
    </div>
  )
}
