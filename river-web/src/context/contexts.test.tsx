import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type {
  User, Library, Movie, TVShow, Season, Episode,
  Artist, Album, Track, Audiobook, AudiobookChapter, WatchlistItem,
} from '../api'

// The providers are the service layer over the `api` singleton. We mock the
// whole `../api` module so each test drives the provider purely through the
// api methods it calls and asserts the resulting context state. ApiError is a
// real Error subclass so the providers' `err instanceof Error` branches work.
const { mockApi, ApiError } = vi.hoisted(() => {
  class ApiError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.name = 'ApiError'
      this.status = status
    }
  }
  const fn = () => vi.fn()
  return {
    ApiError,
    mockApi: {
      // auth
      isAuthenticated: false,
      me: fn(), clearAuth: fn(), login: fn(), register: fn(), logout: fn(),
      // libraries
      listLibraries: fn(), createLibrary: fn(), updateLibrary: fn(), deleteLibrary: fn(),
      // movies
      listMovies: fn(), getMovie: fn(), createMovie: fn(), updateMovie: fn(),
      deleteMovie: fn(), movieStreamUrl: fn(),
      // tv
      listTVShows: fn(), getTVShow: fn(), createTVShow: fn(), updateTVShow: fn(),
      deleteTVShow: fn(), listSeasons: fn(), createSeason: fn(), listEpisodes: fn(),
      createEpisode: fn(), episodeStreamUrl: fn(),
      // music
      listArtists: fn(), listAlbums: fn(), getArtist: fn(), createArtist: fn(),
      updateArtist: fn(), deleteArtist: fn(), getAlbum: fn(), createAlbum: fn(),
      updateAlbum: fn(), deleteAlbum: fn(), listAlbumTracks: fn(), createTrack: fn(),
      deleteTrack: fn(), trackStreamUrl: fn(),
      // audiobooks
      listAudiobooks: fn(), getAudiobook: fn(), createAudiobook: fn(),
      updateAudiobook: fn(), deleteAudiobook: fn(), listChapters: fn(),
      createChapter: fn(), chapterStreamUrl: fn(),
      // watchlist
      getWatchlist: fn(), addToWatchlist: fn(), removeFromWatchlist: fn(),
    },
  }
})

vi.mock('../api', () => ({ api: mockApi, ApiError, RiverClient: class {} }))

// Import providers AFTER the mock is registered.
import { AuthProvider, useAuth } from './AuthContext'
import { LibrariesProvider, useLibraries } from './LibrariesContext'
import { MoviesProvider, useMovies } from './MoviesContext'
import { TVShowsProvider, useTVShows } from './TVShowsContext'
import { MusicProvider, useMusic } from './MusicContext'
import { AudiobooksProvider, useAudiobooks } from './AudiobooksContext'
import { WatchlistProvider, useWatchlist } from './WatchlistContext'

// Build a minimal domain object cast to its full type — tests only touch the
// fields the providers key off (id / media_type / media_id). NoInfer keeps the
// argument from driving inference so T is taken from the call's contextual type
// (e.g. the request type a provider method expects), not from the partial.
function make<T>(o: NoInfer<Partial<T>>): T {
  return o as T
}

// A promise whose resolution we control, for asserting the loading state that
// exists only while an in-flight request is pending.
function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.resetAllMocks()
  mockApi.isAuthenticated = false
  // Safe defaults for the methods providers call on mount.
  mockApi.me.mockResolvedValue(make<User>({ id: 'u1', username: 'neo' }))
  mockApi.getWatchlist.mockResolvedValue([])
})

