import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, RiverClient } from './client'

// --- fetch mocking ---------------------------------------------------------
//
// RiverClient talks to the server exclusively through the global `fetch`, so
// every test drives it by queueing the Response(s) the next call(s) should
// return and asserting on the captured request (method / url / headers / body).
// No real network, per the issue's acceptance criteria.

type Captured = { url: string; init: RequestInit }

let calls: Captured[] = []
let responses: Array<Response | Error> = []

function queue(...rs: Array<Response | Error>) {
  responses.push(...rs)
}

function json(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
}

// Pull the single captured request (asserts exactly one fetch happened).
function lastCall(): Captured {
  expect(calls).toHaveLength(1)
  return calls[0]
}

beforeEach(() => {
  calls = []
  responses = []
  localStorage.clear()
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init: RequestInit = {}) => {
    calls.push({ url: String(input), init })
    const next = responses.shift()
    if (!next) throw new Error(`unexpected fetch: ${init.method ?? 'GET'} ${String(input)}`)
    if (next instanceof Error) return Promise.reject(next)
    return Promise.resolve(next)
  }))
})

afterEach(() => {
  vi.unstubAllGlobals()
})

// Small helpers to read what a captured call sent.
const headersOf = (c: Captured) => new Headers(c.init.headers)
const bodyOf = (c: Captured) => (c.init.body ? JSON.parse(c.init.body as string) : undefined)

describe('request() core', () => {
  it('issues a GET with no body and no Content-Type, prefixed by the base path', async () => {
    queue(json({ ok: true }))
    const client = new RiverClient()

    await client.getStats()

    const c = lastCall()
    expect(c.url).toBe('/api/admin/stats')
    expect(c.init.method).toBe('GET')
    expect(c.init.body).toBeUndefined()
    expect(headersOf(c).has('Content-Type')).toBe(false)
  })

  it('serializes a JSON body and sets Content-Type on writes', async () => {
    queue(json({ tmdb_api_key: 'k' }))
    const client = new RiverClient()

    await client.updateMetadataSettings('secret-key')

    const c = lastCall()
    expect(c.url).toBe('/api/admin/settings/metadata')
    expect(c.init.method).toBe('PUT')
    expect(headersOf(c).get('Content-Type')).toBe('application/json')
    expect(bodyOf(c)).toEqual({ tmdb_api_key: 'secret-key' })
  })

  it('parses and returns the JSON response body', async () => {
    queue(json({ movies: 3, tv_shows: 1, tracks: 0, audiobooks: 2 }))
    const client = new RiverClient()

    const stats = await client.getStats()

    expect(stats).toEqual({ movies: 3, tv_shows: 1, tracks: 0, audiobooks: 2 })
  })

  it('returns undefined for a 204 No Content response', async () => {
    queue(new Response(null, { status: 204 }))
    const client = new RiverClient()

    const result = await client.triggerScan()

    expect(result).toBeUndefined()
  })

  it('honours a custom base URL', async () => {
    queue(json({}))
    const client = new RiverClient('https://example.test/v1')

    await client.getStats()

    expect(lastCall().url).toBe('https://example.test/v1/admin/stats')
  })

  it('builds a query string from defined params and drops empty ones', async () => {
    queue(json([]))
    const client = new RiverClient()

    await client.listMovies({ library_id: 'lib1', page: 2, limit: undefined, sort: '' })

    const url = new URL(lastCall().url, 'http://x')
    expect(url.pathname).toBe('/api/movies')
    expect(url.searchParams.get('library_id')).toBe('lib1')
    expect(url.searchParams.get('page')).toBe('2')
    expect(url.searchParams.has('limit')).toBe(false)
    expect(url.searchParams.has('sort')).toBe(false)
  })
})

describe('auth header injection', () => {
  it('adds Authorization: Bearer when an access token is stored', async () => {
    localStorage.setItem('river:access_token', 'abc123')
    queue(json({ id: 'u1' }))
    const client = new RiverClient()

    await client.me()

    expect(headersOf(lastCall()).get('Authorization')).toBe('Bearer abc123')
  })

  it('omits Authorization when no access token is stored', async () => {
    queue(json({ id: 'u1' }))
    const client = new RiverClient()

    await client.me()

    expect(headersOf(lastCall()).has('Authorization')).toBe(false)
  })
})

