import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { RiArrowLeftSLine, RiArrowRightSLine, RiUserLine } from 'react-icons/ri'
import type { CrewCredit } from '../api'
import { dedupeCrew } from '../util/credits'
import { imageUrl } from '../util/imageUrl'
import styles from './CrewCarousel.module.css'

// CrewCarousel renders a title's crew as a single horizontal, scrollable row of
// person cards (photo + name + role), sitting below the cast/details block and
// above the "More like this" suggestions. Shared by the movie and TV detail
// pages so their crew presentation can't drift apart (#192). Crew is deduped
// per person (jobs joined) via the same helper the cast/crew editor uses.
//
// Renders nothing when there's no crew — better to omit the row than show an
// empty heading. Mirrors the scroll chrome (arrows / edge fades / smooth
// scroll-by) of SimilarCarousel for visual consistency with the other rails.
export function CrewCarousel({ crew }: { crew: CrewCredit[] }) {
  const people = dedupeCrew(crew)
  const trackRef = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)

  const syncArrows = () => {
    const el = trackRef.current
    if (!el) return
    setCanLeft(el.scrollLeft > 4)
    setCanRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 4)
  }

  // Re-evaluate arrow visibility once cards render and on track resize.
  useEffect(() => {
    syncArrows()
    const el = trackRef.current
    if (!el) return
    const ro = new ResizeObserver(syncArrows)
    ro.observe(el)
    return () => ro.disconnect()
  }, [people.length])

  const scroll = (dir: -1 | 1) => {
    const el = trackRef.current
    if (!el) return
    el.scrollBy({ left: el.clientWidth * 0.8 * dir, behavior: 'smooth' })
  }

  if (people.length === 0) return null

  return (
    <section className={styles.section}>
      <h2 className={`headline-sm ${styles.heading}`}>Crew</h2>
      <div className={styles.carousel}>
        {canLeft && <div className={`${styles.fade} ${styles.fadeLeft}`} />}
        {canRight && <div className={`${styles.fade} ${styles.fadeRight}`} />}
        {canLeft && (
          <button className={`${styles.arrow} ${styles.arrowLeft}`} onClick={() => scroll(-1)} aria-label="Scroll left">
            <RiArrowLeftSLine size={26} />
          </button>
        )}
        {canRight && (
          <button className={`${styles.arrow} ${styles.arrowRight}`} onClick={() => scroll(1)} aria-label="Scroll right">
            <RiArrowRightSLine size={26} />
          </button>
        )}
        <div ref={trackRef} className={styles.row} onScroll={syncArrows}>
          {people.map(c => {
            const inner = (
              <>
                <div className={styles.photo}>
                  {c.profile_path ? <img src={imageUrl(c.profile_path)} alt={c.name} loading="lazy" /> : <RiUserLine size={24} />}
                </div>
                <span className={`label-sm ${styles.name}`}>{c.name}</span>
                {c.jobs && <span className={`label-sm ${styles.job}`}>{c.jobs}</span>}
              </>
            )
            // Manually-added crew can lack a person_id — render a non-link card.
            return c.person_id
              ? <Link key={c.person_id} to={`/person/${c.person_id}`} className={styles.card}>{inner}</Link>
              : <div key={c.name} className={styles.card}>{inner}</div>
          })}
        </div>
      </div>
    </section>
  )
}
