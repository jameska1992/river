import { type FormEvent, useEffect, useState } from 'react'
import {
  RiAddLine,
  RiDeleteBinLine,
  RiEditLine,
  RiFilmLine,
  RiHeadphoneLine,
  RiMusicLine,
  RiTv2Line,
  RiAddCircleLine,
  RiCloseLine,
} from 'react-icons/ri'
import { useLibraries } from '../../context/LibrariesContext'
import { ApiError } from '../../api'
import type { Library, LibraryPath, LibraryType } from '../../api'
import styles from './LibrariesPage.module.css'

const TYPES: { value: LibraryType; label: string; icon: React.ReactNode }[] = [
  { value: 'movie',     label: 'Movies',     icon: <RiFilmLine /> },
  { value: 'tvshow',    label: 'TV Shows',   icon: <RiTv2Line /> },
  { value: 'music',     label: 'Music',      icon: <RiMusicLine /> },
  { value: 'audiobook', label: 'Audiobooks', icon: <RiHeadphoneLine /> },
]

type ModalMode = 'create' | 'edit'

interface FormState {
  name: string
  type: LibraryType
  paths: LibraryPath[]
}

const emptyPath = (): LibraryPath => ({ path: '', pre_transcoded: false })

const defaultForm = (): FormState => ({ name: '', type: 'movie', paths: [emptyPath()] })

