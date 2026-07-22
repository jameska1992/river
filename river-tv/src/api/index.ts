import { RiverClient, ApiError } from './client'

export { RiverClient, ApiError }

export type {
  BaseModel, User, LoginResponse, ActivityItem, LibraryType,
  Library, Movie, TVShow, Season, Episode,
  WatchParty, WatchPartyMember,
  ServiceLog, LogsResponse,
  CastCredit, CrewCredit, Credits,
  Person, PersonMovieCastItem, PersonMovieCrewItem, PersonTVShowCastItem, PersonTVShowCrewItem,
  Artist, Album, Track, Audiobook, AudiobookChapter,
  RecentlyAddedItem, WatchProgress, ContinueWatchingItem, NextUpItem, ShowWatchState,
  SimilarItem,
  WatchlistItem,
  ActiveSession,
  Subtitle, AudioTrack,
  Collection, CollectionItem, CollectionDetail,
  PaginationParams, SortParams, SortOrder,
  CreateLibraryRequest, UpdateLibraryRequest,
  CreateMovieRequest, UpdateMovieRequest, IdentifyMovieRequest,
  IdentifyTVShowRequest, UnidentifiedItem,
  CreateTVShowRequest, UpdateTVShowRequest,
  CreateSeasonRequest, UpdateSeasonRequest, CreateEpisodeRequest, UpdateEpisodeRequest,
  CreateArtistRequest, UpdateArtistRequest,
  CreateAlbumRequest, UpdateAlbumRequest, CreateTrackRequest,
  CreateAudiobookRequest, UpdateAudiobookRequest, CreateChapterRequest,
  UploadResult,
  SearchResult, LibrarySearchResult, SearchResultItem, PersonSearchResult,
  MovieSearchResult, ShowSearchResult,
} from './types'

export const api = new RiverClient()
