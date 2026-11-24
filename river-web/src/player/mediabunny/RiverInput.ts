import { CustomSource } from 'mediabunny'
import { EngineError } from '../core/PlaybackEngine'

/**
 * Byte-range reader for River's stream endpoints.
 *
 * River serves media through Go's http.ServeContent, so every stream URL
 * supports `Range: bytes=a-b` → `206 Partial Content` with `Content-Range`.
 * Mediabunny asks for byte ranges as it demuxes (moov first, then the sample
 * data around the playhead), and this reader turns each ask into exactly one
 * Range request. It never falls back to fetching the whole file: a server that
 * ignores Range is reported as an error instead.
 *
 * Authentication matches the native <video> player — the stream token rides
 * in the URL's `token` query param (built fresh by `resolveUrl` on every
 * request). Unlike <video>, a fetch can react to a 401, so an expired stream
 * token (e.g. after an 8h+ pause) is refreshed and the request retried.
 */

export interface RangeRequestInfo {
  start: number
  /** Exclusive. */
  end: number
  status: number
  bytes: number
  ms: number
}

export interface RiverRangeReaderOptions {
  resolveUrl: () => string
  refreshAuth?: () => Promise<void>
  fetchFn?: typeof fetch
  /** Retries for transient failures (network errors, 5xx). Default 3. */
  maxRetries?: number
  /** Delay before retry `attempt` (1-based), in ms. Default 250ms × 2^(attempt-1). */
  retryDelayMs?: (attempt: number) => number
  onRequest?: (info: RangeRequestInfo) => void
}

export interface RangeReaderStats {
  requestCount: number
  bytesFetched: number
  fileSize: number | null
}

const CONTENT_RANGE_RE = /^bytes\s+(\d+)-(\d+)\/(\d+|\*)$/i

export interface ContentRange { start: number; end: number; total: number | null }

/** Parses `Content-Range: bytes 0-99/1234` (end inclusive). Returns null if malformed. */
export function parseContentRange(header: string | null): ContentRange | null {
  if (!header) return null
  const m = CONTENT_RANGE_RE.exec(header.trim())
  if (!m) return null
  const start = Number(m[1])
  const end = Number(m[2])
  if (end < start) return null
  return { start, end, total: m[3] === '*' ? null : Number(m[3]) }
}

function isAbort(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}

export class RiverRangeReader {
  private readonly opts: Required<Omit<RiverRangeReaderOptions, 'refreshAuth' | 'onRequest'>> &
    Pick<RiverRangeReaderOptions, 'refreshAuth' | 'onRequest'>
  private readonly controller = new AbortController()
  private sizePromise: Promise<number> | null = null
  private authRefresh: Promise<void> | null = null
  private readonly stats: RangeReaderStats = { requestCount: 0, bytesFetched: 0, fileSize: null }

  constructor(options: RiverRangeReaderOptions) {
    this.opts = {
      fetchFn: (...args) => fetch(...args),
      maxRetries: 3,
      retryDelayMs: attempt => 250 * 2 ** (attempt - 1),
      ...options,
    }
  }

  get disposed(): boolean { return this.controller.signal.aborted }

  getStats(): RangeReaderStats { return { ...this.stats } }

  /**
   * Total file size, learned from a one-byte probe (`bytes=0-0`) — the
   * Content-Range total. River's stream routes are GET-only, so HEAD isn't an
   * option. Memoised.
   */
  getSize(): Promise<number> {
    if (!this.sizePromise) {
      this.sizePromise = this.request(0, 1).then(({ range }) => {
        if (range.total === null) throw new EngineError('network', 'Server did not report the media size (Content-Range total is "*")', { recoverable: false })
        this.stats.fileSize = range.total
        return range.total
      })
      // Don't memoise failures — a later call should be able to retry.
      this.sizePromise.catch(() => { this.sizePromise = null })
    }
    return this.sizePromise
  }

  /** Reads bytes [start, end). */
  async read(start: number, end: number): Promise<Uint8Array> {
    if (end <= start) return new Uint8Array(0)
    const size = await this.getSize()
    const clampedEnd = Math.min(end, size)
    if (start >= size) throw new EngineError('network', `Read past EOF: ${start} >= ${size}`, { recoverable: false })
    const { data } = await this.request(start, clampedEnd)
    return data
  }

  /** Aborts every in-flight request; all later reads reject. */
  dispose(): void {
    this.controller.abort()
  }

  private async request(start: number, end: number): Promise<{ data: Uint8Array; range: ContentRange }> {
    let attempt = 0
    let refreshed = false
    for (;;) {
      if (this.disposed) throw new DOMException('Reader disposed', 'AbortError')
      try {
        return await this.requestOnce(start, end)
      } catch (err) {
        if (isAbort(err) || this.disposed) throw err
        if (err instanceof EngineError && err.code === 'auth' && !refreshed && this.opts.refreshAuth) {
          refreshed = true
          await this.refreshAuthOnce()
          continue
        }
        const transient = !(err instanceof EngineError) || (err.code === 'network' && err.recoverable)
        if (!transient || attempt >= this.opts.maxRetries) {
          if (err instanceof EngineError) throw err
          throw new EngineError('network', `Media request failed: ${err instanceof Error ? err.message : String(err)}`, { cause: err })
        }
        attempt++
        await this.sleep(this.opts.retryDelayMs(attempt))
      }
    }
  }