describe('token storage', () => {
  it('login persists access/refresh/stream tokens and flips isAuthenticated', async () => {
    queue(json({
      access_token: 'a', refresh_token: 'r', stream_token: 's',
      user: { id: 'u1' },
    }))
    const client = new RiverClient()
    expect(client.isAuthenticated).toBe(false)

    const res = await client.login('neo', 'pw')

    const c = lastCall()
    expect(c.url).toBe('/api/auth/login')
    expect(c.init.method).toBe('POST')
    expect(bodyOf(c)).toEqual({ username: 'neo', password: 'pw' })
    expect(res.access_token).toBe('a')
    expect(client.accessToken).toBe('a')
    expect(client.refreshToken).toBe('r')
    expect(client.streamToken).toBe('s')
    expect(client.isAuthenticated).toBe(true)
  })

  it('clearAuth removes every stored token', () => {
    localStorage.setItem('river:access_token', 'a')
    localStorage.setItem('river:refresh_token', 'r')
    localStorage.setItem('river:stream_token', 's')
    const client = new RiverClient()

    client.clearAuth()

    expect(client.accessToken).toBeNull()
    expect(client.refreshToken).toBeNull()
    expect(client.streamToken).toBeNull()
    expect(client.isAuthenticated).toBe(false)
  })
})

describe('refresh-on-401', () => {
  it('refreshes the token on a 401 then retries the original request with the new token', async () => {
    localStorage.setItem('river:access_token', 'old')
    localStorage.setItem('river:refresh_token', 'r-old')
    const client = new RiverClient()

    queue(
      // 1. original request → 401
      new Response(JSON.stringify({ error: 'expired' }), { status: 401 }),
      // 2. POST /auth/refresh → new tokens
      json({ access_token: 'new', refresh_token: 'r-new', stream_token: 's-new' }),
      // 3. retried original request → success
      json({ id: 'u1' }),
    )

    const user = await client.me()

    expect(user).toEqual({ id: 'u1' })
    expect(calls).toHaveLength(3)
    expect(calls[0].url).toBe('/api/auth/me')
    expect(calls[1].url).toBe('/api/auth/refresh')
    expect(bodyOf(calls[1])).toEqual({ refresh_token: 'r-old' })
    // retry carries the refreshed access token
    expect(headersOf(calls[2]).get('Authorization')).toBe('Bearer new')
    expect(client.accessToken).toBe('new')
    expect(client.refreshToken).toBe('r-new')
  })

  it('does not attempt a refresh when the login call itself returns 401', async () => {
    queue(new Response(JSON.stringify({ error: 'bad credentials' }), { status: 401 }))
    const client = new RiverClient()

    await expect(client.login('neo', 'wrong')).rejects.toMatchObject({ status: 401 })
    // only the login attempt — no /auth/refresh follow-up
    expect(calls).toHaveLength(1)
    expect(calls[0].url).toBe('/api/auth/login')
  })

  it('clears auth and throws when refreshing without a refresh token', async () => {
    localStorage.setItem('river:access_token', 'old')
    const client = new RiverClient()
    queue(new Response(JSON.stringify({ error: 'expired' }), { status: 401 }))

    await expect(client.me()).rejects.toBeInstanceOf(ApiError)
    expect(client.isAuthenticated).toBe(false)
    // original request 401'd, but with no refresh token we never hit /auth/refresh
    expect(calls).toHaveLength(1)
  })

  it('clears auth and surfaces a session-expired error when the refresh call fails', async () => {
    localStorage.setItem('river:access_token', 'old')
    localStorage.setItem('river:refresh_token', 'r-old')
    const client = new RiverClient()
    queue(
      new Response(JSON.stringify({ error: 'expired' }), { status: 401 }),
      new Response(JSON.stringify({ error: 'nope' }), { status: 401 }),
    )

    await expect(client.me()).rejects.toMatchObject({
      status: 401,
      message: 'Session expired, please log in again',
    })
    expect(client.isAuthenticated).toBe(false)
  })
})

describe('ApiError mapping', () => {
  it('maps a non-2xx JSON error body to ApiError with status and message', async () => {
    queue(new Response(JSON.stringify({ error: 'already exists' }), { status: 409 }))
    const client = new RiverClient()

    await expect(client.getStats()).rejects.toMatchObject({
      name: 'ApiError',
      status: 409,
      message: 'already exists',
    })
  })

  it('falls back to statusText when the error body is not JSON', async () => {
    queue(new Response('<html>oops</html>', { status: 500, statusText: 'Internal Server Error' }))
    const client = new RiverClient()

    await expect(client.getStats()).rejects.toMatchObject({
      status: 500,
      message: 'Internal Server Error',
    })
  })

  it('propagates a network error (fetch rejection) without wrapping it', async () => {
    queue(new TypeError('Failed to fetch'))
    const client = new RiverClient()

    await expect(client.getStats()).rejects.toThrow('Failed to fetch')
  })
})

