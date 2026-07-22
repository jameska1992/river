import { useEffect, useState, useCallback } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import {
  RiArrowLeftLine, RiTv2Line, RiStarLine, RiPlayFill,
  RiArrowDownSLine, RiTimeLine, RiDownloadLine, RiUserLine,
  RiBookmarkLine, RiBookmarkFill, RiGroupLine, RiAlertFill, RiEditLine,
  RiEyeLine, RiEyeOffLine, RiDeleteBin6Line,
} from 'react-icons/ri'
import { useTVShows } from '../context/TVShowsContext'
import { useAuth } from '../context/AuthContext'
import { imageUrl } from '../util/imageUrl'
import { useWatchlist } from '../context/WatchlistContext'
import type { TVShow, Season, Episode, Credits } from '../api'
import { api } from '../api'
import { AdminMediaMenu } from '../components/AdminMediaMenu'
import { MetadataModal } from '../components/MetadataModal'
import { IdentifyTVShowModal } from '../components/IdentifyTVShowModal'
import { EpisodeMetadataModal } from '../components/EpisodeMetadataModal'
import { SeasonMetadataModal } from '../components/SeasonMetadataModal'
import { MediaDetailsModal } from '../components/MediaDetailsModal'
import { SimilarCarousel } from '../components/SimilarCarousel'
import { DeleteMediaModal } from '../components/DeleteMediaModal'
import { useBackTo } from '../hooks/useBackTo'
import styles from './TVShowDetailPage.module.css'