export function LibrariesPage() {
  const { libraries, isLoading, error, fetch, create, update, remove } = useLibraries()

  const [modal, setModal] = useState<{ mode: ModalMode; target?: Library } | null>(null)
  const [form, setForm] = useState<FormState>(defaultForm())
  const [formError, setFormError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    void fetch()
  }, [fetch])

  function openCreate() {
    setForm(defaultForm())
    setFormError('')
    setModal({ mode: 'create' })
  }

  function openEdit(lib: Library) {
    setForm({
      name: lib.name,
      type: lib.type,
      paths: lib.paths.length ? lib.paths.map(p => ({ ...p })) : [emptyPath()],
    })
    setFormError('')
    setModal({ mode: 'edit', target: lib })
  }

  function closeModal() {
    setModal(null)
    setFormError('')
  }

  function setPath(index: number, value: string) {
    setForm(f => ({ ...f, paths: f.paths.map((p, i) => i === index ? { ...p, path: value } : p) }))
  }

  function setPathPreTranscoded(index: number, value: boolean) {
    setForm(f => ({ ...f, paths: f.paths.map((p, i) => i === index ? { ...p, pre_transcoded: value } : p) }))
  }

  function addPath() {
    setForm(f => ({ ...f, paths: [...f.paths, emptyPath()] }))
  }

  function removePath(index: number) {
    setForm(f => ({ ...f, paths: f.paths.filter((_, i) => i !== index) }))
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setFormError('')

    const paths = form.paths
      .map(p => ({ ...p, path: p.path.trim() }))
      .filter(p => p.path)
    if (!form.name.trim()) { setFormError('Name is required.'); return }
    if (paths.length === 0) { setFormError('At least one path is required.'); return }

    setSubmitting(true)
    try {
      const data = {
        name: form.name.trim(),
        type: form.type,
        paths,
      }
      if (modal?.mode === 'edit' && modal.target) {
        await update(modal.target.id, data)
      } else {
        await create(data)
      }
      closeModal()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Something went wrong.')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete() {
    if (!deleteId) return
    setDeleting(true)
    try {
      await remove(deleteId)
      setDeleteId(null)
    } catch {
      // keep dialog open on failure
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div>
      <div className={styles.header}>
        <h1 className="headline-lg" style={{ margin: 0 }}>Libraries</h1>
        <button className="btn btn-primary" onClick={openCreate}>
          <RiAddLine size={18} /> Add Library
        </button>
      </div>

      {error && <p className={styles.pageError}>{error}</p>}

      {isLoading ? (
        <p className="label-sm" style={{ color: 'var(--color-on-surface-variant)' }}>Loading…</p>
      ) : libraries.length === 0 ? (
        <div className={`surface-low ${styles.empty}`}>
          <p className="body-md" style={{ color: 'var(--color-on-surface-variant)', margin: 0 }}>
            No libraries yet. Add one to get started.
          </p>
        </div>
      ) : (
        <div className={styles.list}>
          {libraries.map(lib => {
            const cfg = TYPES.find(t => t.value === lib.type)!
            return (
              <div key={lib.id} className={`surface ${styles.row}`}>
                <span className={styles.typeIcon}>{cfg.icon}</span>
                <div className={styles.rowMeta}>
                  <span className="label-md">{lib.name}</span>
                  <span className="label-sm">{cfg.label}</span>
                </div>
                <div className={styles.rowPaths}>
                  {lib.paths.map(p => (
                    <span
                      key={p.path}
                      className={styles.pathChip}
                      title={p.pre_transcoded ? 'Already transcoded — transcoding skipped' : undefined}
                    >
                      {p.path}{p.pre_transcoded && ' ✓'}
                    </span>
                  ))}
                </div>
                <div className={styles.rowActions}>
                  <button className="btn-icon" onClick={() => openEdit(lib)} aria-label="Edit">
                    <RiEditLine size={18} />
                  </button>
                  <button
                    className={`btn-icon ${styles.deleteBtn}`}
                    onClick={() => setDeleteId(lib.id)}
                    aria-label="Delete"
                  >
                    <RiDeleteBinLine size={18} />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Create / Edit modal */}
      {modal && (
        <div className={styles.overlay} onClick={e => { if (e.target === e.currentTarget) closeModal() }}>
          <div className={`glass ${styles.dialog}`} role="dialog" aria-modal>
            <div className={styles.dialogHeader}>
              <h2 className="headline-md" style={{ margin: 0 }}>
                {modal.mode === 'create' ? 'New Library' : 'Edit Library'}
              </h2>
              <button className="btn-icon" onClick={closeModal} aria-label="Close">
                <RiCloseLine size={20} />
              </button>
            </div>

            <form onSubmit={handleSubmit} className={styles.form} noValidate>
              {/* Name */}
              <div className={styles.field}>
                <label htmlFor="lib-name" className="label-md">Name</label>
                <input
                  id="lib-name"
                  type="text"
                  className="input"
                  value={form.name}
                  onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                  autoFocus
                  required
                />
              </div>

              {/* Type — only settable on create */}
              {modal.mode === 'create' && (
                <div className={styles.field}>
                  <span className="label-md">Type</span>
                  <div className={styles.typeGrid}>
                    {TYPES.map(t => (
                      <button
                        key={t.value}
                        type="button"
                        className={`${styles.typeOption} ${form.type === t.value ? styles.typeOptionActive : ''}`}
                        onClick={() => setForm(f => ({ ...f, type: t.value }))}
                      >
                        <span className={styles.typeOptionIcon}>{t.icon}</span>
                        {t.label}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {/* Paths — each carries its own "already transcoded" flag.
                  When checked, the scanner still runs and metadata still
                  enriches, but the video/audio transcoders skip events from
                  that path: its source files ARE the stream files. */}
              <div className={styles.field}>
                <span className="label-md">Paths</span>
                <span className={styles.checkboxHint}>
                  Tick “Already transcoded” for a path whose files are already in the canonical
                  stream format — transcoding is skipped for it; metadata is still collected.
                </span>
                <div className={styles.pathList}>
                  {form.paths.map((p, i) => (
                    <div key={i} className={styles.pathRow}>
                      <input
                        type="text"
                        className="input"
                        value={p.path}
                        onChange={e => setPath(i, e.target.value)}
                        placeholder="/media/movies"
                      />
                      <label className={styles.checkboxRow} title="Already transcoded — skip transcoding for this path">
                        <input
                          type="checkbox"
                          checked={p.pre_transcoded}
                          onChange={e => setPathPreTranscoded(i, e.target.checked)}
                        />
                        <span className="label-sm">Already transcoded</span>
                      </label>
                      {form.paths.length > 1 && (
                        <button
                          type="button"
                          className={`btn-icon ${styles.removePathBtn}`}
                          onClick={() => removePath(i)}
                          aria-label="Remove path"
                        >
                          <RiCloseLine size={18} />
                        </button>
                      )}
                    </div>
                  ))}
                  <button type="button" className={styles.addPathBtn} onClick={addPath}>
                    <RiAddCircleLine size={16} /> Add path
                  </button>
                </div>
              </div>

              {formError && <p className={styles.formError} role="alert">{formError}</p>}

              <div className={styles.dialogFooter}>
                <button type="button" className="btn btn-secondary" onClick={closeModal}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={submitting}>
                  {submitting ? 'Saving…' : modal.mode === 'create' ? 'Create' : 'Save'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete confirmation */}
      {deleteId && (
        <div className={styles.overlay} onClick={e => { if (e.target === e.currentTarget) setDeleteId(null) }}>
          <div className={`glass ${styles.dialog} ${styles.dialogSm}`} role="dialog" aria-modal>
            <h2 className="headline-md" style={{ margin: '0 0 var(--space-2)' }}>Delete library?</h2>
            <p className="body-md" style={{ color: 'var(--color-on-surface-variant)', margin: '0 0 var(--space-4)' }}>
              This will remove the library and all its metadata. Media files on disk are not deleted.
            </p>
            <div className={styles.dialogFooter}>
              <button className="btn btn-secondary" onClick={() => setDeleteId(null)}>Cancel</button>
              <button
                className={`btn ${styles.btnDanger}`}
                onClick={handleDelete}
                disabled={deleting}
              >
                {deleting ? 'Deleting…' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
