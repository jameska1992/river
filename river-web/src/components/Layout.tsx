import { Outlet } from 'react-router-dom'
import { NavBar } from './NavBar'
import styles from './Layout.module.css'

export function Layout() {
  return (
    <div className={styles.root}>
      <NavBar />
      <main className={styles.content}>
        <Outlet />
      </main>
    </div>
  )
}
