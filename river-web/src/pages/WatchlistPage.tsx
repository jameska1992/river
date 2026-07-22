import { RiFilmLine, RiTv2Line, RiHeadphoneLine, RiBookmarkLine } from 'react-icons/ri'
import { useNavigate } from 'react-router-dom'
import { useWatchlist } from '../context/WatchlistContext'
import { api } from '../api'
import { MediaCard } from '../components/MediaCard'
import styles from './WatchlistPage.module.css'

const FALLBACK_ICONS = {
  movie:     <RiFilmLine />,
  tvshow:    <RiTv2Line />,
  audiobook: <RiHeadphoneLine />,
}

const DETAIL_PATHS = {
  movie:     (id: string) => `/movie/${id}`,
  tvshow:    (id: string) => `/show/${id}`,
  audiobook: (id: string) => `/audiobook/${id}`,
}

export function WatchlistPage() {
  const { items, isInWatchlist, toggle } = useWatchlist()
  const navigate = useNavigate()

  const playMovie = (id: string) => navigate(`/movie/${id}/watch`)

  const playShow = async (id: string) => {
    try {
      const { season_id, episode_id } = await api.getNextEpisode(id)
      navigate(`/show/${id}/season/${season_id}/episode/${episode_id}/watch`)
    } catch {
      navigate(`/show/${id}`)
    }
  }

  return (
    <div className={`container ${styles.page}`}>
      <div className={styles.heading}>
        <h1 className="headline-lg">Watchlist</h1>
        {items.length > 0 && (
          <span className={`label-sm ${styles.count}`}>
            {items.length} {items.length === 1 ? 'item' : 'items'}
          </span>
        )}
      </div>

      {items.length === 0 ? (
        <div className={styles.empty}>
          <RiBookmarkLine size={40} className={styles.emptyIcon} />
          <p className="body-md">Your watchlist is empty.</p>
          <p className="body-sm">
            Bookmark movies, shows, and audiobooks to keep track of what you want to watch.
          </p>
        </div>
      ) : (
        <div className={styles.grid}>
          {items.map(item => {
            const onPlay =
              item.media_type === 'movie'
                ? () => playMovie(item.media_id)
                : item.media_type === 'tvshow'
                ? () => playShow(item.media_id)
                : undefined
            return (
              <MediaCard
                key={item.id}
                title={item.title}
                subtitle={item.year > 0 ? String(item.year) : undefined}
                imageSrc={item.poster_path || undefined}
                fallbackIcon={FALLBACK_ICONS[item.media_type]}
                to={DETAIL_PATHS[item.media_type](item.media_id)}
                onPlay={onPlay}
                inWatchlist={isInWatchlist(item.media_type, item.media_id)}
                onWatchlistToggle={() => toggle(item.media_type, item.media_id)}
              />
            )
          })}
        </div>
      )}
    </div>
  )
}