describe('representative endpoints (path / verb / body / transforms)', () => {
  it('me → GET /auth/me', async () => {
    queue(json({ id: 'u1', username: 'neo' }))
    const client = new RiverClient()

    await client.me()

    expect(lastCall().url).toBe('/api/auth/me')
    expect(lastCall().init.method).toBe('GET')
  })

  it('listMovies decodes the JSON-encoded genres string into an array', async () => {
    queue(json([{ id: 'm1', title: 'Heat', genres: '["Crime","Drama"]' }]))
    const client = new RiverClient()

    const movies = await client.listMovies()

    expect(movies[0].genres).toEqual(['Crime', 'Drama'])
  })

  it('listMovies tolerates a malformed genres string by falling back to []', async () => {
    queue(json([{ id: 'm1', title: 'Heat', genres: 'not-json' }]))
    const client = new RiverClient()

    const movies = await client.listMovies()

    expect(movies[0].genres).toEqual([])
  })

  it('getTVShow → GET /tvshows/:id and decodes genres', async () => {
    queue(json({ id: 't1', title: 'Wire', genres: '["Drama"]' }))
    const client = new RiverClient()

    const show = await client.getTVShow('t1')

    expect(lastCall().url).toBe('/api/tvshows/t1')
    expect(show.genres).toEqual(['Drama'])
  })

  it('createMovie → POST /movies and serializes genres back to a JSON string', async () => {
    queue(json({ id: 'm9', title: 'New', genres: '["Action"]' }))
    const client = new RiverClient()

    const created = await client.createMovie({ title: 'New', genres: ['Action'] } as never)

    const c = lastCall()
    expect(c.url).toBe('/api/movies')
    expect(c.init.method).toBe('POST')
    expect(bodyOf(c)).toMatchObject({ title: 'New', genres: '["Action"]' })
    // response is decoded again on the way back out
    expect(created.genres).toEqual(['Action'])
  })

  it('getMetadataSettings → GET /admin/settings/metadata', async () => {
    queue(json({ tmdb_api_key: 'k' }))
    const client = new RiverClient()

    await client.getMetadataSettings()

    expect(lastCall().url).toBe('/api/admin/settings/metadata')
    expect(lastCall().init.method).toBe('GET')
  })

  it('createWebhook → POST /admin/webhooks returning the one-time secret', async () => {
    queue(json({ secret: 'whsec_x', webhook: { id: 'w1' } }))
    const client = new RiverClient()

    const res = await client.createWebhook({ name: 'n', url: 'https://h', events: ['media.ready.movie'] })

    const c = lastCall()
    expect(c.url).toBe('/api/admin/webhooks')
    expect(c.init.method).toBe('POST')
    expect(bodyOf(c)).toEqual({ name: 'n', url: 'https://h', events: ['media.ready.movie'] })
    expect(res.secret).toBe('whsec_x')
  })

  it('mintAPIToken → POST /admin/api-tokens returning the one-time token', async () => {
    queue(json({ token: 'tok_plain', api_token: { id: 'at1' } }))
    const client = new RiverClient()

    const res = await client.mintAPIToken({ name: 'ci', scopes: ['read'] })

    const c = lastCall()
    expect(c.url).toBe('/api/admin/api-tokens')
    expect(c.init.method).toBe('POST')
    expect(res.token).toBe('tok_plain')
  })
})

describe('requestPaged (X-Total-Count)', () => {
  it('reads the total from the X-Total-Count header', async () => {
    queue(json([{ id: 'm1', genres: '[]' }], { headers: { 'X-Total-Count': '42' } }))
    const client = new RiverClient()

    const { items, total } = await client.listMoviesPaged()

    expect(items).toHaveLength(1)
    expect(total).toBe(42)
  })

  it('falls back to the page length when the header is absent', async () => {
    queue(json([{ id: 'm1', genres: '[]' }, { id: 'm2', genres: '[]' }]))
    const client = new RiverClient()

    const { total } = await client.listMoviesPaged()

    expect(total).toBe(2)
  })
})

describe('streamUrl token embedding', () => {
  it('prefers the stream token in the query string', () => {
    localStorage.setItem('river:stream_token', 's-tok')
    localStorage.setItem('river:access_token', 'a-tok')
    const client = new RiverClient()

    const url = new URL(client.subtitleStreamUrl('sub1'), 'http://x')
    expect(url.pathname).toBe('/api/subtitles/sub1/stream')
    expect(url.searchParams.get('token')).toBe('s-tok')
  })

  it('falls back to the access token when no stream token is present', () => {
    localStorage.setItem('river:access_token', 'a-tok')
    const client = new RiverClient()

    const url = new URL(client.subtitleStreamUrl('sub1'), 'http://x')
    expect(url.searchParams.get('token')).toBe('a-tok')
  })
})
