# Installing River on a Fire TV / Firestick

River isn't in the Amazon Appstore, so you install it by **sideloading** — putting
the app on the device yourself. It's a one-time setup that takes about five minutes.
Once installed, River appears in **Your Apps & Channels** like any other app.

You only need two things:

- Your Fire TV or Firestick, on the **same network** as your River server.
- The address of your River server (the same one you open in a web browser),
  for example `https://river.thenerdsquad.co.uk`.

There are two ways to do it. **Method 1 (Downloader app)** needs no computer and is
the easiest for most people. **Method 2 (ADB)** is for those comfortable with a
command line.

---

## Method 1 — The Downloader app (no computer needed)

### Step 1: Allow apps from unknown sources

Fire TV blocks sideloading until you turn it on.

1. From the Fire TV home screen, go to **Settings** (the gear icon).
2. Open **My Fire TV** (on some devices this is **Device & Software** or **System**).
3. Select **Developer options**.
   - Don't see Developer options? Go to **My Fire TV → About**, highlight your
     device name, and press the **Select** (OK) button **seven times**. A
     "You are now a developer" message appears, and the menu unlocks.
4. Turn on **Apps from Unknown Sources** (on newer devices you enable it
   per-app — allow it for **Downloader** when prompted in Step 3).
5. If you see **ADB debugging**, you can leave it off for this method.

### Step 2: Install the Downloader app

1. From the home screen, open **Find → Search** (or press the microphone button).
2. Search for **Downloader** (the orange app by AFTVnews).
3. Select it and choose **Download** / **Get** to install it. It's free.

### Step 3: Download and install River

1. Open the **Downloader** app.
2. In the **Home / URL** box, type your River server address followed by
   `/river-tv.apk`. For example:

   ```
   https://river.thenerdsquad.co.uk/river-tv.apk
   ```

   > Tip: this is exactly the file the **Download** link on your River sign-in
   > page points to. Use the same address you'd type in a web browser, then add
   > `/river-tv.apk` to the end.

3. Select **Go**. Downloader fetches the APK (about 4–5 MB) and then
   automatically opens the Android installer.
4. On the install screen, select **Install**. If Fire TV prompts you to allow
   Downloader to install unknown apps, choose **Settings**, turn the toggle on,
   then go **Back** and select **Install** again.
5. When it finishes, choose **Done** (not "Open" yet).
6. Downloader will offer to **Delete** the downloaded APK file — choose
   **Delete**, then **Delete** again. You don't need the file anymore; this just
   frees up space.

### Step 4: Launch River

1. Go to the home screen → **Apps & Channels** (scroll down to **Your Apps &
   Channels**). River is at the bottom of the list.
2. Highlight **River**, press the **menu** button (☰) on the remote, and select
   **Move to front** so it's easy to find next time.
3. Open River. On first launch, enter your River **server address + /api** and sign in
   with your River username and password.

That's it. 🎉

---

## Method 2 — ADB from a computer (advanced)

If you have `adb` installed (part of the Android platform-tools) and prefer a
terminal:

1. On the Fire TV, enable **Settings → My Fire TV → Developer options → ADB
   debugging** (and **Apps from Unknown Sources**).
2. Find the Fire TV's IP address under **Settings → My Fire TV → About →
   Network**.
3. Download the APK from your River server (or use the copy you already have):

   ```bash
   curl -O https://river.thenerdsquad.co.uk/river-tv.apk
   ```

4. Connect and install:

   ```bash
   adb connect <fire-tv-ip>:5555
   adb install river-tv.apk
   ```

   The first `adb connect` pops up an **"Allow USB debugging?"** dialog on the
   TV — select **OK** (tick "Always allow" to skip it next time).

5. To upgrade later, install over the top with:

   ```bash
   adb install -r river-tv.apk
   ```

---

## Updating River

When a new version is released, repeat the same steps — the installer replaces
the existing app and keeps your server address and login. (With the Downloader
method, just enter the same `/river-tv.apk` URL again.)

## Troubleshooting

| Problem | Fix |
|---|---|
| **"App not installed"** | Almost always the *unknown sources* toggle. Re-check Step 1, and make sure you allowed **Downloader** (or the installer) specifically. |
| **Download fails / "couldn't parse URL"** | Double-check the address. It must start with `http://` and end with `/river-tv.apk`. Confirm the same URL loads in a browser on your phone. |
| **Downloaded a tiny file / an HTML page** | You reached the website but not the APK. Make sure you added `/river-tv.apk` to the end, with no trailing slash or extra path. |
| **App opens but can't reach the server** | The Fire TV must be on the **same network** as River. Enter the server address exactly as you'd type it in a browser plus `/api` (include the port if your server uses one, e.g. `http://river.thenerdsquad.co.uk/api`). |
| **Can't find River after installing** | It's under **Apps & Channels → Your Apps & Channels**, usually at the very bottom. Use **Move to front** to pin it. |

## Note on the "unknown sources" warning

Sideloading means Amazon didn't vet the app, so Fire TV warns you. River is your
own software from your own server — the warning is expected. If you'd rather not
leave sideloading enabled system-wide, you can turn **Apps from Unknown
Sources** back off after installing; River will keep working.
