import { type FormEvent, useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import { useAuth } from '../context/AuthContext'
import styles from './SettingsPage.module.css'

export function SettingsPage() {
  const { user, refreshUser } = useAuth()

  // Profile form
  const [email, setEmail] = useState('')
  const [profileSaving, setProfileSaving] = useState(false)
  const [profileSuccess, setProfileSuccess] = useState(false)
  const [profileError, setProfileError] = useState<string | null>(null)

  // Password form
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [passwordSuccess, setPasswordSuccess] = useState(false)
  const [passwordError, setPasswordError] = useState<string | null>(null)

  useEffect(() => {
    if (user) {
      setEmail(user.email)
    }
  }, [user])

  async function handleProfileSave(e: FormEvent) {
    e.preventDefault()
    setProfileSaving(true)
    setProfileSuccess(false)
    setProfileError(null)
    try {
      await api.updateMe(email)
      await refreshUser()
      setProfileSuccess(true)
    } catch (err) {
      setProfileError(err instanceof ApiError ? err.message : 'Failed to save profile')
    } finally {
      setProfileSaving(false)
    }
  }

  async function handlePasswordChange(e: FormEvent) {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      setPasswordError('New passwords do not match')
      return
    }
    setPasswordSaving(true)
    setPasswordSuccess(false)
    setPasswordError(null)
    try {
      await api.changePassword(currentPassword, newPassword)
      setPasswordSuccess(true)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setPasswordError('Current password is incorrect')
      } else {
        setPasswordError(err instanceof ApiError ? err.message : 'Failed to change password')
      }
    } finally {
      setPasswordSaving(false)
    }
  }

  return (
    <div className="container" style={{ paddingTop: 'var(--space-5)' }}>
      <h1 className={`headline-sm ${styles.heading}`}>Settings</h1>

      <div className={styles.grid}>
      {/* Profile */}
      <section className={styles.section}>
        <h2 className={`label-md ${styles.sectionTitle}`}>Profile</h2>
        <form onSubmit={handleProfileSave} className={styles.form}>
          <div className={styles.field}>
            <span className={`label-sm ${styles.label}`}>Username</span>
            <span className={`body-sm ${styles.readOnly}`}>{user?.username}</span>
          </div>
          <div className={styles.field}>
            <label className={`label-sm ${styles.label}`} htmlFor="email">Email</label>
            <input
              id="email"
              className="input"
              type="email"
              value={email}
              onChange={e => { setEmail(e.target.value); setProfileSuccess(false) }}
              required
            />
          </div>
          {profileError && <p className={styles.error}>{profileError}</p>}
          {profileSuccess && <p className={styles.success}>Profile updated.</p>}
          <button className="btn btn-primary" type="submit" disabled={profileSaving}>
            {profileSaving ? 'Saving…' : 'Save'}
          </button>
        </form>
      </section>

      {/* Password */}
      <section className={styles.section}>
        <h2 className={`label-md ${styles.sectionTitle}`}>Change Password</h2>
        <form onSubmit={handlePasswordChange} className={styles.form}>
          <div className={styles.field}>
            <label className={`label-sm ${styles.label}`} htmlFor="current-password">Current password</label>
            <input
              id="current-password"
              className="input"
              type="password"
              value={currentPassword}
              onChange={e => { setCurrentPassword(e.target.value); setPasswordSuccess(false); setPasswordError(null) }}
              required
              autoComplete="current-password"
            />
          </div>
          <div className={styles.field}>
            <label className={`label-sm ${styles.label}`} htmlFor="new-password">New password</label>
            <input
              id="new-password"
              className="input"
              type="password"
              value={newPassword}
              onChange={e => { setNewPassword(e.target.value); setPasswordSuccess(false); setPasswordError(null) }}
              minLength={8}
              required
              autoComplete="new-password"
            />
          </div>
          <div className={styles.field}>
            <label className={`label-sm ${styles.label}`} htmlFor="confirm-password">Confirm new password</label>
            <input
              id="confirm-password"
              className="input"
              type="password"
              value={confirmPassword}
              onChange={e => { setConfirmPassword(e.target.value); setPasswordSuccess(false); setPasswordError(null) }}
              minLength={8}
              required
              autoComplete="new-password"
            />
          </div>
          {passwordError && <p className={styles.error}>{passwordError}</p>}
          {passwordSuccess && <p className={styles.success}>Password changed.</p>}
          <button className="btn btn-primary" type="submit" disabled={passwordSaving}>
            {passwordSaving ? 'Saving…' : 'Change password'}
          </button>
        </form>
      </section>
      </div>
    </div>
  )
}