describe('AuthProvider', () => {
  it('does not bootstrap when unauthenticated (no me(), not loading)', () => {
    mockApi.isAuthenticated = false
    const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider })

    expect(result.current.isLoading).toBe(false)
    expect(result.current.user).toBeNull()
    expect(mockApi.me).not.toHaveBeenCalled()
  })

  it('bootstraps the user from me() on mount when authenticated', async () => {
    mockApi.isAuthenticated = true
    mockApi.me.mockResolvedValue(make<User>({ id: 'u1', username: 'neo' }))

    const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider })
    // isLoading is seeded true from api.isAuthenticated
    expect(result.current.isLoading).toBe(true)

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.user).toEqual({ id: 'u1', username: 'neo' })
  })

  it('clears auth and stays logged-out when the bootstrap me() fails', async () => {
    mockApi.isAuthenticated = true
    mockApi.me.mockRejectedValue(new ApiError(401, 'expired'))

    const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(mockApi.clearAuth).toHaveBeenCalled()
    expect(result.current.user).toBeNull()
  })

  it('login stores the returned user', async () => {
    mockApi.login.mockResolvedValue({ user: make<User>({ id: 'u2', username: 'trin' }) })
    const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider })

    await act(async () => { await result.current.login('trin', 'pw') })

    expect(mockApi.login).toHaveBeenCalledWith('trin', 'pw')
    expect(result.current.user).toEqual({ id: 'u2', username: 'trin' })
  })

  it('register delegates to the api without setting a user', async () => {
    mockApi.register.mockResolvedValue(make<User>({ id: 'u3' }))
    const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider })

    await act(async () => { await result.current.register('morph', 'm@x.io', 'pw') })

    expect(mockApi.register).toHaveBeenCalledWith('morph', 'm@x.io', 'pw')
    expect(result.current.user).toBeNull()
  })

  it('logout calls the api and clears the user', async () => {
    mockApi.isAuthenticated = true
    mockApi.me.mockResolvedValue(make<User>({ id: 'u1' }))
    mockApi.logout.mockResolvedValue(undefined)
    const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider })
    await waitFor(() => expect(result.current.user).not.toBeNull())

    await act(async () => { await result.current.logout() })

    expect(mockApi.logout).toHaveBeenCalled()
    expect(result.current.user).toBeNull()
  })

  it('refreshUser re-fetches the user via me()', async () => {
    mockApi.me.mockResolvedValue(make<User>({ id: 'u9', username: 'fresh' }))
    const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider })

    await act(async () => { await result.current.refreshUser() })

    expect(result.current.user).toEqual({ id: 'u9', username: 'fresh' })
  })
})

describe('LibrariesProvider', () => {
  const libA = make<Library>({ id: 'l1', name: 'Films' })
  const libB = make<Library>({ id: 'l2', name: 'Shows' })

  it('fetch: loading → data', async () => {
    const d = deferred<Library[]>()
    mockApi.listLibraries.mockReturnValue(d.promise)
    const { result } = renderHook(() => useLibraries(), { wrapper: LibrariesProvider })

    let p!: Promise<void>
    act(() => { p = result.current.fetch() })
    expect(result.current.isLoading).toBe(true)

    d.resolve([libA, libB])
    await act(async () => { await p })
    expect(result.current.isLoading).toBe(false)
    expect(result.current.error).toBeNull()
    expect(result.current.libraries).toEqual([libA, libB])
  })

  it('fetch: error sets the message and stops loading', async () => {
    mockApi.listLibraries.mockRejectedValue(new ApiError(500, 'boom'))
    const { result } = renderHook(() => useLibraries(), { wrapper: LibrariesProvider })

    await act(async () => { await result.current.fetch() })

    expect(result.current.error).toBe('boom')
    expect(result.current.isLoading).toBe(false)
  })

  it('create appends, update replaces, remove filters', async () => {
    mockApi.listLibraries.mockResolvedValue([libA])
    mockApi.createLibrary.mockResolvedValue(libB)
    const { result } = renderHook(() => useLibraries(), { wrapper: LibrariesProvider })
    await act(async () => { await result.current.fetch() })

    await act(async () => { await result.current.create(make({ name: 'Shows' })) })
    expect(result.current.libraries).toEqual([libA, libB])

    const libBRenamed = make<Library>({ id: 'l2', name: 'TV' })
    mockApi.updateLibrary.mockResolvedValue(libBRenamed)
    await act(async () => { await result.current.update('l2', make({ name: 'TV' })) })
    expect(result.current.libraries).toEqual([libA, libBRenamed])

    mockApi.deleteLibrary.mockResolvedValue(undefined)
    await act(async () => { await result.current.remove('l1') })
    expect(result.current.libraries).toEqual([libBRenamed])
  })
})

