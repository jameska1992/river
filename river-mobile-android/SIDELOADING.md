# Installing River on an Android phone / tablet

River isn't on the Play Store, so you install it by **sideloading** — downloading
the app and installing it yourself. It's a one-time setup that takes a couple of
minutes.

You only need two things:

- Your phone or tablet, on the **same network** as your River server.
- The address of your River server (the same one you open in a web browser),
  for example `http://192.168.1.10:8080` or `https://river.example.com`.

There are two ways to do it. **Method 1 (browser)** needs no computer and is the
easiest for most people. **Method 2 (ADB)** is for those comfortable with a
command line.

---

## Method 1 — Download in the browser (no computer needed)

1. On the phone, open your River server's **`/download`** page in a browser
   (e.g. `https://river.example.com/download`).
2. Tap **Android phone / tablet** to download `river-mobile.apk`.
   - You can also go straight to `…/river-mobile.apk`.
3. Open the downloaded file (from the download notification or your Files app).
4. Android will warn that installs from this source are blocked — tap
   **Settings**, enable **Allow from this source** for your browser (or Files
   app), then go **back** and tap **Install**.
5. Open **River**. On the login screen, enter your server address (the same URL
   you use in a browser) and sign in.

---

## Method 2 — ADB from a computer (advanced)

If you have `adb` installed (part of the Android platform-tools):

1. On the phone, enable **Settings → About phone → tap Build number 7×** to
   unlock **Developer options**, then turn on **USB debugging**.
   - On Samsung devices, also turn **off** **Settings → Security and privacy →
     Auto Blocker**, which otherwise blocks ADB.
2. Download the APK (or use the copy you already have):

   ```bash
   curl -O https://river.example.com/river-mobile.apk
   ```

3. Connect the phone by USB (accept the **"Allow USB debugging?"** prompt) and
   install:

   ```bash
   adb install river-mobile.apk
   ```

4. To upgrade later, install over the top:

   ```bash
   adb install -r river-mobile.apk
   ```

---

## Updating River

When a new version is released, repeat the same steps — the installer replaces
the existing app and keeps your server address and login. (With the browser
method, just download `/river-mobile.apk` again and open it.)

## Troubleshooting

| Problem | Fix |
|---|---|
| **"App not installed" / blocked** | The *unknown sources* permission. Re-check Method 1 step 4 and allow your browser (or Files app) to install apps. |
| **Downloaded a tiny file / an HTML page** | You reached the website but not the APK. Make sure the URL ends in `/river-mobile.apk`, with no trailing slash. |
| **App opens but can't reach the server** | The phone must be on the **same network** as River. Enter the server address exactly as you'd type it in a browser, including `/api` and the port if any — e.g. `http://192.168.1.10:8080/api`. |
| **ADB doesn't see the phone (Samsung)** | Disable **Auto Blocker** (Settings → Security and privacy → Auto Blocker); it blocks USB/wireless debugging. |

## Note on the "unknown sources" warning

Sideloading means Google didn't vet the app, so Android warns you. River is your
own software from your own server — the warning is expected.
