import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { RiArrowLeftLine, RiUserLine, RiMusicLine } from 'react-icons/ri'
import { useMusic } from '../context/MusicContext'
import { useAuth } from '../context/AuthContext'
import type { Artist, Album } from '../api'
import { api } from '../api'
import { AdminMediaMenu } from '../components/AdminMediaMenu'
import { MetadataModal } from '../components/MetadataModal'
import { MediaDetailsModal } from '../components/MediaDetailsModal'
import { imageUrl } from '../util/imageUrl'
import styles from './ArtistDetailPage.module.css'

export function ArtistDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { getArtist } = useMusic()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const navigate = useNavigate()

  const [artist, setArtist] = useState<Artist | null>(null)
  const [albums, setAlbums] = useState<Album[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)

  useEffect(() => {
    if (!id) return
    setIsLoading(true)
    setError(null)
    Promise.all([
      getArtist(id),
      api.listArtistAlbums(id),
    ])
      .then(([a, albs]) => {
        setArtist(a)
        setAlbums([...albs].sort((a, b) => (a.year || 0) - (b.year || 0)))
      })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load artist'))
      .finally(() => setIsLoading(false))
  }, [id, getArtist])

  if (isLoading) return <LoadingSkeleton />

  if (error) {
    return (
      <div className={styles.errorPage}>
        <p className="body-md" style={{ color: 'var(--color-error)' }}>{error}</p>
      </div>
    )
  }

  if (!artist) return null

  return (
    <div className={styles.page}>
      {/* Hero */}
      <div className={styles.hero}>
        {artist.image_path ? (
          <img src={imageUrl(artist.image_path, 'backdrop')} alt="" className={styles.heroImage} loading="eager" />
        ) : (
          <div className={styles.heroFallback}>
            <RiUserLine className={styles.heroFallbackIcon} />
          </div>
        )}
        <div className={styles.heroGradient} />
        <button
          className={`btn btn-icon ${styles.backBtn}`}
          onClick={() => navigate(-1)}
          aria-label="Go back"
        >
          <RiArrowLeftLine size={20} />
        </button>
      </div>

      {/* Content row */}
      <div className={`container ${styles.content}`}>
        {/* Artist image */}
        <div className={`${styles.avatar}`}>
          {artist.image_path ? (
            <img src={imageUrl(artist.image_path)} alt={artist.name} className={styles.avatarImg} loading="eager" />
          ) : (
            <div className={styles.avatarFallback}>
              <RiUserLine size={48} />
            </div>
          )}
        </div>

        {/* Metadata */}
        <div className={styles.meta}>
          <div className={styles.titleRow}>
            <h1 className={`headline-lg ${styles.title}`}>{artist.name}</h1>
            {isAdmin && (
              <AdminMediaMenu
                onRefresh={() => api.refreshArtistMetadata(id!)}
                onEdit={() => setEditOpen(true)}
                onShowDetails={() => setDetailsOpen(true)}
              />
            )}
          </div>

          {albums.length > 0 && (
            <p className="label-md" style={{ color: 'var(--color-on-surface-variant)', marginBottom: 'var(--space-2)' }}>
              {albums.length} {albums.length === 1 ? 'album' : 'albums'}
            </p>
          )}

          {artist.bio && (
            <p className={`body-md ${styles.bio}`}>{artist.bio}</p>
          )}
        </div>
      </div>

      {/* Albums */}
      {albums.length > 0 && (
        <div className={`container ${styles.albumsSection}`}>
          <h2 className={`headline-md ${styles.albumsTitle}`}>Albums</h2>
          <div className={styles.albumsGrid}>
            {albums.map(album => (
              <AlbumCard key={album.id} album={album} />
            ))}
          </div>
        </div>
      )}

      {editOpen && artist && (
        <MetadataModal
          type="artist"
          item={artist}
          onSave={updated => setArtist(updated)}
          onClose={() => setEditOpen(false)}
        />
      )}

      {detailsOpen && artist && (
        <MediaDetailsModal
          title="Artist details"
          item={artist as unknown as Record<string, unknown>}
          primaryKeys={['id', 'name', 'library_id']}
          onClose={() => setDetailsOpen(false)}
        />
      )}
    </div>
  )
}

function AlbumCard({ album }: { album: Album }) {
  return (
    <Link to={`/album/${album.id}`} className={styles.albumCard}>
      <div className={`card card-square ${styles.albumCover}`}>
        {album.cover_path ? (
          <img src={imageUrl(album.cover_path)} alt={album.title} loading="lazy" />
        ) : (
          <div className={styles.albumCoverFallback}>
            <RiMusicLine size={32} />
          </div>
        )}
      </div>
      <p className={`label-md ${styles.albumTitle}`}>{album.title}</p>
      {album.year > 0 && (
        <p className={`label-sm ${styles.albumYear}`}>{album.year}</p>
      )}
    </Link>
  )
}

function LoadingSkeleton() {
  return (
    <div className={styles.page}>
      <div className={`${styles.hero} ${styles.skeleton} ${styles.skeletonHero}`} />
      <div className={`container ${styles.content}`}>
        <div className={`${styles.avatar} ${styles.skeleton}`} />
        <div className={styles.meta}>
          <div className={`${styles.skeleton} ${styles.skeletonTitle}`} />
          <div className={`${styles.skeleton} ${styles.skeletonLine}`} />
        </div>
      </div>
    </div>
  )
}
