import { useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type Audiobook, type SortOrder } from '../api'
import { BrowsePage, type BrowseCard, type SortOption } from './BrowsePage'

// Server-side whitelist for audiobooks (see river-api
// audiobookSortColumns): title, author, year, added. Rating isn't
// stored on audiobooks; author replaces it as a useful sort axis.
const AUDIOBOOK_SORT_OPTIONS: SortOption[] = [
  { label: 'Title (A–Z)',    sort: 'title',  order: 'asc'  },
  { label: 'Title (Z–A)',    sort: 'title',  order: 'desc' },
  { label: 'Author (A–Z)',   sort: 'author', order: 'asc'  },
  { label: 'Author (Z–A)',   sort: 'author', order: 'desc' },
  { label: 'Year (newest)',  sort: 'year',   order: 'desc' },
  { label: 'Year (oldest)',  sort: 'year',   order: 'asc'  },
  { label: 'Recently added', sort: 'added',  order: 'desc' },
]

export default function AudiobooksPage() {
  const navigate = useNavigate()
  const fetchPage = useCallback(
    (page: number, limit: number, sort: string, order: SortOrder) =>
      api.listAudiobooksPaged({ page, limit, sort, order }),
    [],
  )
  const toCard = useCallback((a: Audiobook): BrowseCard => ({
    key: a.id,
    title: a.title,
    subtitle: a.author || (a.year ? String(a.year) : undefined),
    imageSrc: a.cover_path || undefined,
    onSelect: () => navigate(`/audiobooks/${a.id}`),
  }), [navigate])

  return (
    <BrowsePage
      title="Audiobooks"
      countSuffix="audiobooks"
      sortPrefKey="river-tv:audiobooks-sort"
      sortOptions={AUDIOBOOK_SORT_OPTIONS}
      cardAspect="square"
      fetchPage={fetchPage}
      toCard={toCard}
    />
  )
}
