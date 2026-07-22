import { useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type TVShow, type SortOrder } from '../api'
import { BrowsePage, type BrowseCard } from './BrowsePage'

export default function TVShowsPage() {
  const navigate = useNavigate()
  const fetchPage = useCallback(
    (page: number, limit: number, sort: string, order: SortOrder) =>
      api.listTVShowsPaged({ page, limit, sort, order }),
    [],
  )
  const toCard = useCallback((s: TVShow): BrowseCard => ({
    key: s.id,
    title: s.title,
    subtitle: s.year ? String(s.year) : undefined,
    imageSrc: s.poster_path || undefined,
    onSelect: () => navigate(`/tvshows/${s.id}`),
  }), [navigate])

  return (
    <BrowsePage
      title="TV Shows"
      countSuffix="shows"
      sortPrefKey="river-tv:tvshows-sort"
      fetchPage={fetchPage}
      toCard={toCard}
    />
  )
}
