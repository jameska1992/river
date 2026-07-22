import { useEffect, useRef, useState } from 'react'
import { RiCloseLine } from 'react-icons/ri'
import type { Movie, TVShow, Audiobook, Artist, Album } from '../api'
import { api } from '../api'
import styles from './MetadataModal.module.css'

type Props =
  | { type: 'movie';     item: Movie;     onSave: (updated: Movie) => void;     onClose: () => void }
  | { type: 'tvshow';    item: TVShow;    onSave: (updated: TVShow) => void;     onClose: () => void }
  | { type: 'audiobook'; item: Audiobook; onSave: (updated: Audiobook) => void; onClose: () => void }
  | { type: 'artist';    item: Artist;    onSave: (updated: Artist) => void;    onClose: () => void }
  | { type: 'album';     item: Album;     onSave: (updated: Album) => void;     onClose: () => void }

export function MetadataModal(props: Props) {
  const { type, item, onClose } = props
  const overlayRef = useRef<HTMLDivElement>(null)

  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Movie / TVShow / Audiobook / Album fields
  const [title, setTitle] = useState(type !== 'artist' ? (item as Movie).title ?? '' : '')
  const [description, setDescription] = useState(
    (type === 'movie' || type === 'tvshow' || type === 'audiobook') ? (item as Movie).description ?? '' : ''
  )
  const [year, setYear] = useState(
    (type !== 'artist') ? String((item as Movie | Album).year || '') : ''
  )

  // Movie / TVShow fields
  const [originalTitle, setOriginalTitle] = useState(
    (type === 'movie' || type === 'tvshow') ? (item as Movie).original_title ?? '' : ''
  )
  const [genres, setGenres] = useState(
    (type === 'movie' || type === 'tvshow') ? ((item as Movie).genres ?? []).join(', ') : ''
  )
  const [rating, setRating] = useState(
    (type === 'movie' || type === 'tvshow') ? String((item as Movie).rating || '') : ''
  )
  const [posterPath, setPosterPath] = useState(
    (type === 'movie' || type === 'tvshow') ? (item as Movie).poster_path ?? '' : ''
  )
  const [backdropPath, setBackdropPath] = useState(
    (type === 'movie' || type === 'tvshow') ? (item as Movie).backdrop_path ?? '' : ''
  )

  // Movie-only
  const [runtime, setRuntime] = useState(
    type === 'movie' ? String((item as Movie).runtime || '') : ''
  )

  // TVShow-only
  const [status, setStatus] = useState(
    type === 'tvshow' ? (item as TVShow).status ?? '' : ''
  )

  // Audiobook-only
  const [author, setAuthor] = useState(
    type === 'audiobook' ? (item as Audiobook).author ?? '' : ''
  )
  const [narrator, setNarrator] = useState(
    type === 'audiobook' ? (item as Audiobook).narrator ?? '' : ''
  )

  // Audiobook / Album shared
  const [genre, setGenre] = useState(
    (type === 'audiobook' || type === 'album') ? (item as Audiobook | Album).genre ?? '' : ''
  )
  const [coverPath, setCoverPath] = useState(
    (type === 'audiobook' || type === 'album') ? (item as Audiobook | Album).cover_path ?? '' : ''
  )

  // Artist-only
  const [name, setName] = useState(type === 'artist' ? (item as Artist).name ?? '' : '')
  const [bio, setBio] = useState(type === 'artist' ? (item as Artist).bio ?? '' : '')
  const [imagePath, setImagePath] = useState(type === 'artist' ? (item as Artist).image_path ?? '' : '')

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const parseList = (s: string) => s.split(',').map(x => x.trim()).filter(Boolean)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError(null)
    try {
      if (type === 'movie') {
        const updated = await api.updateMovie(item.id, {
          library_id: item.library_id,
          title,
          original_title: originalTitle,
          description,
          year: parseInt(year) || 0,
          genres: parseList(genres),
          rating: parseFloat(rating) || 0,
          runtime: parseInt(runtime) || 0,
          poster_path: posterPath,
          backdrop_path: backdropPath,
          file_path: (item as Movie).file_path,
        })
        props.onSave(updated)
      } else if (type === 'tvshow') {
        const updated = await api.updateTVShow(item.id, {
          library_id: item.library_id,
          title,
          original_title: originalTitle,
          description,
          year: parseInt(year) || 0,
          status,
          genres: parseList(genres),
          rating: parseFloat(rating) || 0,
          poster_path: posterPath,
          backdrop_path: backdropPath,
        })
        props.onSave(updated)
      } else if (type === 'audiobook') {
        const updated = await api.updateAudiobook(item.id, {
          library_id: item.library_id,
          title,
          author,
          narrator,
          description,
          year: parseInt(year) || 0,
          genre,
          cover_path: coverPath,
          duration: (item as Audiobook).duration,
        })
        props.onSave(updated)
      } else if (type === 'artist') {
        const updated = await api.updateArtist(item.id, {
          library_id: (item as Artist).library_id,
          name,
          bio,
          image_path: imagePath,
        })
        props.onSave(updated)
      } else {
        const updated = await api.updateAlbum(item.id, {
          library_id: (item as Album).library_id,
          artist_id: (item as Album).artist_id,
          title,
          year: parseInt(year) || 0,
          genre,
          cover_path: coverPath,
        })
        props.onSave(updated)
      }
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className={styles.overlay}
      ref={overlayRef}
      onMouseDown={e => { if (e.target === overlayRef.current) onClose() }}
    >
      <div className={styles.dialog} role="dialog" aria-modal>
        <div className={styles.header}>
          <h2 className={`headline-sm ${styles.headerTitle}`}>Edit metadata</h2>
          <button className="btn btn-icon" onClick={onClose} aria-label="Close">
            <RiCloseLine size={20} />
          </button>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.fields}>

            {/* ── Artist ── */}
            {type === 'artist' && (
              <>
                <Field label="Name" required>
                  <input className={styles.input} value={name} onChange={e => setName(e.target.value)} required />
                </Field>
                <Field label="Bio">
                  <textarea className={`${styles.input} ${styles.textarea}`} value={bio} onChange={e => setBio(e.target.value)} rows={5} />
                </Field>
                <Field label="Image URL">
                  <input className={styles.input} type="url" value={imagePath} onChange={e => setImagePath(e.target.value)} />
                </Field>
              </>
            )}

            {/* ── Album ── */}
            {type === 'album' && (
              <>
                <Field label="Title" required>
                  <input className={styles.input} value={title} onChange={e => setTitle(e.target.value)} required />
                </Field>
                <div className={styles.row}>
                  <Field label="Year">
                    <input className={styles.input} type="number" min="1900" max="2099" value={year} onChange={e => setYear(e.target.value)} />
                  </Field>
                  <Field label="Genre">
                    <input className={styles.input} value={genre} onChange={e => setGenre(e.target.value)} placeholder="e.g. Rock" />
                  </Field>
                </div>
                <Field label="Cover URL">
                  <input className={styles.input} type="url" value={coverPath} onChange={e => setCoverPath(e.target.value)} />
                </Field>
              </>
            )}

            {/* ── Movie / TVShow / Audiobook ── */}
            {type !== 'artist' && type !== 'album' && (
              <>
                <Field label="Title" required>
                  <input className={styles.input} value={title} onChange={e => setTitle(e.target.value)} required />
                </Field>

                {(type === 'movie' || type === 'tvshow') && (
                  <Field label="Original title">
                    <input className={styles.input} value={originalTitle} onChange={e => setOriginalTitle(e.target.value)} />
                  </Field>
                )}

                {type === 'audiobook' && (
                  <div className={styles.row}>
                    <Field label="Author">
                      <input className={styles.input} value={author} onChange={e => setAuthor(e.target.value)} />
                    </Field>
                    <Field label="Narrator">
                      <input className={styles.input} value={narrator} onChange={e => setNarrator(e.target.value)} />
                    </Field>
                  </div>
                )}

                <Field label="Description">
                  <textarea className={`${styles.input} ${styles.textarea}`} value={description} onChange={e => setDescription(e.target.value)} rows={4} />
                </Field>

                <div className={styles.row}>
                  <Field label="Year">
                    <input className={styles.input} type="number" min="1900" max="2099" value={year} onChange={e => setYear(e.target.value)} />
                  </Field>
                  {(type === 'movie' || type === 'tvshow') && (
                    <Field label="Rating">
                      <input className={styles.input} type="number" step="0.1" min="0" max="10" value={rating} onChange={e => setRating(e.target.value)} />
                    </Field>
                  )}
                  {type === 'movie' && (
                    <Field label="Runtime (min)">
                      <input className={styles.input} type="number" min="0" value={runtime} onChange={e => setRuntime(e.target.value)} />
                    </Field>
                  )}
                  {type === 'tvshow' && (
                    <Field label="Status">
                      <input className={styles.input} value={status} onChange={e => setStatus(e.target.value)} placeholder="e.g. Returning Series" />
                    </Field>
                  )}
                </div>

                {type === 'audiobook' ? (
                  <Field label="Genre">
                    <input className={styles.input} value={genre} onChange={e => setGenre(e.target.value)} placeholder="e.g. Science Fiction" />
                  </Field>
                ) : (
                  <Field label="Genres" hint="comma-separated">
                    <input className={styles.input} value={genres} onChange={e => setGenres(e.target.value)} placeholder="Drama, Crime, Thriller" />
                  </Field>
                )}

                {type === 'audiobook' ? (
                  <Field label="Cover URL">
                    <input className={styles.input} type="url" value={coverPath} onChange={e => setCoverPath(e.target.value)} />
                  </Field>
                ) : (
                  <>
                    <Field label="Poster URL">
                      <input className={styles.input} type="url" value={posterPath} onChange={e => setPosterPath(e.target.value)} />
                    </Field>
                    <Field label="Backdrop URL">
                      <input className={styles.input} type="url" value={backdropPath} onChange={e => setBackdropPath(e.target.value)} />
                    </Field>
                  </>
                )}
              </>
            )}

          </div>

          {error && <p className={`label-sm ${styles.errorMsg}`}>{error}</p>}

          <div className={styles.actions}>
            <button type="button" className="btn" onClick={onClose} disabled={saving}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function Field({ label, hint, required, children }: {
  label: string; hint?: string; required?: boolean; children: React.ReactNode
}) {
  return (
    <label className={styles.field}>
      <span className={`label-sm ${styles.label}`}>
        {label}{required && <span className={styles.required}>*</span>}
        {hint && <span className={styles.hint}> ({hint})</span>}
      </span>
      {children}
    </label>
  )
}
