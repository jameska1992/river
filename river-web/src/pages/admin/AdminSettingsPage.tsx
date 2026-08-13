import { type FormEvent, useEffect, useState } from 'react'
import { RiCheckLine, RiCloseLine, RiExchangeFundsLine, RiFilmLine, RiMusic2Line, RiTimeLine } from 'react-icons/ri'
import { api, ApiError } from '../../api'
import type { IntegrationSettings, TranscodingSettings } from '../../api'
import styles from './AdminSettingsPage.module.css'

type Tab = 'integrations' | 'metadata' | 'scanning' | 'transcoding'

const TABS: { id: Tab; label: string }[] = [
  { id: 'integrations', label: 'Integrations' },
  { id: 'metadata', label: 'Metadata' },
  { id: 'scanning', label: 'Scanning' },
  { id: 'transcoding', label: 'Transcoding' },
]

export function AdminSettingsPage() {
  const [tab, setTab] = useState<Tab>('integrations')

  return (
    <div>
      <h1 className={`headline-sm ${styles.title}`}>Settings</h1>

      <div className={styles.tabs} role="tablist">
        {TABS.map(t => (
          <button
            key={t.id}
            role="tab"
            aria-selected={tab === t.id}
            className={`${styles.tab} ${tab === t.id ? styles.tabActive : ''}`}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'integrations' && <IntegrationsTab />}
      {tab === 'metadata' && <MetadataTab />}
      {tab === 'scanning' && <ScanningTab />}
      {tab === 'transcoding' && <TranscodingTab />}
    </div>
  )
}

// ── Transcoding tab ───────────────────────────────────────

const MAX_HEIGHT_OPTIONS = [
  { value: 0, label: 'No cap' },
  { value: 720, label: '720p' },
  { value: 1080, label: '1080p' },
  { value: 2160, label: '2160p (4K)' },
]
const NVENC_PRESETS = ['p1', 'p2', 'p3', 'p4', 'p5', 'p6', 'p7']
const X264_PRESETS = [
  'ultrafast', 'superfast', 'veryfast', 'faster', 'fast',
  'medium', 'slow', 'slower', 'veryslow',
]
const BITRATES = [128, 192, 256, 320]

function TranscodingTab() {
  const [loading, setLoading] = useState(true)
  const [settings, setSettings] = useState<TranscodingSettings | null>(null)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.getTranscodingSettings()
      .then(setSettings)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load settings'))
      .finally(() => setLoading(false))
  }, [])

  // set updates one field while preserving the rest; clearing the saved
  // note so it only shows immediately after a successful save.
  function set<K extends keyof TranscodingSettings>(key: K, value: TranscodingSettings[K]) {
    setSettings(s => (s ? { ...s, [key]: value } : s))
    setSaved(false)
  }

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    if (!settings) return
    setSaving(true); setSaved(false); setError('')
    try {
      setSettings(await api.updateTranscodingSettings(settings))
      setSaved(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  if (loading || !settings) {
    return <div className={`${styles.skeletonCard} skeleton`} style={{ maxWidth: 460 }} />
  }

  return (
    <form onSubmit={handleSave}>
      <p className={`body-md ${styles.blurb}`}>
        How new media is transcoded on ingest. Changes apply to the next transcode job —
        no restart needed — and don't re-process existing files. Defaults match River's
        built-in profile (1080p, quality 23, NVENC p3 / libx264 medium).
      </p>

      <div className={`card ${styles.card}`} style={{ maxWidth: 460 }}>
        <div className={styles.cardHead}>
          <span className={styles.cardIcon} aria-hidden><RiFilmLine size={18} /></span>
          <div>
            <h2 className={`label-lg ${styles.cardTitle}`}>Video</h2>
            <p className={`label-sm ${styles.cardSubtitle}`}>Movies &amp; TV episodes</p>
          </div>
        </div>

        <label className={styles.field}>
          <span className="label-sm">Max resolution</span>
          <select
            className="input"
            value={settings.max_height}
            onChange={e => set('max_height', Number(e.target.value))}
          >
            {MAX_HEIGHT_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </label>

        <label className={styles.field}>
          <span className="label-sm">Quality ({settings.quality}) — lower is better, 0–51</span>
          <input
            className="input"
            type="number"
            min={0}
            max={51}
            value={settings.quality}
            onChange={e => set('quality', Number(e.target.value))}
          />
        </label>

        <label className={styles.field}>
          <span className="label-sm">NVENC preset (GPU) — p1 fastest, p7 best</span>
          <select
            className="input"
            value={settings.nvenc_preset}
            onChange={e => set('nvenc_preset', e.target.value)}
          >
            {NVENC_PRESETS.map(p => <option key={p} value={p}>{p}</option>)}
          </select>
        </label>

        <label className={styles.field}>
          <span className="label-sm">libx264 preset (CPU) — faster is lower quality</span>
          <select
            className="input"
            value={settings.x264_preset}
            onChange={e => set('x264_preset', e.target.value)}
          >
            {X264_PRESETS.map(p => <option key={p} value={p}>{p}</option>)}
          </select>
        </label>

        <label className={styles.field}>
          <span className="label-sm">Audio bitrate (non-AAC tracks)</span>
          <select
            className="input"
            value={settings.audio_bitrate}
            onChange={e => set('audio_bitrate', Number(e.target.value))}
          >
            {BITRATES.map(b => <option key={b} value={b}>{b} kbps</option>)}
          </select>
        </label>

        <label className={styles.checkbox}>
          <input
            type="checkbox"
            checked={settings.force_cpu}
            onChange={e => set('force_cpu', e.target.checked)}
          />
          <span className="label-sm">Force CPU encoding (skip GPU/NVENC even when available)</span>
        </label>
      </div>

      <div className={`card ${styles.card}`} style={{ maxWidth: 460 }}>
        <div className={styles.cardHead}>
          <span className={styles.cardIcon} aria-hidden><RiMusic2Line size={18} /></span>
          <div>
            <h2 className={`label-lg ${styles.cardTitle}`}>Audio</h2>
            <p className={`label-sm ${styles.cardSubtitle}`}>Music &amp; audiobooks</p>
          </div>
        </div>

        <label className={styles.field}>
          <span className="label-sm">Bitrate</span>
          <select
            className="input"
            value={settings.music_bitrate}
            onChange={e => set('music_bitrate', Number(e.target.value))}
          >
            {BITRATES.map(b => <option key={b} value={b}>{b} kbps</option>)}
          </select>
        </label>
      </div>

      {error && <p className={styles.formError}>{error}</p>}

      <div className={styles.footer}>
        {saved && <span className={styles.savedNote}><RiCheckLine size={15} /> Saved</span>}
        <button type="submit" className="btn btn-primary" disabled={saving}>
          {saving ? 'Saving…' : 'Save changes'}
        </button>
      </div>
    </form>
  )
}

// ── Scanning tab ──────────────────────────────────────────

function ScanningTab() {
  const [loading, setLoading] = useState(true)
  const [interval, setInterval] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.getScanningSettings()
      .then(s => setInterval(s.scan_interval))
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load settings'))
      .finally(() => setLoading(false))
  }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setSaving(true); setSaved(false); setError('')
    try {
      const s = await api.updateScanningSettings(interval.trim())
      setInterval(s.scan_interval)
      setSaved(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className={`${styles.skeletonCard} skeleton`} style={{ maxWidth: 400 }} />
  }

  return (
    <form onSubmit={handleSave}>
      <p className={`body-md ${styles.blurb}`}>
        How often river-scan rescans your libraries for new media. Use a duration like
        {' '}<code>1h</code>, <code>30m</code>, or <code>3600s</code>. Changes take effect on the
        next scan cycle — no restart needed.
      </p>

      <div className={`card ${styles.card}`} style={{ maxWidth: 400 }}>
        <div className={styles.cardHead}>
          <span className={styles.cardIcon} aria-hidden><RiTimeLine size={18} /></span>
          <div>
            <h2 className={`label-lg ${styles.cardTitle}`}>Scan interval</h2>
            <p className={`label-sm ${styles.cardSubtitle}`}>Library rescan frequency</p>
          </div>
        </div>

        <label className={styles.field}>
          <span className="label-sm">Interval</span>
          <input
            className="input"
            value={interval}
            onChange={e => setInterval(e.target.value)}
            placeholder="1h"
          />
        </label>
      </div>

      {error && <p className={styles.formError}>{error}</p>}

      <div className={styles.footer}>
        {saved && <span className={styles.savedNote}><RiCheckLine size={15} /> Saved</span>}
        <button type="submit" className="btn btn-primary" disabled={saving}>
          {saving ? 'Saving…' : 'Save changes'}
        </button>
      </div>
    </form>
  )
}

// ── Metadata tab ──────────────────────────────────────────

function MetadataTab() {
  const [loading, setLoading] = useState(true)
  const [hasKey, setHasKey] = useState(false)
  const [tmdbKey, setTmdbKey] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.getMetadataSettings()
      .then(s => setHasKey(s.tmdb_has_key))
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load settings'))
      .finally(() => setLoading(false))
  }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setSaving(true); setSaved(false); setError('')
    try {
      const s = await api.updateMetadataSettings(tmdbKey)
      setHasKey(s.tmdb_has_key)
      setTmdbKey('')
      setSaved(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <div className={`${styles.skeletonCard} skeleton`} style={{ maxWidth: 400 }} />
  }

  return (
    <form onSubmit={handleSave}>
      <p className={`body-md ${styles.blurb}`}>
        The TMDB API key powers movie and TV metadata enrichment. Get one free at{' '}
        <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noreferrer">themoviedb.org</a>.
        Changes are picked up by the metadata services within a few minutes — no restart needed.
      </p>

      <div className={`card ${styles.card}`} style={{ maxWidth: 400 }}>
        <div className={styles.cardHead}>
          <span className={styles.cardIcon} aria-hidden><RiExchangeFundsLine size={18} /></span>
          <div>
            <h2 className={`label-lg ${styles.cardTitle}`}>TMDB</h2>
            <p className={`label-sm ${styles.cardSubtitle}`}>The Movie Database</p>
          </div>
        </div>

        <label className={styles.field}>
          <span className="label-sm">API key</span>
          <input
            className="input"
            type="password"
            autoComplete="off"
            value={tmdbKey}
            onChange={e => setTmdbKey(e.target.value)}
            placeholder={hasKey ? '•••••••••••• (leave blank to keep)' : 'Enter TMDB API key'}
          />
        </label>
      </div>

      {error && <p className={styles.formError}>{error}</p>}

      <div className={styles.footer}>
        {saved && <span className={styles.savedNote}><RiCheckLine size={15} /> Saved</span>}
        <button type="submit" className="btn btn-primary" disabled={saving}>
          {saving ? 'Saving…' : 'Save changes'}
        </button>
      </div>
    </form>
  )
}

// ── Integrations tab ──────────────────────────────────────

function IntegrationsTab() {
  const [loading, setLoading] = useState(true)
  const [settings, setSettings] = useState<IntegrationSettings | null>(null)
  const [radarrUrl, setRadarrUrl] = useState('')
  const [radarrKey, setRadarrKey] = useState('')
  const [sonarrUrl, setSonarrUrl] = useState('')
  const [sonarrKey, setSonarrKey] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.getIntegrationSettings()
      .then(s => {
        setSettings(s)
        setRadarrUrl(s.radarr_url)
        setSonarrUrl(s.sonarr_url)
      })
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load settings'))
      .finally(() => setLoading(false))
  }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setSaving(true); setSaved(false); setError('')
    try {
      const s = await api.updateIntegrationSettings({
        radarr_url: radarrUrl.trim(),
        radarr_api_key: radarrKey,
        sonarr_url: sonarrUrl.trim(),
        sonarr_api_key: sonarrKey,
      })
      setSettings(s)
      setRadarrKey('')
      setSonarrKey('')
      setSaved(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className={styles.cards}>
        {Array.from({ length: 2 }).map((_, i) => (
          <div key={i} className={`${styles.skeletonCard} skeleton`} />
        ))}
      </div>
    )
  }

  return (
    <form onSubmit={handleSave}>
      <p className={`body-md ${styles.blurb}`}>
        Connect Radarr and Sonarr to enable the Requests feature and the upcoming-releases
        calendar. Leave a URL blank to disable that integration.
      </p>

      <div className={styles.cards}>
        <ServiceCard
          title="Radarr"
          subtitle="Movies"
          service="radarr"
          url={radarrUrl} setUrl={setRadarrUrl}
          apiKey={radarrKey} setApiKey={setRadarrKey}
          hasKey={settings?.radarr_has_key ?? false}
        />
        <ServiceCard
          title="Sonarr"
          subtitle="TV shows"
          service="sonarr"
          url={sonarrUrl} setUrl={setSonarrUrl}
          apiKey={sonarrKey} setApiKey={setSonarrKey}
          hasKey={settings?.sonarr_has_key ?? false}
        />
      </div>

      {error && <p className={styles.formError}>{error}</p>}

      <div className={styles.footer}>
        <span className={`label-sm ${styles.hint}`}>Test connection checks the saved settings — save first.</span>
        {saved && <span className={styles.savedNote}><RiCheckLine size={15} /> Saved</span>}
        <button type="submit" className="btn btn-primary" disabled={saving}>
          {saving ? 'Saving…' : 'Save changes'}
        </button>
      </div>
    </form>
  )
}

// ── one integration ───────────────────────────────────────

function ServiceCard({ title, subtitle, service, url, setUrl, apiKey, setApiKey, hasKey }: {
  title: string
  subtitle: string
  service: 'radarr' | 'sonarr'
  url: string; setUrl: (v: string) => void
  apiKey: string; setApiKey: (v: string) => void
  hasKey: boolean
}) {
  const [testing, setTesting] = useState(false)
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null)

  async function handleTest() {
    setTesting(true); setResult(null)
    try {
      setResult(await api.testIntegration(service))
    } catch (err) {
      setResult({ ok: false, message: err instanceof ApiError ? err.message : 'Test failed' })
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className={`card ${styles.card}`}>
      <div className={styles.cardHead}>
        <span className={styles.cardIcon} aria-hidden><RiExchangeFundsLine size={18} /></span>
        <div>
          <h2 className={`label-lg ${styles.cardTitle}`}>{title}</h2>
          <p className={`label-sm ${styles.cardSubtitle}`}>{subtitle}</p>
        </div>
      </div>

      <label className={styles.field}>
        <span className="label-sm">URL</span>
        <input
          className="input"
          value={url}
          onChange={e => setUrl(e.target.value)}
          placeholder={`http://${service}:${service === 'radarr' ? '7878' : '8989'}`}
        />
      </label>

      <label className={styles.field}>
        <span className="label-sm">API key</span>
        <input
          className="input"
          type="password"
          autoComplete="off"
          value={apiKey}
          onChange={e => setApiKey(e.target.value)}
          placeholder={hasKey ? '•••••••••••• (leave blank to keep)' : 'Enter API key'}
        />
      </label>

      <div className={styles.cardFooter}>
        <button type="button" className="btn" onClick={handleTest} disabled={testing}>
          {testing ? 'Testing…' : 'Test connection'}
        </button>
        {result && (
          <span className={`${styles.testResult} ${result.ok ? styles.testOk : styles.testFail}`}>
            {result.ok ? <RiCheckLine size={15} /> : <RiCloseLine size={15} />}
            {result.message}
          </span>
        )}
      </div>
    </div>
  )
}
