import { describe, expect, it, vi } from 'vitest'
import { EngineError } from '../core/PlaybackEngine'
import { RiverRangeReader, createRiverSource, parseContentRange } from './RiverInput'

const FILE = Uint8Array.from({ length: 1000 }, (_, i) => i % 256)

interface FakeOpts {
  /** Override the response for a given call (0-based). */
  override?: (call: number, range: [number, number]) => Response | Error | undefined
}

/** A fake River stream endpoint backed by FILE, honouring Range like http.ServeContent. */
function fakeServer(opts: FakeOpts = {}) {
  let call = 0
  const calls: { url: string; range: string | null }[] = []
  const fetchFn = vi.fn(async (url: RequestInfo | URL, init?: RequestInit) => {
    const range = new Headers(init?.headers).get('Range')
    calls.push({ url: String(url), range })
    const m = /bytes=(\d+)-(\d+)/.exec(range ?? '')!
    const start = Number(m[1])
    const end = Math.min(Number(m[2]), FILE.length - 1)
    const o = opts.override?.(call++, [start, end])
    if (o instanceof Error) throw o
    if (o) return o
    if (start >= FILE.length) return new Response(null, { status: 416 })
    return new Response(FILE.slice(start, end + 1), {
      status: 206,
      headers: { 'Content-Range': `bytes ${start}-${end}/${FILE.length}` },
    })
  })
  return { fetchFn: fetchFn as unknown as typeof fetch, calls, spy: fetchFn }
}

function reader(server: ReturnType<typeof fakeServer>, extra: Partial<ConstructorParameters<typeof RiverRangeReader>[0]> = {}) {
  let token = 'tok1'
  return new RiverRangeReader({
    resolveUrl: () => `/api/movies/m1/stream?token=${token}`,
    refreshAuth: async () => { token = 'tok2' },
    fetchFn: server.fetchFn,
    retryDelayMs: () => 0,
    ...extra,
  })
}

describe('parseContentRange', () => {
  it('parses a full header', () => {
    expect(parseContentRange('bytes 0-99/1234')).toEqual({ start: 0, end: 99, total: 1234 })
  })
  it('accepts an unknown total', () => {
    expect(parseContentRange('bytes 5-9/*')).toEqual({ start: 5, end: 9, total: null })
  })
  it('rejects malformed or inverted ranges', () => {
    expect(parseContentRange(null)).toBeNull()
    expect(parseContentRange('bytes */1234')).toBeNull()
    expect(parseContentRange('bytes 9-5/10')).toBeNull()
    expect(parseContentRange('items 0-1/2')).toBeNull()
  })
})

