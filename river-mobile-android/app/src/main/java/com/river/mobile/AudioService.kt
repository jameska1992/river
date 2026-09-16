package com.river.mobile

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.graphics.drawable.Icon
import android.media.MediaMetadata
import android.media.session.MediaSession
import android.media.session.PlaybackState
import android.os.Build
import android.os.IBinder

/**
 * Foreground service that keeps river-mobile's audio playing while the app is
 * backgrounded / the screen is off, and surfaces lock-screen + notification
 * transport controls.
 *
 * It does NOT play audio itself — the sound is the WebView's <audio> element
 * (river-mobile's player). This service (a) holds the process in the foreground
 * so the WebView isn't suspended, and (b) mirrors the web Media Session state
 * into a framework [MediaSession] + MediaStyle notification, routing control
 * taps (notification buttons, lock-screen, headset) back into the web player
 * via [commandListener].
 *
 * Framework MediaSession is used (not androidx.media) so no extra Gradle
 * dependency is needed. Driven by the JS bridge (window.RiverNative) through
 * MainActivity, which forwards metadata / state as intents.
 */
class AudioService : Service() {
    private lateinit var session: MediaSession

    private var title = ""
    private var artist = ""
    private var album = ""
    private var durationMs = 0L
    private var positionMs = 0L
    private var playing = false

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createChannel()
        session = MediaSession(this, "RiverAudio").apply {
            setCallback(object : MediaSession.Callback() {
                override fun onPlay() = dispatch(Command.Play)
                override fun onPause() = dispatch(Command.Pause)
                override fun onStop() = dispatch(Command.Pause)
                override fun onSkipToNext() = dispatch(Command.Next)
                override fun onSkipToPrevious() = dispatch(Command.Prev)
                override fun onSeekTo(pos: Long) = dispatch(Command.SeekTo(pos))
            })
            isActive = true
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> { stop(); return START_NOT_STICKY }
            ACTION_META -> {
                title = intent.getStringExtra(EXTRA_TITLE).orEmpty()
                artist = intent.getStringExtra(EXTRA_ARTIST).orEmpty()
                album = intent.getStringExtra(EXTRA_ALBUM).orEmpty()
                durationMs = intent.getLongExtra(EXTRA_DURATION, 0L)
            }
            ACTION_STATE -> {
                playing = intent.getBooleanExtra(EXTRA_PLAYING, false)
                positionMs = intent.getLongExtra(EXTRA_POSITION, 0L)
            }
            // Notification-button taps: forward to the web player and update
            // our local state optimistically so the icon flips immediately.
            ACTION_PLAY -> { dispatch(Command.Play); playing = true }
            ACTION_PAUSE -> { dispatch(Command.Pause); playing = false }
            ACTION_NEXT -> dispatch(Command.Next)
            ACTION_PREV -> dispatch(Command.Prev)
        }

        session.setMetadata(
            MediaMetadata.Builder()
                .putString(MediaMetadata.METADATA_KEY_TITLE, title)
                .putString(MediaMetadata.METADATA_KEY_ARTIST, artist)
                .putString(MediaMetadata.METADATA_KEY_ALBUM, album)
                .putLong(MediaMetadata.METADATA_KEY_DURATION, durationMs)
                .build(),
        )
        session.setPlaybackState(
            PlaybackState.Builder()
                .setActions(
                    PlaybackState.ACTION_PLAY or PlaybackState.ACTION_PAUSE or
                        PlaybackState.ACTION_PLAY_PAUSE or PlaybackState.ACTION_SEEK_TO or
                        PlaybackState.ACTION_SKIP_TO_NEXT or PlaybackState.ACTION_SKIP_TO_PREVIOUS,
                )
                .setState(
                    if (playing) PlaybackState.STATE_PLAYING else PlaybackState.STATE_PAUSED,
                    positionMs,
                    if (playing) 1f else 0f,
                )
                .build(),
        )

        startForeground(NOTIF_ID, buildNotification())
        return START_NOT_STICKY
    }

    private fun stop() {
        session.isActive = false
        session.release()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } else {
            @Suppress("DEPRECATION") stopForeground(true)
        }
        stopSelf()
    }

    private fun dispatch(command: Command) {
        commandListener?.invoke(command)
    }

    private fun buildNotification(): Notification {
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            @Suppress("DEPRECATION") Notification.Builder(this)
        }

        val playPause = if (playing) {
            action(android.R.drawable.ic_media_pause, "Pause", ACTION_PAUSE)
        } else {
            action(android.R.drawable.ic_media_play, "Play", ACTION_PLAY)
        }

        return builder
            .setContentTitle(title.ifEmpty { getString(R.string.app_name) })
            .setContentText(listOf(artist, album).filter { it.isNotEmpty() }.joinToString("  ·  "))
            .setSmallIcon(android.R.drawable.ic_media_play)
            .setContentIntent(contentIntent())
            .setVisibility(Notification.VISIBILITY_PUBLIC)
            .setOngoing(playing)
            .addAction(action(android.R.drawable.ic_media_previous, "Previous", ACTION_PREV))
            .addAction(playPause)
            .addAction(action(android.R.drawable.ic_media_next, "Next", ACTION_NEXT))
            .setStyle(
                Notification.MediaStyle()
                    .setMediaSession(session.sessionToken)
                    .setShowActionsInCompactView(0, 1, 2),
            )
            .build()
    }

    private fun action(icon: Int, title: String, action: String): Notification.Action {
        val pi = PendingIntent.getService(
            this,
            action.hashCode(),
            Intent(this, AudioService::class.java).setAction(action),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        return Notification.Action.Builder(Icon.createWithResource(this, icon), title, pi).build()
    }

    private fun contentIntent(): PendingIntent {
        val intent = Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP)
        return PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
    }

    private fun createChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(CHANNEL_ID, "Playback", NotificationManager.IMPORTANCE_LOW).apply {
                setShowBadge(false)
                lockscreenVisibility = Notification.VISIBILITY_PUBLIC
            }
            (getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager).createNotificationChannel(channel)
        }
    }

    /** A transport command originating from the OS media UI. */
    sealed class Command {
        data object Play : Command()
        data object Pause : Command()
        data object Next : Command()
        data object Prev : Command()
        data class SeekTo(val positionMs: Long) : Command()
    }

    companion object {
        // Same-process handoff: MainActivity registers a listener that routes
        // OS media commands back into the WebView's audio player.
        var commandListener: ((Command) -> Unit)? = null

        const val ACTION_META = "com.river.mobile.audio.META"
        const val ACTION_STATE = "com.river.mobile.audio.STATE"
        const val ACTION_STOP = "com.river.mobile.audio.STOP"
        const val ACTION_PLAY = "com.river.mobile.audio.PLAY"
        const val ACTION_PAUSE = "com.river.mobile.audio.PAUSE"
        const val ACTION_NEXT = "com.river.mobile.audio.NEXT"
        const val ACTION_PREV = "com.river.mobile.audio.PREV"

        const val EXTRA_TITLE = "title"
        const val EXTRA_ARTIST = "artist"
        const val EXTRA_ALBUM = "album"
        const val EXTRA_DURATION = "durationMs"
        const val EXTRA_PLAYING = "playing"
        const val EXTRA_POSITION = "positionMs"

        private const val CHANNEL_ID = "river_playback"
        private const val NOTIF_ID = 1001
    }
}
