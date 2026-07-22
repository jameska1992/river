import type {
  User, LoginResponse, ActivityItem,
  Library, CreateLibraryRequest, UpdateLibraryRequest,
  Movie, CreateMovieRequest, UpdateMovieRequest, IdentifyMovieRequest,
  IdentifyTVShowRequest, UnidentifiedItem,
  TVShow, Season, Episode, CreateTVShowRequest, UpdateTVShowRequest,
  CreateSeasonRequest, UpdateSeasonRequest, CreateEpisodeRequest, UpdateEpisodeRequest,
  Artist, Album, Track, CreateArtistRequest, UpdateArtistRequest,
  CreateAlbumRequest, UpdateAlbumRequest, CreateTrackRequest,
  Audiobook, AudiobookChapter, CreateAudiobookRequest, UpdateAudiobookRequest,
  CreateChapterRequest,
  Credits, Person,
  SearchResult,
  Subtitle, AudioTrack,
  Collection, CollectionDetail, CollectionItem,
  RecentlyAddedItem, WatchProgress, ContinueWatchingItem, NextUpItem, ShowWatchState,
  SimilarItem,
  WatchParty,
  PaginationParams, SortParams, UploadResult,
} from './types'

// Internal raw types — genres/paths arrive as JSON-encoded strings from the API.
interface RawLibrary extends Omit<Library, 'paths'> { paths: string }
interface RawMovie extends Omit<Movie, 'genres'> { genres: string }
interface RawTVShow extends Omit<TVShow, 'genres'> { genres: string }

type QueryParams = Record<string, string | number | undefined>

