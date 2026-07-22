import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import {
  RiArrowLeftLine, RiMusicLine, RiPlayFill, RiTimeLine, RiDownloadLine,
  RiUserLine,
} from 'react-icons/ri'
import { useMusic } from '../context/MusicContext'
import { useAuth } from '../context/AuthContext'
import type { Album, Artist, Track } from '../api'
import { api } from '../api'
import { AdminMediaMenu } from '../components/AdminMediaMenu'
import { MediaDetailsModal } from '../components/MediaDetailsModal'
import { MetadataModal } from '../components/MetadataModal'
import { imageUrl } from '../util/imageUrl'
import styles from './AlbumDetailPage.module.css'

export function AlbumDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { getAlbum, fetchTracks, trackStreamUrl } = useMusic()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'

  const [album, setAlbum] = useState<Album | null>(null)
  const [artist, setArtist] = useState<Artist | null>(null)
  const [tracks, setTracks] = useState<Track[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)

  useEffect(() => {
    if (!id) return
    setIsLoading(true)
    setError(null)
    getAlbum(id)
      .then(async a => {
        setAlbum(a)
        const [t, ar] = await Promise.all([
          fetchTracks(id),
          a.artist_id ? api.getArtist(a.artist_id).catch(() => null) : Promise.resolve(null),
        ])
        setTracks([...t].sort((a, b) => a.disc_number - b.disc_number || a.number - b.number))
        setArtist(ar)
      })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load album'))
      .finally(() => setIsLoading(false))
  }, [id, getAlbum, fetchTracks])


  if (isLoading) return <LoadingSkeleton />

  if (error) {
    return (
      <div className={styles.errorPage}>
        <p className="body-md" style={{ color: 'var(--color-error)' }}>{error}</p>
      </div>
    )
  }

  if (!album) return null

  const firstTrack = tracks[0]

  return (
    <div className={styles.page}>
      {/* Hero */}
      <div className={styles.hero}>
        {album.cover_path ? (
          <img src={imageUrl(album.cover_path, 'backdrop')} alt="" className={styles.heroImage} loading="eager" />
        ) : (
          <div className={styles.heroFallback}>
            <RiMusicLine className={styles.heroFallbackIcon} />
          </div>
        )}
        <div className={styles.heroGradient} />
        <Link
          to={artist ? `/artist/${artist.id}` : `/library/${album.library_id}`}
          className={`btn btn-icon ${styles.backBtn}`}
        >
          <RiArrowLeftLine size={20} />
        </Link>
      </div>

      {/* Content row */}
      <div className={`container ${styles.content}`}>
        {/* Cover */}
        <div className={`card card-square ${styles.cover}`}>
          {album.cover_path ? (
            <img src={imageUrl(album.cover_path)} alt={album.title} loading="eager" />
          ) : (
            <div className={styles.coverFallback}>
              <RiMusicLine size={48} />
            </div>
          )}
        </div>

        {/* Metadata */}
        <div className={styles.meta}>
          <div className={styles.titleRow}>
            <h1 className={`headline-lg ${styles.title}`}>{album.title}</h1>
            {isAdmin && (
              <AdminMediaMenu
                onRefresh={album.artist_id ? () => api.refreshArtistMetadata(album.artist_id!) : undefined}
                onEdit={() => setEditOpen(true)}
                onShowDetails={() => setDetailsOpen(true)}
              />
            )}
          </div>

          {artist && (
            <Link to={`/artist/${artist.id}`} className={`label-md ${styles.artist}`}>
              <RiUserLine size={14} />
              {artist.name}
            </Link>
          )}

          <div className={styles.attrs}>
            {album.year > 0 && (
              <span className="label-md" style={{ color: 'var(--color-on-surface-variant)' }}>
                {album.year}
              </span>
            )}
            {tracks.length > 0 && (
              <span className="label-md" style={{ color: 'var(--color-on-surface-variant)' }}>
                {tracks.length} {tracks.length === 1 ? 'track' : 'tracks'}
              </span>
            )}
          </div>

          {album.genre && (
            <div className={styles.genres}>
              <span className="badge badge-primary">{album.genre}</span>
            </div>
          )}

          <div className={styles.mediaActions}>
            {firstTrack?.file_path && (
              <Link
                to={`/album/${id}/play`}
                className={`btn btn-primary ${styles.playBtn}`}
              >
                <RiPlayFill size={18} />
                Play
              </Link>
            )}
          </div>
        </div>
      </div>

      {/* Tracks */}
      {tracks.length > 0 && (
        <div className={`container ${styles.tracksSection}`}>
          <h2 className={`headline-md ${styles.tracksTitle}`}>
            {tracks.length === 1 ? '1 Track' : `${tracks.length} Tracks`}
          </h2>
          <div className={styles.trackList}>
            {tracks.map(t => (
              <TrackRow key={t.id} albumId={id!} track={t} trackStreamUrl={trackStreamUrl} />
            ))}
          </div>
        </div>
      )}

      {editOpen && (
        <MetadataModal
          type="album"
          item={album}
          onSave={updated => setAlbum(updated)}
          onClose={() => setEditOpen(false)}
        />
      )}

      {detailsOpen && (
        <MediaDetailsModal
          title="Album details"
          item={album as unknown as Record<string, unknown>}
          primaryKeys={['id', 'title', 'year', 'artist_id']}
          onClose={() => setDetailsOpen(false)}
        />
      )}
    </div>
  )
}

function TrackRow({
  albumId,
  track,
  trackStreamUrl,
}: {
  albumId: string
  track: Track
  trackStreamUrl: (id: string) => string
}) {
  const listenTo = `/album/${albumId}/play?track=${track.id}`
  const downloadUrl = trackStreamUrl(track.id)

  return (
    <div className={styles.track}>
      <span className={`label-md ${styles.trackNum}`}>{String(track.number).padStart(2, '0')}</span>

      <div className={styles.trackInfo}>
        <span className="label-md">{track.title}</span>
      </div>

      {track.duration > 0 && (
        <div className={styles.trackMeta}>
          <span className={styles.trackMetaItem}>
            <RiTimeLine size={12} />
            <span className="label-sm">{formatDuration(track.duration)}</span>
          </span>
        </div>
      )}

      <div className={styles.trackActions}>
        {track.file_path ? (
          <Link to={listenTo} className={`btn btn-icon ${styles.trackPlay}`} aria-label="Play track">
            <RiPlayFill size={16} />
          </Link>
        ) : (
          <button className={`btn btn-icon ${styles.trackPlay}`} disabled aria-label="Not available">
            <RiPlayFill size={16} />
          </button>
        )}
        {track.file_path && (
          <a
            href={downloadUrl}
            download
            className={`btn btn-icon ${styles.trackDownload}`}
            aria-label="Download track"
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
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

function LoadingSkeleton() {
  return (
    <div className={styles.page}>
      <div className={`${styles.hero} ${styles.skeleton} ${styles.skeletonHero}`} />
      <div className={`container ${styles.content}`}>
        <div className={`card card-square ${styles.cover} ${styles.skeleton}`} />
        <div className={styles.meta}>
          <div className={`${styles.skeleton} ${styles.skeletonTitle}`} />
          <div className={`${styles.skeleton} ${styles.skeletonLine}`} />
          <div className={`${styles.skeleton} ${styles.skeletonLine} ${styles.skeletonLineShort}`} />
        </div>
      </div>
    </div>
  )
}
