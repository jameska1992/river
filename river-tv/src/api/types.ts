export interface BaseModel {
  id: string
  created_at: string
  updated_at: string
}

// --- Auth ---

export interface User extends BaseModel {
  username: string
  email: string
  role: 'admin' | 'user'
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  stream_token: string
  user: User
}

export interface ActivityItem {
  media_type: string
  media_id: string
  title: string
  show_title?: string
  show_id?: string
  position: number
  duration: number
  completed: boolean
  updated_at: string
}

// --- Libraries ---

export type LibraryType = 'movie' | 'tvshow' | 'music' | 'audiobook'

export interface Library extends BaseModel {
  name: string
  type: LibraryType
  paths: string[]
}

// --- Movies ---

export interface Movie extends BaseModel {
  library_id: string
  title: string
  original_title: string
  description: string
  year: number
  genres: string[]
  rating: number
  runtime: number    // minutes
  poster_path: string
  backdrop_path: string
  trailer_url: string
  file_path: string
  source_path: string
}

export interface CastCredit {
  person_id: string
  tmdb_id: number | null
  name: string
  profile_path: string
  character: string
  order: number
}

export interface CrewCredit {
  person_id: string
  tmdb_id: number | null
  name: string
  profile_path: string
  job: string
  department: string
}

export interface Credits {
  cast: CastCredit[]
  crew: CrewCredit[]
}

// --- People ---

export interface PersonMovieCastItem {
  movie_id: string
  title: string
  year: number
  poster_path: string
  character: string
}

export interface PersonMovieCrewItem {
  movie_id: string
  title: string
  year: number
  poster_path: string
  job: string
  department: string
}

export interface PersonTVShowCastItem {
  tv_show_id: string
  title: string
  year: number
  poster_path: string
  character: string
}

export interface PersonTVShowCrewItem {
  tv_show_id: string
  title: string
  year: number
  poster_path: string
  job: string
  department: string
}

export interface Person {
  id: string
  name: string
  profile_path: string
  biography: string
  tmdb_id: number | null
  movie_cast: PersonMovieCastItem[]
  movie_crew: PersonMovieCrewItem[]
  tv_show_cast: PersonTVShowCastItem[]
  tv_show_crew: PersonTVShowCrewItem[]
}

// --- TV Shows ---

export interface TVShow extends BaseModel {
  library_id: string
  title: string
  original_title: string
  description: string
  year: number
  status: string
  genres: string[]
  rating: number
  poster_path: string
  backdrop_path: string
  trailer_url: string
  seasons?: Season[]
}

export interface Season extends BaseModel {
  tv_show_id: string
  number: number
  title: string
  description: string
  year: number
  poster_path: string
  episodes?: Episode[]
}

export interface Episode extends BaseModel {
  season_id: string
  tv_show_id: string
  number: number
  title: string
  description: string
  runtime: number    // minutes
  file_path: string
  source_path: string
  aired_at: string
  is_special: boolean
}

// --- Music ---

export interface Artist extends BaseModel {
  library_id: string
  name: string
  bio: string
  image_path: string
  albums?: Album[]
}

export interface Album extends BaseModel {
  library_id: string
  artist_id: string
  title: string
  year: number
  genre: string
  cover_path: string
  tracks?: Track[]
}

export interface Track extends BaseModel {
  library_id: string
  album_id: string
  artist_id: string
  title: string
  number: number
  disc_number: number
  duration: number   // seconds
  file_path: string
}

// --- Audiobooks ---

export interface Audiobook extends BaseModel {
  library_id: string
  title: string
  author: string
  narrator: string
  description: string
  year: number
  genre: string
  cover_path: string
  duration: number   // seconds
  chapters?: AudiobookChapter[]
}

export interface AudiobookChapter extends BaseModel {
  audiobook_id: string
  number: number
  title: string
  duration: number   // seconds
  file_path: string
}

// --- Recently Added ---

export interface RecentlyAddedItem {
  id: string
  media_type: 'movie' | 'tvshow'
  title: string
  year: number
  description: string
  genres: string[]
  rating: number
  poster_path: string
  backdrop_path: string
  file_path: string
  added_at: string
}