const ACCESS_KEY = 'river:access_token'
const REFRESH_KEY = 'river:refresh_token'
// Long-lived (~8h) token used only in <video> src URLs. Lives separately
// from the access token because range requests on a <video> element can't
// be refreshed mid-stream — the token has to outlast the longest single
// playback session the user might start.
const STREAM_KEY = 'river:stream_token'
// User-configured backend base URL. The hard-coded default is a
// placeholder so a fresh install has something to fall back to; on a
// real deployment set it via the server picker on the login screen —
// that persists to SharedPreferences and takes precedence. You can also
// change this default to your own river-api address before building.
const BASE_URL_KEY = 'river:api-base'
const DEFAULT_BASE_URL = 'http://localhost:8080/api'
// LRU-ish list of base URLs that have been used successfully. Newest
// first, capped to keep the picker shortlist manageable.
const SERVERS_KEY = 'river:api-servers'
const SERVERS_LIMIT = 8

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export class RiverClient {
  private baseURL: string
  private refreshPromise: Promise<void> | null = null

  constructor(baseURL?: string) {
    // Order of precedence: explicit constructor arg, persisted setting,
    // dev default ('/api' which Vite proxies during development).
    this.baseURL = baseURL
      ?? localStorage.getItem(BASE_URL_KEY)
      ?? DEFAULT_BASE_URL
  }

  // --- Backend URL config ---

  get apiBaseURL(): string {
    return this.baseURL
  }

  setBaseURL(url: string): void {
    // Strip a trailing slash so '${baseURL}${path}' joins cleanly with
    // paths that already start with '/'. Empty input falls back to the
    // dev default and clears the persisted override.
    const normalised = url.trim().replace(/\/+$/, '')
    if (normalised === '') {
      this.baseURL = DEFAULT_BASE_URL
      localStorage.removeItem(BASE_URL_KEY)
      return
    }
    this.baseURL = normalised
    localStorage.setItem(BASE_URL_KEY, normalised)
  }

  // Remembered-server list, newest first. Populated automatically on
  // successful login but also exposed so the LoginPage picker can
  // display + manage entries before the user has authenticated yet.
  getRememberedServers(): string[] {
    try {
      const raw = localStorage.getItem(SERVERS_KEY)
      if (!raw) return []
      const arr = JSON.parse(raw) as unknown
      if (!Array.isArray(arr)) return []
      return arr.filter((s): s is string => typeof s === 'string')
    } catch {
      return []
    }
  }

  private rememberServer(url: string): void {
    const normalised = url.trim().replace(/\/+$/, '')
    if (!normalised) return
    // Push to front + dedupe + cap.
    const next = [normalised, ...this.getRememberedServers().filter(s => s !== normalised)]
      .slice(0, SERVERS_LIMIT)
    localStorage.setItem(SERVERS_KEY, JSON.stringify(next))
  }

  forgetServer(url: string): void {
    const next = this.getRememberedServers().filter(s => s !== url)
    if (next.length === 0) localStorage.removeItem(SERVERS_KEY)
    else localStorage.setItem(SERVERS_KEY, JSON.stringify(next))
  }

  // --- Token storage ---

  get accessToken(): string | null {
    return localStorage.getItem(ACCESS_KEY)
  }

  private setAccessToken(v: string | null) {
    if (v) localStorage.setItem(ACCESS_KEY, v)
    else localStorage.removeItem(ACCESS_KEY)
  }

  get refreshToken(): string | null {
    return localStorage.getItem(REFRESH_KEY)
  }

  private setRefreshToken(v: string | null) {
    if (v) localStorage.setItem(REFRESH_KEY, v)
    else localStorage.removeItem(REFRESH_KEY)
  }

  get streamToken(): string | null {
    return localStorage.getItem(STREAM_KEY)
  }

  private setStreamToken(v: string | null) {
    if (v) localStorage.setItem(STREAM_KEY, v)
    else localStorage.removeItem(STREAM_KEY)
  }

  get isAuthenticated(): boolean {
    return !!this.accessToken
  }

  clearAuth(): void {
    this.setAccessToken(null)
    this.setRefreshToken(null)
    this.setStreamToken(null)
  }

  // --- HTTP core ---

  private buildUrl(path: string, params?: QueryParams): string {
    if (!params) return path
    const q = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== '') q.set(k, String(v))
    }
    const s = q.toString()
    return s ? `${path}?${s}` : path
  }

  private async fetchWithRetry(
    method: string,
    path: string,
    body?: unknown,
    retry = true,
  ): Promise<Response> {
    const headers: Record<string, string> = {}
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    const token = this.accessToken
    if (token) headers['Authorization'] = `Bearer ${token}`

    const res = await fetch(this.baseURL + path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })

    if (res.status === 401 && retry && path !== '/auth/login' && path !== '/auth/refresh') {
      if (!this.refreshPromise) {
        this.refreshPromise = this.doRefresh().finally(() => {
          this.refreshPromise = null
        })
      }
      await this.refreshPromise
      return this.fetchWithRetry(method, path, body, false)
    }
    return res
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    retry = true,
  ): Promise<T> {
    const res = await this.fetchWithRetry(method, path, body, retry)
    if (!res.ok) {
      let message = res.statusText
      try {
        const err = await res.json() as { error?: string }
        message = err.error ?? message
      } catch { /* ignore */ }
      throw new ApiError(res.status, message)
    }
    if (res.status === 204) return undefined as T
    return res.json() as Promise<T>
  }

  // requestPaged is the variant for paginated list endpoints. The server
  // returns the page as a plain JSON array and the total row count in the
  // X-Total-Count header — chosen over a wrapped {items, total} body so
  // non-web clients (mobile, tv) keep parsing the array unchanged. Falls
  // back to items.length if the header is missing so older deployments
  // degrade to a single-page view rather than breaking.
  private async requestPaged<T>(path: string): Promise<{ items: T[]; total: number }> {
    const res = await this.fetchWithRetry('GET', path)
    if (!res.ok) {
      let message = res.statusText
      try {
        const err = await res.json() as { error?: string }
        message = err.error ?? message
      } catch { /* ignore */ }
      throw new ApiError(res.status, message)
    }
    const items = await res.json() as T[]
    const raw = res.headers.get('X-Total-Count')
    const parsed = raw ? parseInt(raw, 10) : NaN
    const total = Number.isFinite(parsed) ? parsed : items.length
    return { items, total }
  }

  // --- JSON string helpers ---

  private parseJson<T>(s: string, fallback: T): T {
    try { return JSON.parse(s) as T } catch { return fallback }
  }

  private parseLibrary(raw: RawLibrary): Library {
    return { ...raw, paths: this.parseJson<string[]>(raw.paths, []) }
  }

  private parseMovie(raw: RawMovie): Movie {
    return {
      ...raw,
      genres: this.parseJson<string[]>(raw.genres, []),
    }
  }

  private parseTVShow(raw: RawTVShow): TVShow {
    return { ...raw, genres: this.parseJson<string[]>(raw.genres, []) }
  }

  private serializeGenres(genres?: string[]): string {
    return JSON.stringify(genres ?? [])
  }

  // --- Auth ---

  async login(username: string, password: string): Promise<LoginResponse> {
    const res = await this.request<LoginResponse>('POST', '/auth/login', { username, password })
    this.setAccessToken(res.access_token)
    this.setRefreshToken(res.refresh_token)
    this.setStreamToken(res.stream_token)
    // Server worked — keep it in the recent list for next time.
    this.rememberServer(this.baseURL)
    return res
  }

  async register(username: string, email: string, password: string): Promise<User> {
    return this.request<User>('POST', '/auth/register', { username, email, password })
  }

  private async doRefresh(): Promise<void> {
    const token = this.refreshToken
    if (!token) {
      this.clearAuth()
      throw new ApiError(401, 'No refresh token available')
    }
    try {
      const res = await this.request<{ access_token: string; refresh_token: string; stream_token: string }>(
        'POST', '/auth/refresh', { refresh_token: token }, false,
      )
      this.setAccessToken(res.access_token)
      this.setRefreshToken(res.refresh_token)
      this.setStreamToken(res.stream_token)
    } catch {
      this.clearAuth()
      throw new ApiError(401, 'Session expired, please log in again')
    }
  }

  async logout(): Promise<void> {
    const token = this.refreshToken
    this.clearAuth()
    if (token) {
      // best-effort — don't propagate errors after clearing local state
      await this.request('POST', '/auth/logout', { refresh_token: token }).catch(() => {})
    }
  }

  async me(): Promise<User> {
    return this.request<User>('GET', '/auth/me')
  }

  // --- Admin ---

  async getStats(): Promise<{ movies: number; tv_shows: number; tracks: number; audiobooks: number }> {
    return this.request('GET', '/admin/stats')
  }

  async triggerScan(): Promise<void> {
    await this.request('POST', '/admin/scan')
  }

  async getActiveSessions(): Promise<import('./types').ActiveSession[]> {
    return this.request('GET', '/admin/active-sessions')
  }

  // --- Watchlist ---

  async getWatchlist(): Promise<import('./types').WatchlistItem[]> {
    return this.request('GET', '/watchlist')
  }

  async addToWatchlist(mediaType: 'movie' | 'tvshow' | 'audiobook', mediaId: string): Promise<import('./types').WatchlistItem> {
    return this.request('POST', '/watchlist', { media_type: mediaType, media_id: mediaId })
  }

  async removeFromWatchlist(id: string): Promise<void> {
    await this.request('DELETE', `/watchlist/${id}`)
  }

  async refreshMovieMetadata(id: string): Promise<void> {
    await this.request('POST', `/movies/${id}/refresh-metadata`)
  }

  async identifyMovie(id: string, data: IdentifyMovieRequest): Promise<void> {
    await this.request('POST', `/movies/${id}/identify`, data)
  }

  async identifyTVShow(id: string, data: IdentifyTVShowRequest): Promise<void> {
    await this.request('POST', `/tvshows/${id}/identify`, data)
  }

  async getUnidentifiedMedia(): Promise<UnidentifiedItem[]> {
    return this.request('GET', '/admin/unidentified')
  }

  async refreshTVShowMetadata(id: string): Promise<void> {
    await this.request('POST', `/tvshows/${id}/refresh-metadata`)
  }

  async refreshAudiobookMetadata(id: string): Promise<void> {
    await this.request('POST', `/audiobooks/${id}/refresh-metadata`)
  }

  async refreshArtistMetadata(id: string): Promise<void> {
    await this.request('POST', `/artists/${id}/refresh-metadata`)
  }

  uploadMedia(data: FormData, onProgress?: (pct: number) => void): Promise<UploadResult> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', this.baseURL + '/admin/upload')
      const token = this.accessToken
      if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)

      if (onProgress) {
        xhr.upload.addEventListener('progress', e => {
          if (e.lengthComputable) onProgress(e.loaded / e.total)
        })
      }

      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try { resolve(JSON.parse(xhr.responseText) as UploadResult) }
          catch { reject(new ApiError(0, 'Invalid response')) }
          return
        }
        let message = xhr.statusText
        try { message = (JSON.parse(xhr.responseText) as { error?: string }).error ?? message }
        catch { /* ignore */ }
        reject(new ApiError(xhr.status, message))
      })

      xhr.addEventListener('error', () => reject(new ApiError(0, 'Network error')))
      xhr.addEventListener('abort', () => reject(new ApiError(0, 'Upload cancelled')))
      xhr.send(data)
    })
  }

  // --- Libraries ---

  async listLibraries(): Promise<Library[]> {
    const raw = await this.request<RawLibrary[]>('GET', '/libraries')
    return raw.map(l => this.parseLibrary(l))
  }

  async getLibrary(id: string): Promise<Library> {
    const raw = await this.request<RawLibrary>('GET', `/libraries/${id}`)
    return this.parseLibrary(raw)
  }

  async createLibrary(data: CreateLibraryRequest): Promise<Library> {
    const raw = await this.request<RawLibrary>('POST', '/libraries', {
      ...data,
      paths: JSON.stringify(data.paths ?? []),
    })
    return this.parseLibrary(raw)
  }

  async updateLibrary(id: string, data: UpdateLibraryRequest): Promise<Library> {
    const raw = await this.request<RawLibrary>('PUT', `/libraries/${id}`, {
      ...data,
      paths: JSON.stringify(data.paths ?? []),
    })
    return this.parseLibrary(raw)
  }

  async deleteLibrary(id: string): Promise<void> {
    return this.request('DELETE', `/libraries/${id}`)
  }

  // --- Movies ---

  async listMovies(params?: { library_id?: string } & PaginationParams & SortParams): Promise<Movie[]> {
    const url = this.buildUrl('/movies', params as QueryParams)
    const raw = await this.request<RawMovie[]>('GET', url)
    return raw.map(m => this.parseMovie(m))
  }

  async listMoviesPaged(params?: { library_id?: string } & PaginationParams & SortParams): Promise<{ items: Movie[]; total: number }> {
    const url = this.buildUrl('/movies', params as QueryParams)
    const { items, total } = await this.requestPaged<RawMovie>(url)
    return { items: items.map(m => this.parseMovie(m)), total }
  }

  async getMovie(id: string): Promise<Movie> {
    const raw = await this.request<RawMovie>('GET', `/movies/${id}`)
    return this.parseMovie(raw)
  }

  async getSimilarMovies(id: string, limit = 16): Promise<SimilarItem[]> {
    return this.request<SimilarItem[]>('GET', `/movies/${id}/similar?limit=${limit}`)
  }

  async createMovie(data: CreateMovieRequest): Promise<Movie> {
    const raw = await this.request<RawMovie>('POST', '/movies', {
      ...data,
      genres: this.serializeGenres(data.genres),
    })
    return this.parseMovie(raw)
  }

  async updateMovie(id: string, data: UpdateMovieRequest): Promise<Movie> {
    const raw = await this.request<RawMovie>('PUT', `/movies/${id}`, {
      ...data,
      genres: this.serializeGenres(data.genres),
    })
    return this.parseMovie(raw)
  }

  async getMovieCredits(id: string): Promise<Credits> {
    return this.request<Credits>('GET', `/movies/${id}/credits`)
  }

  async getTVShowCredits(id: string): Promise<Credits> {
    return this.request<Credits>('GET', `/tvshows/${id}/credits`)
  }

  async getPerson(id: string): Promise<Person> {
    return this.request<Person>('GET', `/people/${id}`)
  }

  async search(params: { q?: string; genre?: string }): Promise<SearchResult> {
    const qs = new URLSearchParams()
    if (params.q)     qs.set('q',     params.q)
    if (params.genre) qs.set('genre', params.genre)
    return this.request<SearchResult>('GET', `/search?${qs.toString()}`)
  }

  // deleteMovie removes the movie row plus its scanner state hash so the
  // file can be re-discovered on the next scan. Set deleteFiles=true to
  // also remove the source (and transcoded copy, if reachable) on disk.
  async deleteMovie(id: string, deleteFiles = false): Promise<void> {
    const qs = deleteFiles ? '?delete_files=true' : ''
    return this.request('DELETE', `/movies/${id}${qs}`)
  }

  movieStreamUrl(id: string): string {
    return this.streamUrl(`/movies/${id}/stream`)
  }

  movieDownloadUrl(id: string): string {
    return this.streamUrl(`/movies/${id}/download`)
  }

  async getMovieSubtitles(id: string): Promise<Subtitle[]> {
    return this.request<Subtitle[]>('GET', `/movies/${id}/subtitles`)
  }

  async getEpisodeSubtitles(showId: string, seasonId: string, episodeId: string): Promise<Subtitle[]> {
    return this.request<Subtitle[]>('GET', `/tvshows/${showId}/seasons/${seasonId}/episodes/${episodeId}/subtitles`)
  }

  subtitleStreamUrl(id: string): string {
    return this.streamUrl(`/subtitles/${id}/stream`)
  }

  async getMovieAudioTracks(id: string): Promise<AudioTrack[]> {
    return this.request<AudioTrack[]>('GET', `/movies/${id}/audio-tracks`)
  }

  async getEpisodeAudioTracks(showId: string, seasonId: string, episodeId: string): Promise<AudioTrack[]> {
    return this.request<AudioTrack[]>('GET', `/tvshows/${showId}/seasons/${seasonId}/episodes/${episodeId}/audio-tracks`)
  }

  audioTrackStreamUrl(id: string): string {
    return this.streamUrl(`/audio-tracks/${id}/stream`)
  }

  // --- TV Shows ---

  async listTVShows(params?: { library_id?: string } & PaginationParams & SortParams): Promise<TVShow[]> {
    const url = this.buildUrl('/tvshows', params as QueryParams)
    const raw = await this.request<RawTVShow[]>('GET', url)
    return raw.map(s => this.parseTVShow(s))
  }

  async listTVShowsPaged(params?: { library_id?: string } & PaginationParams & SortParams): Promise<{ items: TVShow[]; total: number }> {
    const url = this.buildUrl('/tvshows', params as QueryParams)
    const { items, total } = await this.requestPaged<RawTVShow>(url)
    return { items: items.map(s => this.parseTVShow(s)), total }
  }

  async getSimilarShows(id: string, limit = 16): Promise<SimilarItem[]> {
    return this.request<SimilarItem[]>('GET', `/tvshows/${id}/similar?limit=${limit}`)
  }

  async getTVShow(id: string): Promise<TVShow> {
    const raw = await this.request<RawTVShow>('GET', `/tvshows/${id}`)
    return this.parseTVShow(raw)
  }

  async createTVShow(data: CreateTVShowRequest): Promise<TVShow> {
    const raw = await this.request<RawTVShow>('POST', '/tvshows', {
      ...data,
      genres: this.serializeGenres(data.genres),
    })
    return this.parseTVShow(raw)
  }

  async updateTVShow(id: string, data: UpdateTVShowRequest): Promise<TVShow> {
    const raw = await this.request<RawTVShow>('PUT', `/tvshows/${id}`, {
      ...data,
      genres: this.serializeGenres(data.genres),
    })
    return this.parseTVShow(raw)
  }

  // deleteTVShow removes the show row (cascading seasons/episodes) plus
  // the scanner state for each season and the folder-path → show-id
  // mapping. Set deleteFiles=true to also recursively remove the show's
  // folder on disk.
  async deleteTVShow(id: string, deleteFiles = false): Promise<void> {
    const qs = deleteFiles ? '?delete_files=true' : ''
    return this.request('DELETE', `/tvshows/${id}${qs}`)
  }

  // --- Seasons ---

  async listSeasons(showId: string): Promise<Season[]> {
    return this.request<Season[]>('GET', `/tvshows/${showId}/seasons`)
  }

  async createSeason(showId: string, data: CreateSeasonRequest): Promise<Season> {
    return this.request<Season>('POST', `/tvshows/${showId}/seasons`, data)
  }

  async updateSeason(showId: string, seasonId: string, data: UpdateSeasonRequest): Promise<Season> {
    return this.request<Season>('PUT', `/tvshows/${showId}/seasons/${seasonId}`, data)
  }

  // --- Episodes ---

  async listEpisodes(showId: string, seasonId: string): Promise<Episode[]> {
    return this.request<Episode[]>('GET', `/tvshows/${showId}/seasons/${seasonId}/episodes`)
  }

  async createEpisode(showId: string, seasonId: string, data: CreateEpisodeRequest): Promise<Episode> {
    return this.request<Episode>('POST', `/tvshows/${showId}/seasons/${seasonId}/episodes`, data)
  }

  async updateEpisode(showId: string, seasonId: string, episodeId: string, data: UpdateEpisodeRequest): Promise<Episode> {
    return this.request<Episode>('PUT', `/tvshows/${showId}/seasons/${seasonId}/episodes/${episodeId}`, data)
  }

  episodeStreamUrl(showId: string, seasonId: string, episodeId: string): string {
    return this.streamUrl(`/tvshows/${showId}/seasons/${seasonId}/episodes/${episodeId}/stream`)
  }

  episodeDownloadUrl(showId: string, seasonId: string, episodeId: string): string {
    return this.streamUrl(`/tvshows/${showId}/seasons/${seasonId}/episodes/${episodeId}/download`)
  }

  // --- Artists ---

  async listArtists(params?: { library_id?: string } & PaginationParams & SortParams): Promise<Artist[]> {
    return this.request<Artist[]>('GET', this.buildUrl('/artists', params as QueryParams))
  }

  async getArtist(id: string): Promise<Artist> {
    return this.request<Artist>('GET', `/artists/${id}`)
  }

  async createArtist(data: CreateArtistRequest): Promise<Artist> {
    return this.request<Artist>('POST', '/artists', data)
  }

  async updateArtist(id: string, data: UpdateArtistRequest): Promise<Artist> {
    return this.request<Artist>('PUT', `/artists/${id}`, data)
  }

  async deleteArtist(id: string): Promise<void> {
    return this.request('DELETE', `/artists/${id}`)
  }

  async listArtistAlbums(artistId: string, params?: PaginationParams): Promise<Album[]> {
    return this.request<Album[]>('GET', this.buildUrl(`/artists/${artistId}/albums`, params as QueryParams))
  }

  // --- Albums ---

  async listAlbums(params?: { library_id?: string } & PaginationParams & SortParams): Promise<Album[]> {
    return this.request<Album[]>('GET', this.buildUrl('/albums', params as QueryParams))
  }

  async listAlbumsPaged(params?: { library_id?: string } & PaginationParams & SortParams): Promise<{ items: Album[]; total: number }> {
    return this.requestPaged<Album>(this.buildUrl('/albums', params as QueryParams))
  }

  async getAlbum(id: string): Promise<Album> {
    return this.request<Album>('GET', `/albums/${id}`)
  }

  async createAlbum(data: CreateAlbumRequest): Promise<Album> {
    return this.request<Album>('POST', '/albums', data)
  }

  async updateAlbum(id: string, data: UpdateAlbumRequest): Promise<Album> {
    return this.request<Album>('PUT', `/albums/${id}`, data)
  }

  async deleteAlbum(id: string): Promise<void> {
    return this.request('DELETE', `/albums/${id}`)
  }

  async listAlbumTracks(albumId: string, params?: PaginationParams): Promise<Track[]> {
    return this.request<Track[]>('GET', this.buildUrl(`/albums/${albumId}/tracks`, params as QueryParams))
  }

  // --- Tracks ---

  async getTrack(id: string): Promise<Track> {
    return this.request<Track>('GET', `/tracks/${id}`)
  }

  async createTrack(data: CreateTrackRequest): Promise<Track> {
    return this.request<Track>('POST', '/tracks', data)
  }

  async deleteTrack(id: string): Promise<void> {
    return this.request('DELETE', `/tracks/${id}`)
  }

  trackStreamUrl(id: string): string {
    return this.streamUrl(`/tracks/${id}/stream`)
  }

  // --- Audiobooks ---

  async listAudiobooks(params?: { library_id?: string } & PaginationParams & SortParams): Promise<Audiobook[]> {
    return this.request<Audiobook[]>('GET', this.buildUrl('/audiobooks', params as QueryParams))
  }

  async listAudiobooksPaged(params?: { library_id?: string } & PaginationParams & SortParams): Promise<{ items: Audiobook[]; total: number }> {
    return this.requestPaged<Audiobook>(this.buildUrl('/audiobooks', params as QueryParams))
  }

  async getAudiobook(id: string): Promise<Audiobook> {
    return this.request<Audiobook>('GET', `/audiobooks/${id}`)
  }

  async getSimilarAudiobooks(id: string, limit = 16): Promise<SimilarItem[]> {
    return this.request<SimilarItem[]>('GET', `/audiobooks/${id}/similar?limit=${limit}`)
  }

  async createAudiobook(data: CreateAudiobookRequest): Promise<Audiobook> {
    return this.request<Audiobook>('POST', '/audiobooks', data)
  }

  async updateAudiobook(id: string, data: UpdateAudiobookRequest): Promise<Audiobook> {
    return this.request<Audiobook>('PUT', `/audiobooks/${id}`, data)
  }

  async deleteAudiobook(id: string): Promise<void> {
    return this.request('DELETE', `/audiobooks/${id}`)
  }

  // --- Chapters ---

  async listChapters(audiobookId: string): Promise<AudiobookChapter[]> {
    return this.request<AudiobookChapter[]>('GET', `/audiobooks/${audiobookId}/chapters`)
  }

  async createChapter(audiobookId: string, data: CreateChapterRequest): Promise<AudiobookChapter> {
    return this.request<AudiobookChapter>('POST', `/audiobooks/${audiobookId}/chapters`, data)
  }

  chapterStreamUrl(audiobookId: string, chapterId: string): string {
    return this.streamUrl(`/audiobooks/${audiobookId}/chapters/${chapterId}/stream`)
  }

  // --- Recently Added ---

  async getRecentlyAdded(): Promise<RecentlyAddedItem[]> {
    interface RawItem extends Omit<RecentlyAddedItem, 'genres'> { genres: string }
    const raw = await this.request<RawItem[]>('GET', '/recently-added')
    return raw.map(item => ({ ...item, genres: this.parseJson<string[]>(item.genres, []) }))
  }

  async getNextEpisode(showId: string): Promise<{ season_id: string; episode_id: string }> {
    return this.request('GET', `/tvshows/${showId}/next-episode`)
  }

  // --- Watch Progress ---

  openProgressSocket(): { send(mediaType: 'movie' | 'episode' | 'chapter', mediaId: string, position: number, duration: number): void; close(): void } {
    const queue: string[] = []
    const token = this.accessToken
    const base = new URL(this.baseURL + '/progress/ws', window.location.href)
    base.protocol = base.protocol.replace('http', 'ws')
    if (token) base.searchParams.set('token', token)
    const ws = new WebSocket(base.toString())
    ws.addEventListener('open', () => {
      for (const msg of queue.splice(0)) ws.send(msg)
    })
    return {
      send(mediaType, mediaId, position, duration) {
        const msg = JSON.stringify({ media_type: mediaType, media_id: mediaId, position, duration })
        if (ws.readyState === WebSocket.OPEN) ws.send(msg)
        else if (ws.readyState === WebSocket.CONNECTING) queue.push(msg)
      },
      close() { ws.close(1000, 'watch page unmounting') },
    }
  }

  async getProgress(mediaType: 'movie' | 'episode' | 'chapter', mediaId: string): Promise<WatchProgress | null> {
    try {
      return await this.request<WatchProgress>('GET', `/progress?media_type=${mediaType}&media_id=${encodeURIComponent(mediaId)}`)
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) return null
      throw e
    }
  }

  async getContinueWatching(): Promise<ContinueWatchingItem[]> {
    return this.request<ContinueWatchingItem[]>('GET', '/progress/continue-watching')
  }

  async getNextUp(limit = 16): Promise<NextUpItem[]> {
    return this.request<NextUpItem[]>('GET', `/progress/next-up?limit=${limit}`)
  }

  async dismissNextUp(episodeId: string): Promise<void> {
    return this.request('POST', `/progress/next-up/${encodeURIComponent(episodeId)}/dismiss`)
  }

  async undismissNextUp(episodeId: string): Promise<void> {
    return this.request('DELETE', `/progress/next-up/${encodeURIComponent(episodeId)}/dismiss`)
  }

  async deleteProgress(mediaType: 'movie' | 'episode' | 'chapter', mediaId: string): Promise<void> {
    return this.request('DELETE', `/progress?media_type=${mediaType}&media_id=${encodeURIComponent(mediaId)}`)
  }

  async setProgressCompleted(mediaType: 'movie' | 'episode' | 'chapter', mediaId: string, completed: boolean): Promise<void> {
    return this.request('PUT', '/progress/completed', { media_type: mediaType, media_id: mediaId, completed })
  }

  async setShowCompleted(showId: string, completed: boolean): Promise<void> {
    return this.request('PUT', '/progress/show-completed', { show_id: showId, completed })
  }

  async getShowWatchStates(libraryId?: string): Promise<ShowWatchState[]> {
    const qs = libraryId ? `?library_id=${encodeURIComponent(libraryId)}` : ''
    return this.request<ShowWatchState[]>('GET', `/progress/show-states${qs}`)
  }

  async getShowWatchState(showId: string): Promise<ShowWatchState> {
    return this.request<ShowWatchState>('GET', `/progress/show-state?show_id=${encodeURIComponent(showId)}`)
  }

  async getProgressAll(mediaType: 'movie' | 'episode' | 'chapter'): Promise<WatchProgress[]> {
    return this.request<WatchProgress[]>('GET', `/progress/all?media_type=${mediaType}`)
  }

  // --- Collections ---

  async listCollections(): Promise<Collection[]> {
    return this.request<Collection[]>('GET', '/collections')
  }

  async createCollection(data: { name: string; description?: string }): Promise<Collection> {
    return this.request<Collection>('POST', '/collections', data)
  }

  async getCollection(id: string): Promise<CollectionDetail> {
    return this.request<CollectionDetail>('GET', `/collections/${id}`)
  }

  async updateCollection(id: string, data: { name: string; description?: string }): Promise<Collection> {
    return this.request<Collection>('PUT', `/collections/${id}`, data)
  }

  async deleteCollection(id: string): Promise<void> {
    return this.request('DELETE', `/collections/${id}`)
  }

  async addCollectionItem(collectionId: string, mediaType: 'movie' | 'tvshow' | 'audiobook', mediaId: string): Promise<CollectionItem> {
    return this.request<CollectionItem>('POST', `/collections/${collectionId}/items`, { media_type: mediaType, media_id: mediaId })
  }

  async removeCollectionItem(collectionId: string, itemId: string): Promise<void> {
    return this.request('DELETE', `/collections/${collectionId}/items/${itemId}`)
  }

  // --- Admin: user management ---

  async listUsers(): Promise<User[]> {
    return this.request<User[]>('GET', '/admin/users')
  }

  async createUser(data: { username: string; email: string; password: string; role: string }): Promise<User> {
    return this.request<User>('POST', '/admin/users', data)
  }

  async updateUser(id: string, data: { username: string; email: string; role: string }): Promise<User> {
    return this.request<User>('PUT', `/admin/users/${id}`, data)
  }

  async setUserPassword(id: string, password: string): Promise<void> {
    return this.request('POST', `/admin/users/${id}/set-password`, { password })
  }

  async deleteUser(id: string): Promise<void> {
    return this.request('DELETE', `/admin/users/${id}`)
  }

  async getUserActivity(id: string): Promise<ActivityItem[]> {
    return this.request<ActivityItem[]>('GET', `/admin/users/${id}/activity`)
  }

  // --- Watch Party ---

  async createWatchParty(input: { media_type: string; media_id: string; show_id?: string; season_id?: string }): Promise<WatchParty> {
    return this.request<WatchParty>('POST', '/watchparty', input)
  }

  async getWatchParty(id: string): Promise<WatchParty> {
    return this.request<WatchParty>('GET', `/watchparty/${id}`)
  }

  async deleteWatchParty(id: string): Promise<void> {
    return this.request('DELETE', `/watchparty/${id}`)
  }

  openWatchPartySocket(roomId: string): {
    send(type: string, position: number): void
    onMessage(cb: (msg: unknown) => void): void
    close(): void
  } {
    const token = this.accessToken
    const base = new URL(`${this.baseURL}/watchparty/${roomId}/ws`, window.location.href)
    base.protocol = base.protocol.replace('http', 'ws')
    if (token) base.searchParams.set('token', token)
    const ws = new WebSocket(base.toString())

    let messageCallback: ((msg: unknown) => void) | null = null
    ws.addEventListener('message', e => {
      if (messageCallback) {
        try { messageCallback(JSON.parse(e.data as string)) } catch { /* ignore */ }
      }
    })

    return {
      send(type, position) {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type, position }))
        }
      },
      onMessage(cb) { messageCallback = cb },
      close() { ws.close(1000, 'leaving watch party') },
    }
  }

  async getLogs(params?: {
    level?: string
    service?: string
    from?: string
    to?: string
    page?: number
    limit?: number
  }): Promise<{ logs: import('./types').ServiceLog[]; total: number }> {
    const q = new URLSearchParams()
    if (params?.level)   q.set('level',   params.level)
    if (params?.service) q.set('service', params.service)
    if (params?.from)    q.set('from',    params.from)
    if (params?.to)      q.set('to',      params.to)
    if (params?.page)    q.set('page',    String(params.page))
    if (params?.limit)   q.set('limit',   String(params.limit))
    const qs = q.toString()
    return this.request('GET', `/admin/logs${qs ? `?${qs}` : ''}`)
  }

  async updateMe(email: string): Promise<import('./types').User> {
    return this.request('PUT', '/auth/me', { email })
  }

  async changePassword(currentPassword: string, newPassword: string): Promise<void> {
    return this.request('POST', '/auth/me/password', { current_password: currentPassword, new_password: newPassword })
  }

  async searchMovieRequests(q: string): Promise<import('./types').MovieSearchResult[]> {
    return this.request('GET', `/request/movies?q=${encodeURIComponent(q)}`)
  }

  async searchShowRequests(q: string): Promise<import('./types').ShowSearchResult[]> {
    return this.request('GET', `/request/shows?q=${encodeURIComponent(q)}`)
  }

  async requestMovie(tmdbId: number, title: string, year: number): Promise<void> {
    return this.request('POST', '/request/movies', { tmdbId, title, year })
  }

  async requestShow(tvdbId: number, title: string, year: number): Promise<void> {
    return this.request('POST', '/request/shows', { tvdbId, title, year })
  }

  // streamUrl builds a fully-qualified URL with the long-lived stream
  // token in the query string. Used for media URLs that go into <video>
  // / <audio> src — those can't carry an Authorization header, and the
  // regular access token (15 min) would otherwise expire mid-playback.
  // Falls back to the access token only when the stream token isn't
  // available yet (e.g. session created against an older server build).
  private streamUrl(path: string): string {
    const token = this.streamToken || this.accessToken
    const base = `${this.baseURL}${path}`
    return token ? `${base}?token=${encodeURIComponent(token)}` : base
  }
}