describe('RiverRangeReader', () => {
  it('learns the size from a one-byte Range probe and memoises it', async () => {
    const s = fakeServer()
    const r = reader(s)
    expect(await r.getSize()).toBe(1000)
    expect(await r.getSize()).toBe(1000)
    expect(s.calls).toHaveLength(1)
    expect(s.calls[0].range).toBe('bytes=0-0')
  })

  it('reads an exact byte range with an inclusive Range header', async () => {
    const s = fakeServer()
    const r = reader(s)
    const data = await r.read(100, 200)
    expect(data).toEqual(FILE.slice(100, 200))
    expect(s.calls.at(-1)!.range).toBe('bytes=100-199')
    expect(r.getStats()).toMatchObject({ requestCount: 2, bytesFetched: 1 + 100, fileSize: 1000 })
  })

  it('clamps reads that run past EOF', async () => {
    const s = fakeServer()
    const data = await reader(s).read(990, 2000)
    expect(data).toEqual(FILE.slice(990, 1000))
    expect(s.calls.at(-1)!.range).toBe('bytes=990-999')
  })

  it('rejects a read that starts at or past EOF without a request', async () => {
    const s = fakeServer()
    const r = reader(s)
    await expect(r.read(1000, 1010)).rejects.toThrow(/past EOF/)
    expect(s.calls).toHaveLength(1) // only the size probe
  })

  it('returns an empty array for an empty range', async () => {
    expect(await reader(fakeServer()).read(5, 5)).toHaveLength(0)
  })

  it('never downloads the whole file when the server ignores Range (200)', async () => {
    const cancel = vi.fn()
    const s = fakeServer({
      override: () => {
        const body = new ReadableStream({ cancel })
        return new Response(body, { status: 200 })
      },
    })
    const err = await reader(s).getSize().catch(e => e)
    expect(err).toBeInstanceOf(EngineError)
    expect(err.code).toBe('unsupported')
    expect(cancel).toHaveBeenCalled()
  })

  it('refreshes auth once on 401 and retries with the new token', async () => {
    const s = fakeServer({ override: c => (c === 0 ? new Response(null, { status: 401 }) : undefined) })
    expect(await reader(s).getSize()).toBe(1000)
    expect(s.calls[0].url).toContain('token=tok1')
    expect(s.calls[1].url).toContain('token=tok2')
  })

  it('fails with an auth error if the refreshed token is also rejected', async () => {
    const s = fakeServer({ override: () => new Response(null, { status: 401 }) })
    const err = await reader(s).getSize().catch(e => e)
    expect(err.code).toBe('auth')
    expect(s.calls).toHaveLength(2)
  })

  it('shares one auth refresh between concurrent 401s', async () => {
    const refreshAuth = vi.fn(async () => {})
    const s = fakeServer({ override: c => (c < 3 ? new Response(null, { status: 401 }) : undefined) })
    const r = reader(s, { refreshAuth })
    await r.getSize().catch(() => {})
    await Promise.all([r.read(0, 10), r.read(10, 20)])
    expect(refreshAuth.mock.calls.length).toBeLessThanOrEqual(2)
  })

  it('retries transient network errors and 5xx with backoff', async () => {
    const delays: number[] = []
    const s = fakeServer({
      override: c => (c === 0 ? new TypeError('Failed to fetch') : c === 1 ? new Response(null, { status: 503 }) : undefined),
    })
    const r = reader(s, { retryDelayMs: a => { delays.push(a); return 0 } })
    expect(await r.getSize()).toBe(1000)
    expect(delays).toEqual([1, 2])
  })

  it('gives up after maxRetries and reports a network error', async () => {
    const s = fakeServer({ override: () => new TypeError('Failed to fetch') })
    const err = await reader(s, { maxRetries: 2 }).getSize().catch(e => e)
    expect(err).toBeInstanceOf(EngineError)
    expect(err.code).toBe('network')
    expect(s.calls).toHaveLength(3)
  })

  it('does not memoise a failed size probe', async () => {
    const s = fakeServer({ override: c => (c === 0 ? new Response(null, { status: 404 }) : undefined) })
    const r = reader(s)
    await expect(r.getSize()).rejects.toThrow(/404/)
    expect(await r.getSize()).toBe(1000)
  })

  it('rejects a response whose Content-Range starts elsewhere', async () => {
    const s = fakeServer({
      override: c => (c === 1 ? new Response(FILE.slice(0, 10), { status: 206, headers: { 'Content-Range': 'bytes 0-9/1000' } }) : undefined),
    })
    await expect(reader(s).read(50, 60)).rejects.toThrow(/starts at 0/)
  })

  it('rejects a missing Content-Range', async () => {
    const s = fakeServer({ override: () => new Response(new Uint8Array(1), { status: 206 }) })
    await expect(reader(s).getSize()).rejects.toThrow(/Content-Range/)
  })

  it('rejects an unknown total size', async () => {
    const s = fakeServer({ override: () => new Response(new Uint8Array(1), { status: 206, headers: { 'Content-Range': 'bytes 0-0/*' } }) })
    await expect(reader(s).getSize()).rejects.toThrow(/size/)
  })

  it('retries a truncated body', async () => {
    const s = fakeServer({
      override: c => (c === 1 ? new Response(FILE.slice(0, 5), { status: 206, headers: { 'Content-Range': 'bytes 0-9/1000' } }) : undefined),
    })
    expect(await reader(s).read(0, 10)).toEqual(FILE.slice(0, 10))
    expect(s.calls).toHaveLength(3)
  })

  it('treats 416 as a non-retryable error', async () => {
    const s = fakeServer({ override: c => (c === 1 ? new Response(null, { status: 416 }) : undefined) })
    await expect(reader(s).read(0, 10)).rejects.toThrow(/416/)
    expect(s.calls).toHaveLength(2)
  })

  it('aborts in-flight requests on dispose', async () => {
    let seenSignal: AbortSignal | undefined
    const fetchFn = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
      seenSignal = init?.signal ?? undefined
      return new Promise<Response>((_, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
      })
    }) as unknown as typeof fetch
    const r = new RiverRangeReader({ resolveUrl: () => '/x', fetchFn })
    const p = r.getSize()
    r.dispose()
    await expect(p).rejects.toThrow(/aborted/)
    expect(seenSignal?.aborted).toBe(true)
    expect(r.disposed).toBe(true)
    await expect(r.read(0, 1)).rejects.toThrow()
  })

  it('reports each completed request', async () => {
    const onRequest = vi.fn()
    await reader(fakeServer(), { onRequest }).read(0, 4)
    expect(onRequest).toHaveBeenCalledWith(expect.objectContaining({ start: 0, end: 4, status: 206, bytes: 4 }))
  })
})

describe('createRiverSource', () => {
  it('exposes the reader as a Mediabunny source', async () => {
    const r = reader(fakeServer())
    const src = createRiverSource(r)
    expect(await src.getSize()).toBe(1000)
  })
})
