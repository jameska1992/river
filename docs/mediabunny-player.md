# Mediabunny playback engine (prototype)

**Status:** experimental, branch `feature/mediabunny-player`. Production watch pages
(`MovieWatchPage`, `EpisodeWatchPage`, …) are untouched and still use their own `<video>` code.
The only change outside the new code is one lazily-loaded route in `river-web/src/App.tsx`.

The question this prototype answers: **can Mediabunny do the hard media plumbing
(demux, decode, A/V sync, range-based streaming) while River owns the player experience?**
Short answer: **yes, for River's pre-transcoded H.264/AAC MP4 output**, in both Chromium and
Firefox. See [Results](#test-results) and [Recommendation](#recommendation).

## Why Mediabunny

- Pure TypeScript, no WASM, tree-shakeable; the debug page's lazy chunk is **74 KB gzipped**
  (Mediabunny + the player subsystem) and never loads on production pages.
- Reads MP4 with random access from any byte source, so River's existing HTTP Range endpoints
  work as-is. No HLS/DASH packaging and no server changes.
- Decodes through the browser's WebCodecs decoders, so we get native (potentially hardware)
  H.264/AAC decode without shipping codecs.
- Exposes every audio track in the file. River's transcoder already muxes **all** audio streams
  into the main MP4 (`river-video-trans/internal/transcoder/transcoder.go`, `-map 0:a:N`), so
  language switching needs no per-language variant files.

## Architecture

```
River React UI   (src/player/react/Player.tsx: controls, keyboard, fullscreen, subtitle overlay)
      │  usePlaybackState() ← useSyncExternalStore(engine.subscribe)
      ▼
PlaybackEngine   (src/player/core/PlaybackEngine.ts: backend-independent interface)
      │
 ┌────┴───────────────┐
 ▼                    ▼
MediabunnyEngine      NativeEngine (<video>, same mechanism as the production pages)
 │
 ├─ CanvasSink ── VideoDecoder (WebCodecs) ── frames ──▶ <canvas>
 └─ AudioBufferSink ── AudioDecoder (WebCodecs) ──▶ AudioBufferSourceNodes ─▶ GainNode ─▶ speakers
      │
      ▼
RiverInput  (src/player/mediabunny/RiverInput.ts: Mediabunny CustomSource over HTTP Range)
      │
      ▼
River API   GET /api/movies/:id/stream?token=…   (http.ServeContent → 206 Partial Content)
```

| Path (`river-web/src/player/`) | Responsibility |
|---|---|
| `core/types.ts` | `PlaybackState`, `PlaybackStatus`, `MediaTrackInfo`, `PlaybackSource`, errors and capabilities. Imports no backend. |
| `core/PlaybackEngine.ts` | Engine interface plus `EngineError` (`unsupported` / `network` / `auth` / `decode`). |
| `core/PlaybackStore.ts` | Tiny observable state container; React subscribes, never polls. |
| `core/PlaybackClock.ts` | Maps a time source (AudioContext or performance.now) to media time. |
| `core/selectEngine.ts` | Capability-ordered engine choice and fallback on `unsupported`. |
| `core/PlaybackRecovery.ts` | Engine-agnostic recovery (reload at position, backoff, `online`, wake-from-sleep). |
| `mediabunny/RiverInput.ts` | Range reader plus Mediabunny `CustomSource`. |
| `mediabunny/MediaTrackAdapter.ts` | Mediabunny tracks → `MediaTrackInfo` (labels, languages, 5.1 etc.). |
| `mediabunny/MediabunnyEngine.ts` | The WebCodecs pipeline, clocking, stalls and seeking. |
| `native/NativeEngine.ts` | `<video>` behind the same interface. |
| `subtitles/vtt.ts`, `react/SubtitleOverlay.tsx` | River's sidecar-VTT overlay, engine-independent. |
| `react/Player.tsx`, `react/DebugPanel.tsx` | Prototype UI and live diagnostics. |
| `riverSource.ts` | Builds a `PlaybackSource` from the River API client. |

Design rules: the UI never touches `HTMLVideoElement`, WebCodecs or Mediabunny; engines own their
render surface (`attach(container)`) and everything layered on top (subtitles, controls, a future
watch-party overlay) is the UI's job, so it is identical for both engines.

### RiverInput

`RiverRangeReader` turns each Mediabunny read into **exactly one** `Range: bytes=a-b` request:

- **Size:** a one-byte probe (`bytes=0-0`) reads the `Content-Range` total. River's stream
  routes are GET-only, so HEAD isn't available. The result is memoised; failures aren't.
- **Validation:** it requires `206`, a well-formed `Content-Range` whose start matches the
  request, and a body as long as promised. A short or truncated body is retried, and reads
  past EOF are clamped or rejected.
- **Never the whole file:** a `200` (Range ignored) is treated as `unsupported`. The body is
  cancelled rather than downloading a multi-GB movie.
- **Auth:** it uses the same stream token as the native player (`?token=`, rebuilt by
  `resolveUrl()` on every request). Unlike `<video>`, a 401 is visible to `fetch`, so the reader
  refreshes the token once and retries. Concurrent 401s share one refresh.
- **Resilience:** transient errors (network, 5xx) get 3 retries with exponential backoff.
  `dispose()` aborts every in-flight request (wired to `Input.dispose()`).
- **Read-ahead:** the `CustomSource` uses Mediabunny's `'network'` prefetch profile with a
  32 MiB cache, which coalesces sequential reads and reads ahead during linear playback.

In practice, loading a 389 MB / 20-minute 1080p file fetched **8.4 MB in 6 requests**
(`moov` plus the first GOP).

### Clocking, A/V sync and buffering

- **Audio is the master clock.** `PlaybackClock` runs on `AudioContext.currentTime`. Each decoded
  AAC buffer is scheduled at the exact context time its timestamp maps to, so audio is
  sample-accurate by construction. Video frames come from `CanvasSink.canvases(t)` and a rAF loop
  draws whichever frame is due.
- **Output latency compensation.** Sound leaves the speakers `outputLatency` after it is
  scheduled (measured: Chromium ≈72–80 ms, Firefox ≈42–51 ms headless; Bluetooth is typically
  150–250 ms). Video is presented against `clock − latency`. Browsers revise the value after the
  device opens (Chromium reports 0, then 72 ms ~250 ms later), so the applied value slews at
  2 ms/frame instead of stepping. Position, progress and subtitles follow the uncompensated
  clock.
- **Backpressure:** audio is decoded at most 2 s ahead, which also covers background-tab timer
  throttling.
- **Buffering:** if the next frame is >250 ms late, or scheduled audio runs dry, the engine
  enters a stall. It suspends the AudioContext, which freezes the clock and every scheduled
  node, so A/V stay aligned. It resumes once a future frame and ≥0.5 s of audio are ready. The UI
  sees `buffering`.
- **Late video:** frames that are late are dropped. If decode falls >1 s behind (e.g. after a
  frozen tab), the decoder is re-seeked to the playhead instead of decoding every stale frame.

### Seeking

`seek(t)` stops the clock at `t`, stops audio, restarts the video iterator at `t` and resolves
once the frame at `t` is on the canvas. If the player was playing, it then restarts audio from
`t` and the clock, so audio and video restart from the same instant. Each async loop carries a
generation number, so a burst of seeks cancels the stale ones and the **last seek wins**. Seeks
are frame-accurate: Mediabunny decodes from the preceding keyframe to `t` over Range requests.

### Native fallback

`engineOrder('auto', …)` tries Mediabunny when `VideoDecoder` exists, then native.
`loadWithFallback` destroys an engine that rejects with `EngineError('unsupported')` (no
WebCodecs, track not decodable, server without Range) and tries the next. Other errors
(network, auth) are **not** fallen back on, because that would hide the real fault.
`?engine=native|mediabunny` forces one.

`NativeEngine` mirrors the production page. Audio switching uses `AudioTrackList` where it
exists (Safari) and otherwise reloads onto River's per-language variant file at the current
position. It also exposes `.element` for PiP and casting, which need a real `<video>`.

### Recovery

Two layers:

1. **Inside the engine:** `RiverInput` absorbs blips (retries, token refresh). `engine.reload()`
   rebuilds the whole pipeline at a remembered position and play state. The resume point is kept
   across failed reloads, so a recovery attempt made while offline doesn't lose the position.
2. **Above the engine:** `PlaybackRecovery` (engine-agnostic) calls `reload()` on a recoverable
   error. It retries with exponential backoff (8 s doubling to 60 s), retries immediately on the
   browser `online` event, and reloads when a tab returns visible but the position isn't moving.
   This is the abstraction River's `useMediaRecovery` behaviour should move onto.

### Subtitles

Mediabunny's input side doesn't expose subtitle tracks for MP4, and River stores subtitles as
sidecar WebVTT anyway. `SubtitleOverlay` fetches and parses the VTT (a copy of the parser
private to `MovieWatchPage`; merge them on migration) and renders the cue for
`state.position` above whatever surface the engine draws. Nothing is burned into frames.

## Running the prototype

```bash
cd river-web
RIVER_API_TARGET=http://localhost:8080 npm run dev
# log in as an admin, then open:
#   http://localhost:5173/debug/mediabunny-player
#   http://localhost:5173/debug/mediabunny-player?movie=<id>&engine=auto|mediabunny|native
```

The page is admin-only and not linked from the UI. It provides:

- a movie picker, engine selector and subtitle selector
- scripted seek buttons (0, 50%, end−5 s, a burst of 8 concurrent seeks) and "Reload pipeline"
- an event log covering engine choice, fallback reasons and recoveries
- a debug panel showing: status, position, buffer ahead, codecs and resolution, audio
  layout, WebCodecs availability, H.264/AAC decodability, Range request count, bytes fetched vs
  file size, dropped frames, measured A/V offset, latency compensation and late audio starts

`window.__riverPlayer` holds the active engine for devtools and automation.

Unit tests: `npm test` (104 tests under `src/player/`).

## Test results

Measured 2026-10-07 against the local docker stack, through River's real `/api/movies/:id/stream`
endpoint, headless, using a scripted Playwright harness (28 checks per run).

**Media**

| File | Description |
|---|---|
| Fixture | The 10 local library movies: 30.5 s, 1280×720 H.264 High, AAC-LC stereo, 9.8 MB, 3 s GOP. |
| Sync test | 20 min, 1920×1080p24 H.264 High (`avc1.640028`), 389 MB. Two AAC-LC tracks (English stereo, French 5.1). Burnt-in timecode, plus a white flash and a beep on every second. Produced with river-video-trans's exact `buildArgs` output (stream-copy remux to MP4 `+faststart`). |
| Long GOP | 3 min 1080p with libx264's default 250-frame (10.4 s) GOP, for worst-case seeks. |

**Browsers**

| Browser | Mediabunny engine | Native engine |
|---|---|---|
| Chromium 153 (Playwright) | **28/28** | 24/24 |
| Chromium 152 (Arch system build), basic suite | **22/22** | — |
| Firefox 155 (Playwright) | **27/27** (no CDP tab-freeze test) | 23/23 |
| Safari/WebKit | not tested (no WebKit build available) | — |

All three report WebCodecs decode support for H.264 High@4.0 and for AAC-LC stereo and 5.1.

**Results, 1080p sync test**

| Area | Chromium | Firefox |
|---|---|---|
| Load to first frame | 0.82 s, 8.4 MB of 389 MB read | 1.2 s, same bytes |
| Seek while paused / playing, forward / backward / large | 28–75 ms | 35–88 ms |
| 8 concurrent seeks | last target wins (±0.05 s) | same |
| Worst-case seek, 10 s GOP (Mediabunny vs native) | 394 vs 357 ms median | 386 vs 364 ms median |
| 60 s drift vs wall clock | ≤ 0.18 s (within the 250 ms position-publish granularity) | ≤ 0.11 s |
| A/V offset (presented frame vs heard audio) | mean 10 ms, max 37 ms | mean 9 ms, max 21 ms |
| Dropped frames over 60 s | 0–3 | 0 |
| Audio track switch (English → French 5.1) | 9–31 ms, video uninterrupted | 14 ms |
| Pause/resume, volume, mute, fullscreen, EOF to `ended`, replay | pass | pass |
| 10 s offline mid-playback | absorbed by read-ahead | absorbed by read-ahead |
| 15 s offline during a seek | `buffering → error(network)`, auto-resumes at the seek target on reconnect | same |
| Tab frozen 10 s (CDP lifecycle freeze) | resumes in sync | n/a |
| Browser refresh | clean reload | clean reload |
| Range-only streaming | never a Range-less request | same |

A/V offset is the presented frame's timestamp minus the audio clock at draw time. Anything under
one frame (41.7 ms at 24 fps) is the best a frame-based renderer can do. Headless CI has no real
audio device, so the ear-and-eye check (beep vs flash on the sync file) should be repeated on a
desktop with speakers and with Bluetooth headphones.

## Bugs found by the live tests

| Bug | Where | Status |
|---|---|---|
| Recovery attempted while offline lost position and play state, then stayed stuck in `error` | prototype engines and `PlaybackRecovery` | Fixed: resume point kept across failed reloads, backoff retries, `online` trigger |
| Firefox reports a mid-playback network failure as `MEDIA_ERR_SRC_NOT_SUPPORTED`, which was classified as non-recoverable | `NativeEngine` | Fixed: code 4 means "unsupported" only before the first `loadedmetadata` |
| Chromium revises `outputLatency` from 0 to ~72 ms after start, which froze a few frames | `MediabunnyEngine` | Fixed: compensation slews |
| **`RiverClient.doRefresh()` clears the whole session on *any* failure, including a network error.** A token refresh attempted while offline logs the user out (verified in a browser: all three tokens removed). | **production** `src/api/client.ts:251`, reached via `useMediaRecovery` on wake-from-sleep, often before Wi-Fi reconnects | **Not fixed here** (shared auth code, out of scope for the prototype). The prototype avoids it by skipping the pre-emptive refresh while `navigator.onLine` is false. Recommended fix: only `clearAuth()` when `/auth/refresh` itself returns 401/403. |

## Known limitations and unsupported functionality

- **Formats:** MP4/MOV container, H.264 video and AAC audio only, by design. Anything else
  falls back to native.
- **Playback rate:** unsupported in the Mediabunny engine (`capabilities.playbackRate = false`).
  WebAudio buffer playback has no pitch-preserving time-stretch.
- **Picture-in-picture:** not available from a canvas. A possible route is
  `canvas.captureStream()` into a hidden `<video>`. Use native for now.
- **Casting / Chromecast** needs a URL, not a canvas; it stays on native or URL-based.
- **Autoplay:** an AudioContext needs a user gesture. `play()` without one rejects with a clear
  error instead of silently showing frozen video.
- **Hidden tabs:** rAF stops, so video stops painting while audio keeps playing (the same as
  `<video>`). It resyncs within one keyframe seek on return.
- **Safari:** untested. It has no `outputLatency` (falls back to `baseLatency`, which
  under-compensates). WebCodecs `AudioDecoder` AAC support should be verified.
- **Hardware decode:** headless test browsers decoded in software. GPU decode and CPU/battery
  cost on real desktops, TVs and mobiles are unmeasured.
- **Memory:** 32 MiB byte cache, 3 pooled 1080p canvases (~25 MB) and ≤2 s of decoded audio.
- **Buffering visibility:** Mediabunny's byte cache is opaque. `bufferedAhead` reports decoded
  read-ahead (≤2 s of audio), not downloaded bytes.
- **Seek latency scales with GOP length.** It's at parity with native, but ~400 ms on 10 s GOPs.
  If River ever wants instant scrubbing, a shorter keyint in river-video-trans helps both
  engines.
- **Not built** (out of scope): watch-party sync, progress reporting, continue-watching, casting,
  thumbnails, HLS/DASH, DRM, TV remote handling.

## Recommendation

1. **Mediabunny is viable** as River's custom-player pipeline for its transcode format. Seeking,
   sync, drift, multi-audio, streaming efficiency and recovery all match or beat the native
   element in Chromium and Firefox, and audio-track switching is genuinely better (in-file,
   instant, no variant reload).
2. **Keep the native engine as a permanent fallback.** It's needed where WebCodecs is
   missing or a track isn't decodable, and for PiP, casting and source-file playback.
3. **The `PlaybackEngine` boundary suits a `MovieWatchPage` migration.** Do it in two steps:
   first move the page onto `Player` + `NativeEngine` with no behaviour change, then flip
   `auto` on behind a setting. Progress reporting, watch-party commands and resume-from-progress
   map onto `state.position`, `seek()`, `play()`, `pause()` and `subscribe()`.
4. **Before migrating:**
   - fix the `doRefresh` logout bug (it affects production today);
   - do an audible A/V check on real hardware, including Bluetooth latency;
   - test Safari/iOS;
   - measure CPU and battery against native on a TV-class device;
   - decide on PiP and casting for the Mediabunny engine;
   - port watch-party sync onto the engine interface.

Nothing found so far would justify a custom Go/WASM component.
