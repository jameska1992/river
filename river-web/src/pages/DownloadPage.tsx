import { Link } from 'react-router-dom'
import { RiPlayCircleFill, RiDownload2Line, RiTvLine, RiSmartphoneLine } from 'react-icons/ri'
import styles from './AuthPage.module.css'

// Public page (no auth) for downloading the River Android apps. The APKs are
// served statically by nginx from /river-tv.apk and /river-mobile.apk (see
// river-web/public/).
export function DownloadPage() {
  return (
    <div className={styles.page}>
      <div className={styles.glow} aria-hidden />

      <div className={`glass ${styles.card}`}>
        <div className={styles.wordmark}>
          <RiPlayCircleFill className={styles.wordmarkIcon} aria-hidden />
          <span>River</span>
        </div>

        <h1 className={`headline-md ${styles.heading}`}>Get River for Android</h1>

        <p className={`label-md ${styles.blurb}`}>
          Install River on your device to stream your library. Pick the build for
          your device below.
        </p>

        <a
          href="/river-tv.apk"
          download="river-tv.apk"
          className={`btn btn-primary ${styles.downloadBtn}`}
        >
          <RiTvLine className={styles.downloadIcon} aria-hidden />
          Android TV / Fire TV
          <RiDownload2Line className={styles.downloadIcon} aria-hidden />
        </a>

        <a
          href="/river-mobile.apk"
          download="river-mobile.apk"
          className={`btn btn-primary ${styles.downloadBtn}`}
        >
          <RiSmartphoneLine className={styles.downloadIcon} aria-hidden />
          Android phone / tablet
          <RiDownload2Line className={styles.downloadIcon} aria-hidden />
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