describe('MoviesProvider', () => {
  const movieA = make<Movie>({ id: 'm1', title: 'Heat', genres: [] })
  const movieB = make<Movie>({ id: 'm2', title: 'Collateral', genres: [] })

  it('fetch passes the library_id filter and populates state', async () => {
    mockApi.listMovies.mockResolvedValue([movieA])
    const { result } = renderHook(() => useMovies(), { wrapper: MoviesProvider })

    await act(async () => { await result.current.fetch('lib1') })

    expect(mockApi.listMovies).toHaveBeenCalledWith({ library_id: 'lib1' })
    expect(result.current.movies).toEqual([movieA])
  })

  it('fetch without a library_id passes undefined', async () => {
    mockApi.listMovies.mockResolvedValue([])
    const { result } = renderHook(() => useMovies(), { wrapper: MoviesProvider })

    await act(async () => { await result.current.fetch() })

    expect(mockApi.listMovies).toHaveBeenCalledWith(undefined)
  })

  it('fetch error sets the fallback message', async () => {
    mockApi.listMovies.mockRejectedValue(new ApiError(500, 'nope'))
    const { result } = renderHook(() => useMovies(), { wrapper: MoviesProvider })

    await act(async () => { await result.current.fetch() })

    expect(result.current.error).toBe('nope')
  })

  it('create / update / remove keep the list in sync', async () => {
    mockApi.listMovies.mockResolvedValue([movieA])
    mockApi.createMovie.mockResolvedValue(movieB)
    const { result } = renderHook(() => useMovies(), { wrapper: MoviesProvider })
    await act(async () => { await result.current.fetch() })

    await act(async () => { await result.current.create(make({ title: 'Collateral' })) })
    expect(result.current.movies.map(m => m.id)).toEqual(['m1', 'm2'])

    const movieBEdited = make<Movie>({ id: 'm2', title: 'Collateral (2004)', genres: [] })
    mockApi.updateMovie.mockResolvedValue(movieBEdited)
    await act(async () => { await result.current.update('m2', make({ title: 'Collateral (2004)' })) })
    expect(result.current.movies.find(m => m.id === 'm2')).toEqual(movieBEdited)

    mockApi.deleteMovie.mockResolvedValue(undefined)
    await act(async () => { await result.current.remove('m1') })
    expect(result.current.movies.map(m => m.id)).toEqual(['m2'])
  })

  it('getOne and streamUrl delegate straight to the api', async () => {
    mockApi.getMovie.mockResolvedValue(movieA)
    mockApi.movieStreamUrl.mockReturnValue('/api/movies/m1/stream?token=t')
    const { result } = renderHook(() => useMovies(), { wrapper: MoviesProvider })

    await act(async () => { expect(await result.current.getOne('m1')).toEqual(movieA) })
    expect(result.current.streamUrl('m1')).toBe('/api/movies/m1/stream?token=t')
  })
})

