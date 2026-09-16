import { useCallback, useMemo } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { api, type Episode } from '../api'
import { useAsync } from '../hooks/useAsync'
import { VideoPlayer, type UpNext } from '../components/VideoPlayer'
import { episodeCode } from '../util/format'

export default function EpisodePlayerPage() {
  const { showId = '', seasonId = '', episodeId = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  // Set by in-player skip / up-next navigations so the new episode opens at 0
  // rather than resuming a previous viewing's saved position.
  const startFromBeginning = (location.state as { fresh?: boolean } | null)?.fresh === true

  const { data } = useAsync(async () => {
    const [show, seasons, episodes] = await Promise.all([
      api.getTVShow(showId),
      api.listSeasons(showId),
      api.listEpisodes(showId, seasonId),
    ])
    return {
      show,
      season: seasons.find(s => s.id === seasonId) ?? null,
      episodes: [...episodes].sort((a, b) => a.number - b.number),
    }
  }, [showId, seasonId])

  const episodes = useMemo(() => data?.episodes ?? [], [data])
  const current = useMemo(() => episodes.find(e => e.id === episodeId), [episodes, episodeId])

  // Next / previous playable episode within the same season.
  const nextEp = useMemo(
    () => (current ? episodes.find(e => e.number > current.number && e.file_path) : undefined),
    [current, episodes],
  )
  const prevEp = useMemo(() => {
    if (!current) return undefined
    let pick: Episode | undefined
    for (const e of episodes) if (e.number < current.number && e.file_path) pick = e
    return pick
  }, [current, episodes])

  const watchUrl = (epId: string) => `/tvshows/${showId}/seasons/${seasonId}/episodes/${epId}/watch`
  const goFresh = (epId: string) => navigate(watchUrl(epId), { state: { fresh: true } })

  const upNext: UpNext | undefined = useMemo(() => {
    if (!nextEp) return undefined
    const seasonNum = data?.season?.number ?? 0
    return {
      title: nextEp.title || episodeCode(seasonNum, nextEp.number),
      subtitle: seasonNum ? `Season ${seasonNum} · Episode ${nextEp.number}` : undefined,
      posterUrl: data?.show?.backdrop_path || data?.show?.poster_path || undefined,
      onPlay: () => goFresh(nextEp.id),
    }
    // goFresh/navigate are stable enough for this memo; deps track the data.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nextEp, data])

  const fetchSubtitles = useCallback(() => api.getEpisodeSubtitles(showId, seasonId, episodeId), [showId, seasonId, episodeId])
  const fetchAudioTracks = useCallback(() => api.getEpisodeAudioTracks(showId, seasonId, episodeId), [showId, seasonId, episodeId])
  const buildStreamUrl = useCallback(() => api.episodeStreamUrl(showId, seasonId, episodeId), [showId, seasonId, episodeId])

  if (!showId || !seasonId || !episodeId) return null

  const seasonNum = data?.season?.number ?? 0
  const epNum = current?.number ?? 0
  const epLabel = current?.is_special
    ? (seasonNum ? `S${seasonNum} SPEC` : 'SPEC')
    : (seasonNum && epNum ? episodeCode(seasonNum, epNum) : '')
  const line = current?.title ? `${epLabel} · ${current.title}` : epLabel

  return (
    <VideoPlayer
      streamUrl={api.episodeStreamUrl(showId, seasonId, episodeId)}
      buildStreamUrl={buildStreamUrl}
      title={data?.show?.title ?? ''}
      subtitle={line}
      progressKind="episode"
      progressId={episodeId}
      fetchSubtitles={fetchSubtitles}
      fetchAudioTracks={fetchAudioTracks}
      upNext={upNext}
      onPrev={prevEp ? () => goFresh(prevEp.id) : undefined}
      onNext={nextEp ? () => goFresh(nextEp.id) : undefined}
      startFromBeginning={startFromBeginning}
      onExit={() => navigate(`/tvshows/${showId}`, { replace: true })}
    />
  )
}
