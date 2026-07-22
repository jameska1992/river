import { Link } from 'react-router-dom'
import { RiPlayCircleFill, RiDownload2Line } from 'react-icons/ri'
import styles from './AuthPage.module.css'

// Public page (no auth) for downloading the river-tv-android APK. The file is
// served statically by nginx from /river-tv.apk (see river-web/public/).
export function DownloadPage() {
  return (
    <div className={styles.page}>
      <div className={styles.glow} aria-hidden />

      <div className={`glass ${styles.card}`}>
        <div className={styles.wordmark}>
          <RiPlayCircleFill className={styles.wordmarkIcon} aria-hidden />
          <span>River</span>
        </div>

        <h1 className={`headline-md ${styles.heading}`}>River for Android TV</h1>

        <p className={`label-md ${styles.blurb}`}>
          Install the River app on your Android TV or Fire TV device to stream your
          library on the big screen.
        </p>

        <a
          href="/river-tv.apk"
          download="river-tv.apk"
          className={`btn btn-primary ${styles.downloadBtn}`}
        >
          <RiDownload2Line className={styles.downloadIcon} aria-hidden />
          Download APK
        </a>

        <p className={`label-sm ${styles.hint}`}>
          You may need to allow installs from unknown sources on your device.
        </p>

        <p className={`label-sm ${styles.footer}`}>
          <Link to="/login" className={styles.link}>Back to sign in</Link>
        </p>
      </div>
    </div>
  )
}