describe('TVShowsProvider', () => {
  const showA = make<TVShow>({ id: 't1', title: 'Wire', genres: [] })

  it('fetch populates shows and error branch sets the message', async () => {
    mockApi.listTVShows.mockResolvedValue([showA])
    const { result } = renderHook(() => useTVShows(), { wrapper: TVShowsProvider })
    await act(async () => { await result.current.fetch() })
    expect(result.current.shows).toEqual([showA])

    mockApi.listTVShows.mockRejectedValue(new ApiError(500, 'tv-boom'))
    await act(async () => { await result.current.fetch() })
    expect(result.current.error).toBe('tv-boom')
  })

  it('create appends and remove filters', async () => {
    mockApi.createTVShow.mockResolvedValue(showA)
    const { result } = renderHook(() => useTVShows(), { wrapper: TVShowsProvider })

    await act(async () => { await result.current.create(make({ title: 'Wire' })) })
    expect(result.current.shows).toEqual([showA])

    const showEdited = make<TVShow>({ id: 't1', title: 'The Wire', genres: [] })
    mockApi.updateTVShow.mockResolvedValue(showEdited)
    await act(async () => { await result.current.update('t1', make({ title: 'The Wire' })) })
    expect(result.current.shows).toEqual([showEdited])

    mockApi.deleteTVShow.mockResolvedValue(undefined)
    await act(async () => { await result.current.remove('t1') })
    expect(result.current.shows).toEqual([])
  })

  it('season / episode helpers delegate to the api with the right ids', async () => {
    const season = make<Season>({ id: 's1' })
    const episode = make<Episode>({ id: 'e1' })
    mockApi.listSeasons.mockResolvedValue([season])
    mockApi.createSeason.mockResolvedValue(season)
    mockApi.listEpisodes.mockResolvedValue([episode])
    mockApi.createEpisode.mockResolvedValue(episode)
    mockApi.episodeStreamUrl.mockReturnValue('/ep/stream')
    const { result } = renderHook(() => useTVShows(), { wrapper: TVShowsProvider })

    await act(async () => {
      expect(await result.current.fetchSeasons('t1')).toEqual([season])
      expect(await result.current.createSeason('t1', make({}))).toEqual(season)
      expect(await result.current.fetchEpisodes('t1', 's1')).toEqual([episode])
      expect(await result.current.createEpisode('t1', 's1', make({}))).toEqual(episode)
    })
    expect(mockApi.listSeasons).toHaveBeenCalledWith('t1')
    expect(mockApi.listEpisodes).toHaveBeenCalledWith('t1', 's1')
    expect(result.current.episodeStreamUrl('t1', 's1', 'e1')).toBe('/ep/stream')
  })
})

describe('MusicProvider', () => {
  const artist = make<Artist>({ id: 'ar1', name: 'Boards' })
  const album = make<Album>({ id: 'al1', title: 'Music Has...' })

  it('fetchArtists populates artists; fetchAlbums populates albums', async () => {
    mockApi.listArtists.mockResolvedValue([artist])
    mockApi.listAlbums.mockResolvedValue([album])
    const { result } = renderHook(() => useMusic(), { wrapper: MusicProvider })

    await act(async () => { await result.current.fetchArtists('lib1') })
    expect(mockApi.listArtists).toHaveBeenCalledWith({ library_id: 'lib1' })
    expect(result.current.artists).toEqual([artist])

    await act(async () => { await result.current.fetchAlbums() })
    expect(mockApi.listAlbums).toHaveBeenCalledWith(undefined)
    expect(result.current.albums).toEqual([album])
  })

  it('fetchArtists error sets the message', async () => {
    mockApi.listArtists.mockRejectedValue(new ApiError(500, 'artist-boom'))
    const { result } = renderHook(() => useMusic(), { wrapper: MusicProvider })

    await act(async () => { await result.current.fetchArtists() })

    expect(result.current.error).toBe('artist-boom')
  })

  it('artist mutations keep the list in sync', async () => {
    mockApi.listArtists.mockResolvedValue([])
    mockApi.createArtist.mockResolvedValue(artist)
    const artistEdited = make<Artist>({ id: 'ar1', name: 'Boards of Canada' })
    mockApi.updateArtist.mockResolvedValue(artistEdited)
    mockApi.deleteArtist.mockResolvedValue(undefined)
    const { result } = renderHook(() => useMusic(), { wrapper: MusicProvider })

    await act(async () => { await result.current.createArtist(make({ name: 'Boards' })) })
    expect(result.current.artists).toEqual([artist])

    await act(async () => { await result.current.updateArtist('ar1', make({ name: 'Boards of Canada' })) })
    expect(result.current.artists).toEqual([artistEdited])

    await act(async () => { await result.current.removeArtist('ar1') })
    expect(result.current.artists).toEqual([])
  })

  it('album mutations keep the list in sync', async () => {
    mockApi.listAlbums.mockResolvedValue([])
    mockApi.createAlbum.mockResolvedValue(album)
    const albumEdited = make<Album>({ id: 'al1', title: 'Music Has the Right to Children' })
    mockApi.updateAlbum.mockResolvedValue(albumEdited)
    mockApi.deleteAlbum.mockResolvedValue(undefined)
    const { result } = renderHook(() => useMusic(), { wrapper: MusicProvider })

    await act(async () => { await result.current.createAlbum(make({ title: 'Music Has...' })) })
    expect(result.current.albums).toEqual([album])

    await act(async () => { await result.current.updateAlbum('al1', make({ title: 'Music Has the Right to Children' })) })
    expect(result.current.albums).toEqual([albumEdited])

    await act(async () => { await result.current.removeAlbum('al1') })
    expect(result.current.albums).toEqual([])
  })

  it('getArtist / getAlbum / track helpers delegate to the api', async () => {
    const track = make<Track>({ id: 'tr1', title: 'Roygbiv' })
    mockApi.getArtist.mockResolvedValue(artist)
    mockApi.getAlbum.mockResolvedValue(album)
    mockApi.listAlbumTracks.mockResolvedValue([track])
    mockApi.createTrack.mockResolvedValue(track)
    mockApi.deleteTrack.mockResolvedValue(undefined)
    mockApi.trackStreamUrl.mockReturnValue('/api/tracks/tr1/stream?token=t')
    const { result } = renderHook(() => useMusic(), { wrapper: MusicProvider })

    await act(async () => {
      expect(await result.current.getArtist('ar1')).toEqual(artist)
      expect(await result.current.getAlbum('al1')).toEqual(album)
      expect(await result.current.fetchTracks('al1')).toEqual([track])
      expect(await result.current.createTrack(make({ title: 'Roygbiv' }))).toEqual(track)
      await result.current.removeTrack('tr1')
    })
    expect(mockApi.listAlbumTracks).toHaveBeenCalledWith('al1')
    expect(mockApi.deleteTrack).toHaveBeenCalledWith('tr1')
    expect(result.current.trackStreamUrl('tr1')).toBe('/api/tracks/tr1/stream?token=t')
  })
})