// --- Watchlist ---

export interface WatchlistItem extends BaseModel {
  media_type: 'movie' | 'tvshow' | 'audiobook'
  media_id: string
  title: string
  year: number
  poster_path: string
  added_at: string
}

// --- Active Sessions ---

export interface ActiveSession {
  user_id: string
  username: string
  media_type: 'movie' | 'episode' | 'chapter'
  title: string
  // Doubles as parent title: TV show name for episodes, audiobook title
  // for chapters. The admin UI renders "<title> · <show_title>".
  show_title?: string
  position: number  // seconds
  duration: number  // seconds
  updated_at: string
}

// --- Watch Progress ---

// ShowWatchState summarises how much of a show the user has watched. A
// show is considered fully watched when completed === total && total > 0.
// total === 0 means the show has no episode records yet (likely
// unscanned or pre-enrichment) — UI treats that as "not watchable".
export interface ShowWatchState {
  show_id: string
  total: number
  completed: number
}

export interface WatchProgress extends BaseModel {
  user_id: string
  media_type: 'movie' | 'episode' | 'chapter'
  media_id: string
  position: number  // seconds
  duration: number  // seconds
  completed: boolean
}

export interface ContinueWatchingItem extends WatchProgress {
  title: string
  poster_path: string
  // Wide "cover/backdrop" artwork — populated for movies and episodes,
  // empty for audiobooks (which only have a square cover surfaced via
  // poster_path). The home rail prefers this when present.
  backdrop_path?: string
  // Episode-only:
  show_title?: string
  show_id?: string
  season_id?: string
  season_number?: number
  episode_number?: number
  // Chapter (audiobook) only. The deduped continue-watching feed surfaces
  // at most one row per audiobook, so audiobook_id is what you route to;
  // chapter_number/title are for "Chapter N: Title" display.
  audiobook_id?: string
  chapter_number?: number
  chapter_title?: string
}

// NextUpItem is the shape river-api returns from /progress/next-up.
// One row per TV show the user has recently completed an episode of —
// media_id is the *next* episode to play, not the one just completed.
export interface NextUpItem {
  media_type: 'episode'
  media_id: string
  title: string
  poster_path: string
  backdrop_path?: string
  show_title: string
  show_id: string
  season_id: string
  season_number: number
  episode_number: number
  updated_at: string
}

// SimilarItem is the trimmed shape returned by the /*/similar
// endpoints — one entry per "more like this" card in the recommended
// carousel at the bottom of movie / show / audiobook detail pages.
export interface SimilarItem {
  id: string
  type: 'movie' | 'tvshow' | 'audiobook'
  title: string
  year?: number
  poster_path: string
  backdrop_path?: string
}

// --- Pagination ---

export interface PaginationParams {
  page?: number
  limit?: number
}

// --- Sort ---

export type SortOrder = 'asc' | 'desc'

// SortParams piggy-backs onto list-call params. `sort` field names are
// per-resource and whitelisted on the server (see services/movie.go etc.);
// unknown values fall back to the default sort.
export interface SortParams {
  sort?: string
  order?: SortOrder
}

// --- Request types ---

export interface CreateLibraryRequest {
  name: string
  type: LibraryType
  paths?: string[]
}
export type UpdateLibraryRequest = CreateLibraryRequest

export interface CreateMovieRequest {
  library_id: string
  title: string
  file_path: string
  original_title?: string
  description?: string
  year?: number
  genres?: string[]
  rating?: number
  runtime?: number
  poster_path?: string
  backdrop_path?: string
}
export type UpdateMovieRequest = CreateMovieRequest

export interface IdentifyMovieRequest {
  title?: string
  year?: number
  imdb_id?: string
}

export interface IdentifyTVShowRequest {
  title?: string
  year?: number
  imdb_id?: string
}

export interface UnidentifiedItem {
  id: string
  type: 'movie' | 'tvshow'
  title: string
  year: number
  library_id: string
  file_path?: string
}

