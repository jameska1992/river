# river-mobile-android

Android phone/tablet app that ships the [river-mobile](../river-mobile/) web build inside the APK and runs it in a full-screen WebView. The phone counterpart to [`river-tv-android`](../river-tv-android/).

- The Vite build is packaged as APK assets and served under `http://appassets.androidplatform.net/` via `WebViewAssetLoader` — no external web server is required and no first-run URL prompt is needed. HTTP (not HTTPS) is deliberate: river-api on a LAN is almost always plain HTTP, and matching schemes avoids WebView's inconsistent mixed-content handling.
- The hardware **Back** button walks the WebView's history (`goBack()`), which — because react-router pushes real history entries — steps back through the app (detail → list, exit the video route, …). At the root of history a second Back within 2 s exits the app. (This differs from the TV shell, which maps Back → Escape for its D-pad focus manager.)
- Orientation is portrait-first but not locked, so the device can rotate and the in-app video player can go landscape while it's on screen.
- **Background audio**: while music/audiobooks are loaded, a `mediaPlayback` foreground service (`AudioService`) keeps the WebView alive so playback continues with the screen off / app backgrounded, and a framework `MediaSession` + MediaStyle notification surfaces lock-screen and notification transport controls. The web player talks to it through the `window.RiverNative` bridge (start/stop, metadata, play/pause state); control taps route back into the web player via `window.__riverAudio`. The API server URL is configured **inside** river-mobile (login screen), not here.

## Build

Requires `npm` on PATH — the Android build invokes it to produce the bundled `river-mobile/dist/` before packaging.

```bash
gradle wrapper          # one-time, populates gradle/wrapper/gradle-wrapper.jar
./gradlew assembleDebug # APK at app/build/outputs/apk/debug/app-debug.apk
```

Under the hood the build runs three tasks in order:

| Task | What it does |
|---|---|
| `riverMobileNpmInstall` | `npm ci` in `../river-mobile/` |
| `riverMobileBuild` | `npm run build` (Vite → `../river-mobile/dist/`) |
| `riverMobileSyncAssets` | Copies `dist/` into `app/build/generated/river-mobile-webapp/webapp/`, registered as an assets source dir so `mergeAssets` bundles it into the APK. |

All three declare Gradle inputs/outputs so they're skipped on incremental builds when nothing under `river-mobile/src/` (or its config files / `package-lock.json`) has changed.

Sideload to a phone with ADB:

```bash
adb install app/build/outputs/apk/debug/app-debug.apk
```

## Configuration

There is no build-time URL — the JS app is embedded. On first run, enter your river-api server's address on the login screen (the default points at `localhost`, which on a phone means the phone itself). river-mobile persists that setting in its own storage (localStorage), scoped to the `appassets.androidplatform.net` origin.

To wipe the API URL and any other river-mobile state, clear the app's data from the device's app settings.

## Devices

- **Phones / tablets** — `android.intent.category.LAUNCHER` puts the app in the launcher. No TV leanback intent or banner (that's `river-tv-android`).

## Replace before shipping

- `app/src/main/res/mipmap-*/ic_launcher.png` currently reuses the shared River mark — swap for a phone-tuned adaptive icon before publishing.
- `applicationId` and `versionCode` / `versionName` in `app/build.gradle.kts`.