describe('AudiobooksProvider', () => {
  const book = make<Audiobook>({ id: 'b1', title: 'Dune' })

  it('fetch populates and error sets the message', async () => {
    mockApi.listAudiobooks.mockResolvedValue([book])
    const { result } = renderHook(() => useAudiobooks(), { wrapper: AudiobooksProvider })
    await act(async () => { await result.current.fetch('lib1') })
    expect(mockApi.listAudiobooks).toHaveBeenCalledWith({ library_id: 'lib1' })
    expect(result.current.audiobooks).toEqual([book])

    mockApi.listAudiobooks.mockRejectedValue(new ApiError(500, 'book-boom'))
    await act(async () => { await result.current.fetch() })
    expect(result.current.error).toBe('book-boom')
  })

  it('create appends, remove filters, chapter helpers delegate', async () => {
    mockApi.createAudiobook.mockResolvedValue(book)
    const chapter = make<AudiobookChapter>({ id: 'c1' })
    mockApi.listChapters.mockResolvedValue([chapter])
    mockApi.createChapter.mockResolvedValue(chapter)
    mockApi.chapterStreamUrl.mockReturnValue('/ch/stream')
    mockApi.deleteAudiobook.mockResolvedValue(undefined)
    const { result } = renderHook(() => useAudiobooks(), { wrapper: AudiobooksProvider })

    await act(async () => { await result.current.create(make({ title: 'Dune' })) })
    expect(result.current.audiobooks).toEqual([book])

    const bookEdited = make<Audiobook>({ id: 'b1', title: 'Dune (Unabridged)' })
    mockApi.updateAudiobook.mockResolvedValue(bookEdited)
    await act(async () => { await result.current.update('b1', make({ title: 'Dune (Unabridged)' })) })
    expect(result.current.audiobooks).toEqual([bookEdited])

    await act(async () => {
      expect(await result.current.fetchChapters('b1')).toEqual([chapter])
      expect(await result.current.createChapter('b1', make({}))).toEqual(chapter)
    })
    expect(mockApi.listChapters).toHaveBeenCalledWith('b1')
    expect(result.current.chapterStreamUrl('b1', 'c1')).toBe('/ch/stream')

    await act(async () => { await result.current.remove('b1') })
    expect(result.current.audiobooks).toEqual([])
  })
})