export function TVShowDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { getOne, fetchSeasons, fetchEpisodes } = useTVShows()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const { isInWatchlist, toggle } = useWatchlist()

  const navigate = useNavigate()
  const [show, setShow] = useState<TVShow | null>(null)
  const goBack = useBackTo(show ? `/library/${show.library_id}` : '/')
  const [credits, setCredits] = useState<Credits | null>(null)
  const [seasons, setSeasons] = useState<Season[]>([])
  const [episodes, setEpisodes] = useState<Record<string, Episode[]>>({})
  const [expandedSeason, setExpandedSeason] = useState<string | null>(null)
  const [loadingSeasons, setLoadingSeasons] = useState<Record<string, boolean>>({})
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [identifyOpen, setIdentifyOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  // editingSeason / editingEpisode hold the row currently being edited; we
  // pass them to the modal as a prop rather than threading callbacks down.
  // When set, the corresponding modal renders.
  const [editingSeason, setEditingSeason] = useState<Season | null>(null)
  const [editingEpisode, setEditingEpisode] = useState<{ seasonId: string; episode: Episode } | null>(null)
  const [deletingEpisode, setDeletingEpisode] = useState<{ seasonId: string; episode: Episode } | null>(null)
  const [playNextLoading, setPlayNextLoading] = useState(false)
  // Watched state for the show is the "all episodes completed" summary;
  // null until first fetch resolves so the button can render its loading
  // state without flashing the wrong label.
  const [watchedState, setWatchedState] = useState<{ total: number; completed: number } | null>(null)
  const [watchedSaving, setWatchedSaving] = useState(false)

  const playNext = async () => {
    if (!id) return
    setPlayNextLoading(true)
    try {
      const { season_id, episode_id } = await api.getNextEpisode(id)
      navigate(`/show/${id}/season/${season_id}/episode/${episode_id}/watch`)
    } finally {
      setPlayNextLoading(false)
    }
  }

  useEffect(() => {
    if (!id) return
    api.getShowWatchState(id)
      .then(s => setWatchedState({ total: s.total, completed: s.completed }))
      .catch(() => setWatchedState(null))
  }, [id])

  const isWatched = !!watchedState && watchedState.total > 0 && watchedState.completed >= watchedState.total

  async function toggleShowWatched() {
    if (!id || watchedSaving || !watchedState) return
    const next = !isWatched
    const previous = watchedState
    setWatchedSaving(true)
    // Optimistic: marking watched flips the count to total; unmarking
    // resets completed to 0. A real refresh-from-server would be nicer
    // but adds a round trip; we revert on failure.
    setWatchedState({
      total: previous.total,
      completed: next ? previous.total : 0,
    })
    try {
      await api.setShowCompleted(id, next)
    } catch {
      setWatchedState(previous)
    } finally {
      setWatchedSaving(false)
    }
  }

  useEffect(() => {
    if (!id) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- resets loading state before refetching when the route id changes
    setIsLoading(true)
    Promise.all([getOne(id), fetchSeasons(id), api.getTVShowCredits(id).catch(() => null)])
      .then(([s, seas, c]) => {
        setShow(s)
        setCredits(c)
        const sorted = [...seas].sort((a, b) => a.number - b.number)
        setSeasons(sorted)
        if (sorted.length > 0) setExpandedSeason(sorted[0].id)
      })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load show'))
      .finally(() => setIsLoading(false))
  }, [id, getOne, fetchSeasons])

  const loadEpisodes = useCallback(async (seasonId: string) => {
    if (!id || episodes[seasonId] || loadingSeasons[seasonId]) return
    setLoadingSeasons(prev => ({ ...prev, [seasonId]: true }))
    try {
      const eps = await fetchEpisodes(id, seasonId)
      setEpisodes(prev => ({ ...prev, [seasonId]: eps.sort((a, b) => a.number - b.number) }))
    } finally {
      setLoadingSeasons(prev => ({ ...prev, [seasonId]: false }))
    }
  }, [id, episodes, loadingSeasons, fetchEpisodes])

  const toggleSeason = useCallback((seasonId: string) => {
    setExpandedSeason(prev => prev === seasonId ? null : seasonId)
    loadEpisodes(seasonId)
  }, [loadEpisodes])

  // Load episodes for initially expanded season
  useEffect(() => {
    if (expandedSeason && !episodes[expandedSeason]) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- lazily fetches episodes for the expanded season; guarded to run once per season
      loadEpisodes(expandedSeason)
    }
  }, [expandedSeason, episodes, loadEpisodes])

  if (isLoading) return <LoadingSkeleton />
  if (error) {
    return (
      <div className={styles.errorPage}>
        <p className="body-md" style={{ color: 'var(--color-error)' }}>{error}</p>
      </div>
    )
  }
  if (!show) return null

  return (
    <div className={styles.page}>
      {/* Hero */}
      <div className={styles.hero}>
        {show.backdrop_path ? (
          <img src={imageUrl(show.backdrop_path, 'backdrop')} alt="" className={styles.heroImage} loading="eager" />
        ) : (
          <div className={styles.heroFallback}>
            <RiTv2Line className={styles.heroFallbackIcon} />
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
          {show.poster_path ? (
            <img src={imageUrl(show.poster_path)} alt={show.title} loading="eager" />
          ) : (
            <div className={styles.posterFallback}><RiTv2Line size={48} /></div>
          )}
        </div>

        {/* Meta */}
        <div className={styles.meta}>
          <div className={styles.titleRow}>
            <h1 className={`headline-lg ${styles.title}`}>{show.title}</h1>
            {isAdmin && (
              <AdminMediaMenu
                onRefresh={() => api.refreshTVShowMetadata(id!)}
                onEdit={() => setEditOpen(true)}
                onIdentify={() => setIdentifyOpen(true)}
                onShowDetails={() => setDetailsOpen(true)}
                onDelete={() => setDeleteOpen(true)}
                identifyLabel="Identify series"
              />
            )}
          </div>

          {show.original_title && show.original_title !== show.title && (
            <p className={`label-sm ${styles.originalTitle}`}>{show.original_title}</p>
          )}

          <div className={styles.attrs}>
            {show.year > 0 && (
              <span className="label-md" style={{ color: 'var(--color-on-surface-variant)' }}>
                {show.year}
              </span>
            )}
            {show.status && (
              <span className={`badge ${statusBadgeClass(show.status)}`}>{show.status}</span>
            )}
            {show.rating > 0 && (
              <span className={styles.attrItem}>
                <RiStarLine size={14} style={{ color: 'var(--color-tertiary)' }} />
                <span className="label-md" style={{ color: 'var(--color-tertiary)' }}>
                  {show.rating.toFixed(1)}
                </span>
              </span>
            )}
          </div>

          {show.genres.length > 0 && (
            <div className={styles.genres}>
              {show.genres.map(g => (
                <Link key={g} to={`/search?genre=${encodeURIComponent(g)}`} className="badge badge-primary">{g}</Link>
              ))}
            </div>
          )}

          <div className={styles.contentBody}>
            <div className={styles.contentLeft}>
              {show.description && (
                <p className={`body-md ${styles.description}`}>{show.description}</p>
              )}
              <div className={styles.mediaActions}>
                <button
                  className={`btn btn-primary btn-lg ${styles.playNextBtn}`}
                  onClick={playNext}
                  disabled={playNextLoading}
                >
                  {playNextLoading ? (
                    <span className={styles.playNextSpinner} />
                  ) : (
                    <RiPlayFill size={18} />
                  )}
                  Play Next
                </button>
                <button
                  className={`btn btn-icon ${isInWatchlist('tvshow', id!) ? styles.watchlistBtnActive : ''}`}
                  onClick={() => toggle('tvshow', id!)}
                  title={isInWatchlist('tvshow', id!) ? 'Remove from watchlist' : 'Add to watchlist'}
                  aria-label={isInWatchlist('tvshow', id!) ? 'Remove from watchlist' : 'Add to watchlist'}
                >
                  {isInWatchlist('tvshow', id!) ? <RiBookmarkFill size={18} /> : <RiBookmarkLine size={18} />}
                </button>
                <button
                  className={`btn btn-icon ${isWatched ? styles.watchedBtnActive : ''}`}
                  onClick={toggleShowWatched}
                  disabled={watchedSaving || !watchedState || watchedState.total === 0}
                  title={
                    !watchedState || watchedState.total === 0
                      ? 'No episodes to mark'
                      : isWatched ? 'Mark show as unwatched' : 'Mark show as watched'
                  }
                  aria-label={isWatched ? 'Mark show as unwatched' : 'Mark show as watched'}
                  aria-pressed={isWatched}
                >
                  {isWatched ? <RiEyeOffLine size={18} /> : <RiEyeLine size={18} />}
                </button>
              </div>
              {credits && (credits.cast.length > 0 || credits.crew.length > 0) && (
                <TVShowCreditsSection credits={credits} />
              )}
            </div>
            {show.trailer_url && (
              <div className={styles.trailerEmbed}>
                <iframe
                  src={`${show.trailer_url}?rel=0`}
                  title="Trailer"
                  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
                  allowFullScreen
                />
              </div>
            )}
          </div>
        </div>

      </div>

      {/* Seasons */}
      {seasons.length > 0 && (
        <div className={`container ${styles.seasonsSection}`}>
          <h2 className={`headline-md ${styles.seasonsTitle}`}>
            {seasons.length === 1 ? '1 Season' : `${seasons.length} Seasons`}
          </h2>

          <div className={styles.seasonsList}>
            {seasons.map(season => (
              <SeasonRow
                key={season.id}
                showId={id!}
                season={season}
                episodes={episodes[season.id]}
                loading={!!loadingSeasons[season.id]}
                expanded={expandedSeason === season.id}
                onToggle={() => toggleSeason(season.id)}
                isAdmin={isAdmin}
                onEditSeason={() => setEditingSeason(season)}
                onEditEpisode={ep => setEditingEpisode({ seasonId: season.id, episode: ep })}
                onDeleteEpisode={ep => setDeletingEpisode({ seasonId: season.id, episode: ep })}
              />
            ))}
          </div>
        </div>
      )}

      {show && <SimilarCarousel sourceId={show.id} type="tvshow" />}

      {editOpen && show && (
        <MetadataModal
          type="tvshow"
          item={show}
          onSave={updated => setShow(updated)}
          onClose={() => setEditOpen(false)}
        />
      )}

      {identifyOpen && show && (
        <IdentifyTVShowModal
          show={show}
          onClose={() => setIdentifyOpen(false)}
          onSubmitted={() => {
            // Re-fetch shortly after submit so the post-enrich title/poster
            // and any newly-added episodes from the rescan show up without
            // a manual reload.
            setTimeout(() => {
              getOne(show.id).then(updated => updated && setShow(updated)).catch(() => {})
            }, 1500)
          }}
        />
      )}

      {editingSeason && (
        <SeasonMetadataModal
          showId={id!}
          season={editingSeason}
          onSave={updated => {
            setSeasons(prev => prev.map(s => s.id === updated.id ? updated : s))
          }}
          onClose={() => setEditingSeason(null)}
        />
      )}

      {detailsOpen && show && (
        <MediaDetailsModal
          title="Series details"
          item={show as unknown as Record<string, unknown>}
          primaryKeys={['id', 'title', 'year', 'folder_path']}
          onClose={() => setDetailsOpen(false)}
        />
      )}

      {deleteOpen && show && (
        <DeleteMediaModal
          mediaLabel="series"
          title={show.year > 0 ? `${show.title} (${show.year})` : show.title}
          onConfirm={async deleteFiles => {
            await api.deleteTVShow(show.id, deleteFiles)
            navigate(`/library/${show.library_id}`)
          }}
          onClose={() => setDeleteOpen(false)}
        />
      )}

      {deletingEpisode && id && (
        <DeleteMediaModal
          mediaLabel="episode"
          title={`Episode ${deletingEpisode.episode.number}${deletingEpisode.episode.title ? ` · ${deletingEpisode.episode.title}` : ''}`}
          onConfirm={async deleteFiles => {
            const { seasonId, episode } = deletingEpisode
            await api.deleteEpisode(id, seasonId, episode.id, deleteFiles)
            // Drop the row from local state so the list reflects the
            // deletion without waiting for a fresh fetch.
            setEpisodes(prev => {
              const list = prev[seasonId]
              if (!list) return prev
              return { ...prev, [seasonId]: list.filter(e => e.id !== episode.id) }
            })
          }}
          onClose={() => setDeletingEpisode(null)}
        />
      )}

      {editingEpisode && (
        <EpisodeMetadataModal
          showId={id!}
          seasonId={editingEpisode.seasonId}
          episode={editingEpisode.episode}
          seasons={seasons}
          onSave={updated => {
            const oldSeasonId = editingEpisode.seasonId
            const newSeasonId = updated.season_id
            setEpisodes(prev => {
              const next = { ...prev }
              // Remove from old season's list (covers both same-season and
              // reparented edits).
              if (next[oldSeasonId]) {
                next[oldSeasonId] = next[oldSeasonId].filter(e => e.id !== updated.id)
              }
              // Insert into the new season's list if it's loaded; otherwise
              // a fresh fetch on next expand picks it up.
              if (next[newSeasonId]) {
                next[newSeasonId] = [...next[newSeasonId].filter(e => e.id !== updated.id), updated]
                  .sort((a, b) => a.number - b.number)
              }
              return next
            })
          }}
          onClose={() => setEditingEpisode(null)}
        />
      )}
    </div>
  )
}

// ── Credits section ──────────────────────────────────────

function TVShowCreditsSection({ credits }: { credits: Credits }) {
  const creators = credits.crew.filter(c => c.job === 'Creator' || c.job === 'Executive Producer')

  return (
    <div className={styles.creditsSection}>
      {creators.length > 0 && (
        <dl className={styles.creditsRow}>
          <dt className="label-sm">Created by</dt>
          <dd className="body-md">{creators.map(c => c.name).join(', ')}</dd>
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

// ── Season row ───────────────────────────────────────────

interface SeasonRowProps {
  showId: string
  season: Season
  episodes: Episode[] | undefined
  loading: boolean
  expanded: boolean
  onToggle: () => void
  isAdmin: boolean
  onEditSeason: () => void
  onEditEpisode: (ep: Episode) => void
  onDeleteEpisode: (ep: Episode) => void
}

// ── Start Party helper ────────────────────────────────────

async function startEpisodeParty(
  showId: string,
  seasonId: string,
  episodeId: string,
  navigate: ReturnType<typeof useNavigate>,
) {
  const room = await api.createWatchParty({ media_type: 'episode', media_id: episodeId, show_id: showId, season_id: seasonId })
  navigate(`/show/${showId}/season/${seasonId}/episode/${episodeId}/watch?party=${room.id}`)
}

function SeasonRow({ showId, season, episodes, loading, expanded, onToggle, isAdmin, onEditSeason, onEditEpisode, onDeleteEpisode }: SeasonRowProps) {
  const label = season.title && season.title !== `Season ${season.number}`
    ? season.title
    : `Season ${season.number}`

  return (
    <div className={`${styles.season} ${expanded ? styles.seasonExpanded : ''}`}>
      <div className={styles.seasonHeaderRow}>
        <button className={styles.seasonHeader} onClick={onToggle}>
          <span className="label-md">{label}</span>
          <span className={styles.seasonMeta}>
            {season.year > 0 && (
              <span className="label-sm">{season.year}</span>
            )}
            <RiArrowDownSLine size={18} className={styles.seasonChevron} />
          </span>
        </button>
        {isAdmin && (
          <button
            className={`btn btn-icon ${styles.seasonEdit}`}
            onClick={onEditSeason}
            aria-label={`Edit ${label} metadata`}
            title="Edit season"
          >
            <RiEditLine size={15} />
          </button>
        )}
      </div>

      {/* Always-rendered wrapper so the height transition has both endpoints
          to animate between. Inner div has overflow:hidden so the content is
          clipped while the grid track collapses to 0fr in the closed state. */}
      <div className={styles.episodeListWrap} aria-hidden={!expanded}>
        <div className={styles.episodeListInner}>
          <div className={styles.episodeList}>
            {loading && <p className="label-sm" style={{ padding: 'var(--space-3)', color: 'var(--color-on-surface-variant)' }}>Loading…</p>}
            {episodes?.map(ep => (
              <EpisodeRow
                key={ep.id}
                showId={showId}
                seasonId={season.id}
                episode={ep}
                isAdmin={isAdmin}
                onEdit={() => onEditEpisode(ep)}
                onDelete={() => onDeleteEpisode(ep)}
              />
            ))}
            {!loading && episodes?.length === 0 && (
              <p className="label-sm" style={{ padding: 'var(--space-3)', color: 'var(--color-on-surface-variant)' }}>No episodes found</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// ── Episode row ──────────────────────────────────────────

function EpisodeRow({ showId, seasonId, episode, isAdmin, onEdit, onDelete }: { showId: string; seasonId: string; episode: Episode; isAdmin: boolean; onEdit: () => void; onDelete: () => void }) {
  const navigate = useNavigate()
  const runtime = episode.runtime > 0
    ? `${Math.floor(episode.runtime / 60) > 0 ? `${Math.floor(episode.runtime / 60)}h ` : ''}${episode.runtime % 60}m`
    : null

  const airedAt = episode.aired_at
    ? new Date(episode.aired_at).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
    : null

  const watchPath = `/show/${showId}/season/${seasonId}/episode/${episode.id}/watch`

  return (
    <div className={styles.episode}>
      <span className={`label-md ${styles.epNum}`}>
        {episode.is_special ? 'SPEC' : `E${String(episode.number).padStart(2, '0')}`}
      </span>

      <div className={styles.epInfo}>
        <span className="label-md">
          {episode.title || (episode.is_special ? 'Special' : `Episode ${episode.number}`)}
        </span>
        {episode.description && (
          <p className={`label-sm ${styles.epDesc}`}>{episode.description}</p>
        )}
      </div>

      <div className={styles.epMeta}>
        {runtime && (
          <span className={styles.epMetaItem}>
            <RiTimeLine size={12} />
            <span className="label-sm">{runtime}</span>
          </span>
        )}
        {airedAt && <span className="label-sm">{airedAt}</span>}
      </div>

      <div className={styles.epActions}>
        <div className={styles.epPlayWrap}>
          <Link to={watchPath} className={`btn btn-icon ${styles.epPlay}`} aria-label="Play episode">
            <RiPlayFill size={16} />
          </Link>
          {!episode.file_path && (
            <span
              className={styles.epNotReady}
              title="Not transcoded yet — playing from original source"
              aria-label="Not transcoded yet — playing from original source"
            >
              <RiAlertFill size={12} />
            </span>
          )}
        </div>
        <button
          className={`btn btn-icon ${styles.epDownload}`}
          onClick={() => startEpisodeParty(showId, seasonId, episode.id, navigate)}
          aria-label="Start Watch Party"
          title="Start Watch Party"
        >
          <RiGroupLine size={15} />
        </button>
        {episode.file_path && (
          <a
            href={api.episodeDownloadUrl(showId, seasonId, episode.id)}
            className={`btn btn-icon ${styles.epDownload}`}
            aria-label="Download episode"
            title="Download"
          >
            <RiDownloadLine size={16} />
          </a>
        )}
        {isAdmin && (
          <>
            <button
              className={`btn btn-icon ${styles.epDownload}`}
              onClick={onEdit}
              aria-label="Edit episode metadata"
              title="Edit episode"
            >
              <RiEditLine size={15} />
            </button>
            <button
              className={`btn btn-icon ${styles.epDownload}`}
              onClick={onDelete}
              aria-label="Delete episode"
              title="Delete episode"
            >
              <RiDeleteBin6Line size={15} />
            </button>
          </>
        )}
      </div>
    </div>
  )
}

// ── Helpers ──────────────────────────────────────────────

function statusBadgeClass(status: string): string {
  switch (status.toLowerCase()) {
    case 'returning series': return 'badge-secondary'
    case 'ended':
    case 'canceled': return 'badge-error'
    default: return 'badge-primary'
  }
}

function LoadingSkeleton() {
  return (
    <div className={styles.page}>
      <div className={`${styles.hero} ${styles.skeleton} ${styles.skeletonHero}`} />
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