  // Concurrent reads that all 401 share a single refresh.
  private refreshAuthOnce(): Promise<void> {
    if (!this.authRefresh) {
      this.authRefresh = this.opts.refreshAuth!().finally(() => { this.authRefresh = null })
    }
    return this.authRefresh
  }

  private async requestOnce(start: number, end: number): Promise<{ data: Uint8Array; range: ContentRange }> {
    const t0 = performance.now()
    const res = await this.opts.fetchFn(this.opts.resolveUrl(), {
      headers: { Range: `bytes=${start}-${end - 1}` },
      signal: this.controller.signal,
      // Byte ranges of an immutable transcode — let the HTTP cache help on re-seeks.
      cache: 'default',
    })
    this.stats.requestCount++

    if (res.status === 401 || res.status === 403) {
      void res.body?.cancel()
      throw new EngineError('auth', `Stream request unauthorised (${res.status})`, { recoverable: false })
    }
    if (res.status === 416) {
      void res.body?.cancel()
      throw new EngineError('network', `Range ${start}-${end - 1} not satisfiable (416)`, { recoverable: false })
    }
    if (res.status === 200) {
      // The server ignored Range and is sending the whole file. Abort the body
      // rather than download a multi-GB movie into memory.
      void res.body?.cancel()
      throw new EngineError('unsupported', 'Server ignored the Range header (200 instead of 206); progressive range playback needs Range support', { recoverable: false })
    }
    if (res.status >= 500) {
      void res.body?.cancel()
      throw new EngineError('network', `Stream request failed (${res.status})`, { recoverable: true })
    }
    if (res.status !== 206) {
      void res.body?.cancel()
      throw new EngineError('network', `Unexpected stream response ${res.status}`, { recoverable: false })
    }

    const range = parseContentRange(res.headers.get('Content-Range'))
    if (!range) {
      void res.body?.cancel()
      throw new EngineError('network', `Missing or malformed Content-Range: ${res.headers.get('Content-Range')}`, { recoverable: false })
    }
    if (range.start !== start) {
      void res.body?.cancel()
      throw new EngineError('network', `Content-Range starts at ${range.start}, requested ${start}`, { recoverable: false })
    }

    const data = new Uint8Array(await res.arrayBuffer())
    // The server may legitimately return fewer bytes than asked when the
    // request runs past EOF (ServeContent clamps the range). Anything shorter
    // than what Content-Range promised is a truncated response — retryable.
    const promised = range.end - range.start + 1
    if (data.byteLength < promised) {
      throw new EngineError('network', `Truncated range response: got ${data.byteLength} of ${promised} bytes`, { recoverable: true })
    }
    if (range.end < end - 1 && (range.total === null || range.end + 1 < range.total)) {
      throw new EngineError('network', `Short range response: ${range.start}-${range.end} for ${start}-${end - 1}`, { recoverable: true })
    }

    this.stats.bytesFetched += data.byteLength
    this.opts.onRequest?.({ start, end, status: res.status, bytes: data.byteLength, ms: performance.now() - t0 })
    return { data, range }
  }

  private sleep(ms: number): Promise<void> {
    return new Promise((resolve, reject) => {
      const signal = this.controller.signal
      const timer = setTimeout(() => { signal.removeEventListener('abort', onAbort); resolve() }, ms)
      const onAbort = () => { clearTimeout(timer); reject(new DOMException('Reader disposed', 'AbortError')) }
      signal.addEventListener('abort', onAbort, { once: true })
    })
  }
}

/** 32 MiB of demuxed-byte cache; enough for several seconds of 1080p either side of the playhead. */
const CACHE_BYTES = 32 * 1024 * 1024

/**
 * Wraps a RiverRangeReader as a Mediabunny Source. The `'network'` prefetch
 * profile makes Mediabunny coalesce sequential reads into larger requests and
 * read ahead during linear playback, while random access (seeks) still only
 * fetches what's needed around the target.
 */
export function createRiverSource(reader: RiverRangeReader): CustomSource {
  return new CustomSource({
    getSize: () => reader.getSize(),
    read: (start, end) => reader.read(start, end),
    dispose: () => reader.dispose(),
    prefetchProfile: 'network',
    maxCacheSize: CACHE_BYTES,
    // Reads that fail while nothing is awaiting them (prefetch) would otherwise
    // become unhandled rejections; the next awaited read re-surfaces the error.
    handleUnhandledError: () => {},
  })
}