describe('WatchlistProvider', () => {
  const movieItem = make<WatchlistItem>({ id: 'wl1', media_type: 'movie', media_id: 'm1' })

  it('bootstraps items from getWatchlist on mount', async () => {
    mockApi.getWatchlist.mockResolvedValue([movieItem])
    const { result } = renderHook(() => useWatchlist(), { wrapper: WatchlistProvider })

    await waitFor(() => expect(result.current.items).toEqual([movieItem]))
    expect(result.current.isInWatchlist('movie', 'm1')).toBe(true)
    expect(result.current.isInWatchlist('movie', 'nope')).toBe(false)
    expect(result.current.findItem('movie', 'm1')).toEqual(movieItem)
  })

  it('toggle add: optimistic placeholder then replaced by the server item', async () => {
    mockApi.getWatchlist.mockResolvedValue([])
    const real = make<WatchlistItem>({ id: 'wl9', media_type: 'movie', media_id: 'm1' })
    const d = deferred<WatchlistItem>()
    mockApi.addToWatchlist.mockReturnValue(d.promise)
    const { result } = renderHook(() => useWatchlist(), { wrapper: WatchlistProvider })
    await waitFor(() => expect(result.current.items).toEqual([]))

    act(() => { result.current.toggle('movie', 'm1') })
    // optimistic placeholder present immediately
    expect(result.current.items).toHaveLength(1)
    expect(result.current.items[0].id).toBe('pending-movie-m1')
    expect(mockApi.addToWatchlist).toHaveBeenCalledWith('movie', 'm1')

    await act(async () => { d.resolve(real); await d.promise })
    expect(result.current.items).toEqual([real])
  })

  it('toggle add: rolls back the placeholder when the api rejects', async () => {
    mockApi.getWatchlist.mockResolvedValue([])
    const d = deferred<WatchlistItem>()
    mockApi.addToWatchlist.mockReturnValue(d.promise)
    const { result } = renderHook(() => useWatchlist(), { wrapper: WatchlistProvider })
    await waitFor(() => expect(result.current.items).toEqual([]))

    act(() => { result.current.toggle('movie', 'm1') })
    expect(result.current.items).toHaveLength(1)

    await act(async () => {
      d.reject(new ApiError(500, 'fail'))
      await d.promise.catch(() => {})
    })
    expect(result.current.items).toEqual([])
  })

  it('toggle remove: optimistic removal of an existing item', async () => {
    mockApi.getWatchlist.mockResolvedValue([movieItem])
    mockApi.removeFromWatchlist.mockResolvedValue(undefined)
    const { result } = renderHook(() => useWatchlist(), { wrapper: WatchlistProvider })
    await waitFor(() => expect(result.current.items).toEqual([movieItem]))

    await act(async () => { result.current.toggle('movie', 'm1') })

    expect(mockApi.removeFromWatchlist).toHaveBeenCalledWith('wl1')
    expect(result.current.items).toEqual([])
  })

  it('toggle remove: restores the item when the api rejects', async () => {
    mockApi.getWatchlist.mockResolvedValue([movieItem])
    const d = deferred<void>()
    mockApi.removeFromWatchlist.mockReturnValue(d.promise)
    const { result } = renderHook(() => useWatchlist(), { wrapper: WatchlistProvider })
    await waitFor(() => expect(result.current.items).toEqual([movieItem]))

    act(() => { result.current.toggle('movie', 'm1') })
    expect(result.current.items).toEqual([]) // optimistic removal

    await act(async () => {
      d.reject(new ApiError(500, 'fail'))
      await d.promise.catch(() => {})
    })
    expect(result.current.items).toEqual([movieItem]) // restored
  })
})
