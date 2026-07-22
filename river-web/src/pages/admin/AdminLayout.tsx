import { NavLink, Outlet } from 'react-router-dom'
import { RiDashboard2Line, RiFolder3Line, RiUploadCloud2Line, RiTeamLine, RiFileListLine, RiQuestionLine, RiDatabase2Line } from 'react-icons/ri'
import styles from './AdminLayout.module.css'

const navItems = [
  { to: '/admin',                label: 'Overview',      icon: <RiDashboard2Line />, end: true },
  { to: '/admin/libraries',      label: 'Libraries',     icon: <RiFolder3Line /> },
  { to: '/admin/upload',         label: 'Upload',        icon: <RiUploadCloud2Line /> },
  { to: '/admin/unidentified',   label: 'Unidentified',  icon: <RiQuestionLine /> },
  { to: '/admin/scanner-state',  label: 'Scanner State', icon: <RiDatabase2Line /> },
  { to: '/admin/users',          label: 'Users',         icon: <RiTeamLine /> },
  { to: '/admin/logs',           label: 'Logs',          icon: <RiFileListLine /> },
]

export function AdminLayout() {
  return (
    <div className={styles.root}>
      <aside className={styles.sidebar}>
        <p className={`label-sm ${styles.sidebarLabel}`}>Admin</p>
        <nav className={styles.nav}>
          {navItems.map(item => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                `${styles.navItem} ${isActive ? styles.navItemActive : ''}`
              }
            >
              <span className={styles.navIcon} aria-hidden>{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>

      <main className={styles.content}>
        <Outlet />
      </main>
    </div>
  )
}
