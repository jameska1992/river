import { type FormEvent, useEffect, useState } from 'react'
import { RiCheckLine, RiCloseLine, RiExchangeFundsLine } from 'react-icons/ri'
import { api, ApiError } from '../../api'
import type { IntegrationSettings } from '../../api'
import styles from './AdminSettingsPage.module.css'

type Tab = 'integrations'

const TABS: { id: Tab; label: string }[] = [
  { id: 'integrations', label: 'Integrations' },
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
    </div>
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
