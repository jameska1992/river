import { useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type Movie, type SortOrder } from '../api'
import { BrowsePage, type BrowseCard } from './BrowsePage'

export default function MoviesPage() {
  const navigate = useNavigate()
  const fetchPage = useCallback(
    (page: number, limit: number, sort: string, order: SortOrder) =>
      api.listMoviesPaged({ page, limit, sort, order }),
    [],
  )
  const toCard = useCallback((m: Movie): BrowseCard => ({
    key: m.id,
    title: m.title,
    subtitle: m.year ? String(m.year) : undefined,
    imageSrc: m.poster_path || undefined,
    onSelect: () => navigate(`/movies/${m.id}`),
  }), [navigate])

  return (
    <BrowsePage
      title="Movies"
      countSuffix="titles"
      sortPrefKey="river-tv:movies-sort"
      fetchPage={fetchPage}
      toCard={toCard}
    />
  )
}
