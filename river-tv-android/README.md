# river-tv-android

Android TV / Fire TV launcher app that ships the [river-tv](../river-tv/) web build inside the APK and runs it in a full-screen WebView.

- The Vite build is packaged as APK assets and served under `http://appassets.androidplatform.net/` via `WebViewAssetLoader` — no external web server is required and no first-run URL prompt is needed. HTTP (not HTTPS) is deliberate: river-api on a LAN is almost always plain HTTP, and matching schemes avoids Fire TV WebView's inconsistent mixed-content handling.
- D-pad / OK keys reach the WebView as normal keydown events and river-tv's `FocusProvider` handles them.
- The TV remote's **Back** button is forwarded to JS as an Escape keydown so popups close, the player exits, and the sidebar takes focus exactly like in the browser. A second Back press within 2 s quits the app.
- The API server URL is configured **inside** river-tv (login screen), not here.

## Build

Requires `npm` on PATH — the Android build invokes it to produce the bundled `river-tv/dist/` before packaging.

```bash
gradle wrapper          # one-time, populates gradle/wrapper/gradle-wrapper.jar
./gradlew assembleDebug # APK at app/build/outputs/apk/debug/app-debug.apk
```

Under the hood the build runs three tasks in order:

| Task | What it does |
|---|---|
| `riverTvNpmInstall` | `npm ci` in `../river-tv/` |
| `riverTvBuild` | `npm run build` (Vite → `../river-tv/dist/`) |
| `riverTvSyncAssets` | Copies `dist/` into `app/build/generated/river-tv-webapp/webapp/`, which is registered as an assets source dir so `mergeAssets` bundles it into the APK. |

All three have proper Gradle inputs/outputs so they're skipped on incremental builds when nothing under `river-tv/src/` (or its config files / `package-lock.json`) has changed.

Sideload to a TV with ADB:

```bash
adb connect <tv-ip>:5555
adb install app/build/outputs/apk/debug/app-debug.apk
```

## Configuration

There is no build-time URL — the JS app is embedded. To point at a different river-api server, log out of river-tv inside the app and enter a new API URL in the login screen. river-tv persists that setting in its own storage (localStorage), scoped to the `appassets.androidplatform.net` origin.

To wipe the API URL and any other river-tv state, clear the app's data from the TV's app settings.

## Devices

- **Android TV** — the `android.intent.category.LEANBACK_LAUNCHER` filter on `MainActivity` puts the app in the TV home rail.
- **Fire TV** — same launcher intent; nothing Fire-specific needed.
- **Phones / tablets** — the manifest also includes `LAUNCHER` so the app installs and runs there too (useful for dev). Touchscreen + Leanback are both declared *not required*.

## Replace before shipping

- `app/src/main/res/drawable/ic_launcher.xml` and `banner.xml` are placeholder vectors. Replace with rasterised PNGs (`mipmap-*dpi/ic_launcher.png` for the icon; a 320×180 png for the banner) before publishing.
- `applicationId` and `versionCode` / `versionName` in `app/build.gradle.kts`.