export interface CreateTVShowRequest {
  library_id: string
  title: string
  original_title?: string
  description?: string
  year?: number
  status?: string
  genres?: string[]
  rating?: number
  poster_path?: string
  backdrop_path?: string
}
export type UpdateTVShowRequest = CreateTVShowRequest

export interface CreateSeasonRequest {
  number: number
  title?: string
  description?: string
  year?: number
  poster_path?: string
}

// PATCH-style: every field optional. Server skips empty/zero fields so
// callers can edit a subset without re-sending the rest.
export interface UpdateSeasonRequest {
  number?: number
  title?: string
  description?: string
  year?: number
  poster_path?: string
}

export interface CreateEpisodeRequest {
  number: number
  file_path: string
  title?: string
  description?: string
  runtime?: number
  aired_at?: string
}

// PATCH-style. season_id moves the episode to a different season under the
// same show — useful when the scanner misclassified it.
export interface UpdateEpisodeRequest {
  number?: number
  season_id?: string
  title?: string
  description?: string
  runtime?: number
  aired_at?: string
}

export interface CreateArtistRequest {
  library_id: string
  name: string
  bio?: string
  image_path?: string
}
export type UpdateArtistRequest = CreateArtistRequest

export interface CreateAlbumRequest {
  library_id: string
  title: string
  artist_id?: string
  year?: number
  genre?: string
  cover_path?: string
}
export type UpdateAlbumRequest = CreateAlbumRequest

export interface CreateTrackRequest {
  library_id: string
  album_id: string
  title: string
  file_path: string
  artist_id?: string
  number?: number
  disc_number?: number
  duration?: number
}

export interface CreateAudiobookRequest {
  library_id: string
  title: string
  author?: string
  narrator?: string
  description?: string
  year?: number
  genre?: string
  cover_path?: string
  duration?: number
}
export type UpdateAudiobookRequest = CreateAudiobookRequest

export interface CreateChapterRequest {
  number: number
  file_path: string
  title?: string
  duration?: number
}

export interface UploadResult {
  library_id: string
  path: string
}

// --- Search ---

export interface SearchResultItem {
  id: string
  title: string
  year: number
  poster_path: string
  media_type: 'movie' | 'tvshow' | 'audiobook'
}

export interface LibrarySearchResult {
  library_id: string
  library_name: string
  library_type: string
  items: SearchResultItem[]
}

export interface SearchResult {
  libraries: LibrarySearchResult[]
  people: PersonSearchResult[]
}

export interface PersonSearchResult {
  id: string
  name: string
  profile_path: string
}

// --- Subtitles ---

export interface Subtitle extends BaseModel {
  media_type: 'movie' | 'episode'
  media_id: string
  language: string
  label: string
  file_path: string
}

// --- Collections ---

export interface Collection extends BaseModel {
  user_id: string
  name: string
  description: string
  item_count: number
  covers: string[]
}

export interface CollectionItem extends BaseModel {
  collection_id: string
  media_type: 'movie' | 'tvshow' | 'audiobook'
  media_id: string
  sort_order: number
  title: string
  poster_path: string
  year?: number
}

export interface CollectionDetail extends Collection {
  items: CollectionItem[]
}

// --- Watch Party ---

export interface WatchParty extends BaseModel {
  host_id: string
  media_type: 'movie' | 'episode'
  media_id: string
  show_id?: string
  season_id?: string
}

export interface WatchPartyMember {
  user_id: string
  username: string
}

// --- Service Logs ---

export interface ServiceLog extends BaseModel {
  level: 'info' | 'warn' | 'error'
  service: string
  message: string
}

export interface LogsResponse {
  logs: ServiceLog[]
  total: number
}

// --- Requests (Radarr / Sonarr) ---

export interface MovieSearchResult {
  tmdbId: number
  title: string
  year: number
  overview: string
  poster: string
  added: boolean
}

export interface ShowSearchResult {
  tvdbId: number
  title: string
  year: number
  overview: string
  poster: string
  added: boolean
}

// --- Audio tracks ---

export interface AudioTrack extends BaseModel {
  media_type: 'movie' | 'episode'
  media_id: string
  language: string
  label: string
  stream_index: number
}
