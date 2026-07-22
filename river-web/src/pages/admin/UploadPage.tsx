import { useEffect, useRef, useState, type DragEvent, type FormEvent } from 'react'
import { RiUploadCloud2Line, RiCheckLine } from 'react-icons/ri'
import { useLibraries } from '../../context/LibrariesContext'
import { api, ApiError } from '../../api'
import type { UploadResult } from '../../api'
import styles from './UploadPage.module.css'

type MediaType = 'movie' | 'episode'

export function UploadPage() {
  const { libraries, fetch: fetchLibs } = useLibraries()

  const [libraryId, setLibraryId] = useState('')
  const [title, setTitle] = useState('')
  const [season, setSeason] = useState('')
  const [episode, setEpisode] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [fileKey, setFileKey] = useState(0)
  const [dragging, setDragging] = useState(false)

  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState(0)
  const [result, setResult] = useState<UploadResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const labelRef = useRef<HTMLLabelElement>(null)

  useEffect(() => { void fetchLibs() }, [fetchLibs])

  const eligible = libraries.filter(l => l.type === 'movie' || l.type === 'tvshow')
  const selectedLib = libraries.find(l => l.id === libraryId)
  const mediaType: MediaType | null = selectedLib?.type === 'movie' ? 'movie'
    : selectedLib?.type === 'tvshow' ? 'episode'
    : null

  function handleDrop(e: DragEvent<HTMLLabelElement>) {
    e.preventDefault()
    setDragging(false)
    const f = e.dataTransfer.files[0]
    if (f) setFile(f)
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setResult(null)

    if (!libraryId || !mediaType) { setError('Select a library'); return }
    if (!title.trim()) { setError('Title is required'); return }
    if (!file) { setError('Select a file to upload'); return }
    if (mediaType === 'episode') {
      if (!season || parseInt(season, 10) < 1) { setError('Season must be a positive number'); return }
      if (!episode || parseInt(episode, 10) < 1) { setError('Episode must be a positive number'); return }
    }

    const form = new FormData()
    form.set('type', mediaType)
    form.set('library_id', libraryId)
    form.set('title', title.trim())
    form.set('file', file)
    if (mediaType === 'episode') {
      form.set('season', season)
      form.set('episode', episode)
    }

    setUploading(true)
    setProgress(0)
    try {
      const res = await api.uploadMedia(form, pct => setProgress(pct))
      setResult(res)
      setTitle('')
      setSeason('')
      setEpisode('')
      setFile(null)
      setFileKey(k => k + 1)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Upload failed')
    } finally {
      setUploading(false)
      setProgress(0)
    }
  }

  return (
    <div className={styles.page}>
      <h1 className={`headline-lg ${styles.pageTitle}`}>Upload Media</h1>

      {result && (
        <div className={styles.successBanner}>
          <RiCheckLine size={18} />
          <span className="label-md">File uploaded — the scanner will process it shortly.</span>
        </div>
      )}

      <div className={`glass ${styles.card}`}>
        <form onSubmit={handleSubmit} className={styles.form} noValidate>

          {/* Library */}
          <div className={styles.field}>
            <label htmlFor="upload-library" className="label-md">Library</label>
            <select
              id="upload-library"
              className="input"
              value={libraryId}
              onChange={e => setLibraryId(e.target.value)}
            >
              <option value="">Select a library…</option>
              {eligible.map(lib => (
                <option key={lib.id} value={lib.id}>{lib.name}</option>
              ))}
            </select>
          </div>

          {/* Title */}
          <div className={styles.field}>
            <label htmlFor="upload-title" className="label-md">
              {mediaType === 'episode' ? 'Show Title' : 'Movie Title'}
            </label>
            <input
              id="upload-title"
              type="text"
              className="input"
              value={title}
              onChange={e => setTitle(e.target.value)}
              placeholder={mediaType === 'episode' ? 'e.g. Breaking Bad' : 'e.g. Inception'}
            />
          </div>

          {/* Season + Episode */}
          {mediaType === 'episode' && (
            <div className={styles.row2}>
              <div className={styles.field}>
                <label htmlFor="upload-season" className="label-md">Season</label>
                <input
                  id="upload-season"
                  type="number"
                  className="input"
                  value={season}
                  onChange={e => setSeason(e.target.value)}
                  min={1}
                  placeholder="1"
                />
              </div>
              <div className={styles.field}>
                <label htmlFor="upload-episode" className="label-md">Episode</label>
                <input
                  id="upload-episode"
                  type="number"
                  className="input"
                  value={episode}
                  onChange={e => setEpisode(e.target.value)}
                  min={1}
                  placeholder="1"
                />
              </div>
            </div>
          )}

          {/* File */}
          <div className={styles.field}>
            <span className="label-md">File</span>
            <label
              ref={labelRef}
              className={`${styles.dropZone} ${dragging ? styles.dropZoneHover : ''} ${file ? styles.dropZoneHasFile : ''}`}
              onDragOver={e => { e.preventDefault(); setDragging(true) }}
              onDragLeave={() => setDragging(false)}
              onDrop={handleDrop}
            >
              <RiUploadCloud2Line size={26} className={styles.dropIcon} />
              {file
                ? <span className={`label-md ${styles.fileName}`}>{file.name}</span>
                : <span className="label-md">Drag &amp; drop or click to browse</span>
              }
              <input
                key={fileKey}
                type="file"
                accept="video/*,.mkv,.avi"
                className={styles.fileInput}
                onChange={e => { const f = e.target.files?.[0]; if (f) setFile(f) }}
              />
            </label>
          </div>

          {error && <p className={styles.formError} role="alert">{error}</p>}

          {uploading && (
            <div className={styles.progressWrap}>
              <div className={styles.progressTrack}>
                <div
                  className={styles.progressFill}
                  style={{ width: `${Math.round(progress * 100)}%` }}
                />
              </div>
              <span className="label-sm" style={{ width: '36px', textAlign: 'right', color: 'var(--color-on-surface-variant)' }}>
                {Math.round(progress * 100)}%
              </span>
            </div>
          )}

          <div className={styles.formFooter}>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={uploading || !file || !libraryId || !mediaType || !title.trim()}
            >
              {uploading ? 'Uploading…' : 'Upload'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
