import { useCallback, useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  api,
  type Episode,
  type Season,
  type TVShow,
} from '../api'
import { PlayerScreen, type UpNext } from '../components/PlayerScreen'

export default function EpisodePlayerPage() {
  const { showId = '', seasonId = '', episodeId = '' } = useParams<{
    showId: string
    seasonId: string
    episodeId: string
  }>()
  const navigate = useNavigate()
  const location = useLocation()
  // Flagged by in-player skip-next / skip-prev / up-next navigations so
  // the new episode opens at 0 instead of resuming the saved position
  // from a previous viewing.
  const startFromBeginning = (location.state as { fresh?: boolean } | null)?.fresh === true

  const [show, setShow] = useState<TVShow | null>(null)
  const [season, setSeason] = useState<Season | null>(null)
  const [seasons, setSeasons] = useState<Season[]>([])
  const [episodes, setEpisodes] = useState<Episode[]>([])

  useEffect(() => {
    if (!showId || !seasonId) return
    let alive = true
    Promise.all([
      api.getTVShow(showId),
      api.listSeasons(showId),
      api.listEpisodes(showId, seasonId),
    ])
      .then(([s, allSeasons, eps]) => {
        if (!alive) return
        setShow(s)
        const sortedSeasons = [...allSeasons].sort((a, b) => a.number - b.number)
        setSeasons(sortedSeasons)
        setSeason(sortedSeasons.find(x => x.id === seasonId) ?? null)
        setEpisodes([...eps].sort((a, b) => a.number - b.number))
      })
      .catch(() => {})
    return () => { alive = false }
  }, [showId, seasonId])

  const current = useMemo(() => episodes.find(e => e.id === episodeId), [episodes, episodeId])

  // Pick the next playable episode in the same season; cross-season
  // jumps fall back to the show detail page (we don't have the next
  // season's episode list without another fetch).
  const nextEp = useMemo(
    () => current ? episodes.find(e => e.number > current.number && e.file_path) : undefined,
    [current, episodes],
  )
  // Previous playable episode in the same season. Looking backwards
  // so we want the LAST episode with number < current's.
  const prevEp = useMemo(() => {
    if (!current) return undefined
    let pick: Episode | undefined
    for (const e of episodes) {
      if (e.number < current.number && e.file_path) pick = e
    }
    return pick
  }, [current, episodes])

  const nextEpWatchUrl = nextEp
    ? `/tvshows/${showId}/seasons/${seasonId}/episodes/${nextEp.id}/watch`
    : null
  const prevEpWatchUrl = prevEp
    ? `/tvshows/${showId}/seasons/${seasonId}/episodes/${prevEp.id}/watch`
    : null

  // Up Next overlay — prefer the same-season next episode, fall back
  // to next season's first episode (deferred to show detail page).
  const upNext: UpNext | undefined = useMemo(() => {
    if (!current) return undefined
    if (nextEp && nextEpWatchUrl) {
      return {
        title: nextEp.title || `Episode ${nextEp.number}`,
        subtitle: season ? `Season ${season.number} · Episode ${nextEp.number}` : undefined,
        posterUrl: season?.poster_path || undefined,
        // { state: { fresh: true } } — the user is explicitly jumping
        // to the next item, so skip the resume-position step on arrival.
        onPlay: () => navigate(nextEpWatchUrl, { state: { fresh: true } }),
      }
    }
    if (!season) return undefined
    const nextSeason = seasons.find(s => s.number > season.number)
    if (!nextSeason) return undefined
    return {
      title: `Season ${nextSeason.number}`,
      subtitle: 'Up next',
      posterUrl: nextSeason.poster_path || undefined,
      onPlay: () => navigate(`/tvshows/${showId}`),
    }
  }, [current, nextEp, nextEpWatchUrl, season, seasons, showId, navigate])

  const onNext = nextEpWatchUrl
    ? () => navigate(nextEpWatchUrl, { state: { fresh: true } })
    : undefined
  const onPrev = prevEpWatchUrl
    ? () => navigate(prevEpWatchUrl, { state: { fresh: true } })
    : undefined

  const fetchSubtitles = useCallback(
    () => api.getEpisodeSubtitles(showId, seasonId, episodeId),
    [showId, seasonId, episodeId],
  )
  const fetchAudioTracks = useCallback(
    () => api.getEpisodeAudioTracks(showId, seasonId, episodeId),
    [showId, seasonId, episodeId],
  )

  if (!showId || !seasonId || !episodeId) return null

  const seasonNum = season?.number ?? 0
  const epNum = current?.number ?? 0
  const epLabel = current?.is_special
    ? (seasonNum ? `S${seasonNum} SPEC` : 'SPEC')
    : (seasonNum && epNum ? `S${seasonNum}E${epNum}` : '')
  const subtitleLine = current?.title
    ? `${epLabel} · ${current.title}`
    : epLabel

  return (
    <PlayerScreen
      streamUrl={api.episodeStreamUrl(showId, seasonId, episodeId)}
      title={show?.title ?? ''}
      subtitle={subtitleLine}
      progressKind="episode"
      progressId={episodeId}
      fetchSubtitles={fetchSubtitles}
      fetchAudioTracks={fetchAudioTracks}
      upNext={upNext}
      onPrev={onPrev}
      onNext={onNext}
      startFromBeginning={startFromBeginning}
      // Always exit to the show's detail page (replace so Back from
      // there returns to the upstream page rather than back into the
      // player). Works even when the user landed on the player via a
      // deep link with no history to pop.
      onExit={() => navigate(`/tvshows/${showId}`, { replace: true })}
    />
  )
}
