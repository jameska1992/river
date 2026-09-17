import type { Credits, CrewCredit, SetCreditsRequest } from '../api'

// One crew person for display: their jobs deduped and joined (e.g. a person
// credited as both Writer and Producer appears once as "Writer, Producer").
export interface CrewEntry {
  person_id: string
  name: string
  profile_path: string
  jobs: string
}

// Dedupe crew per person, joining their distinct jobs, first-seen order kept.
// Keyed by person_id, falling back to name when a person has no id.
export function dedupeCrew(crew: CrewCredit[]): CrewEntry[] {
  const byPerson = new Map<string, CrewEntry>()
  for (const c of crew) {
    const key = c.person_id || c.name
    const existing = byPerson.get(key)
    if (existing) {
      if (c.job && !existing.jobs.split(', ').includes(c.job)) {
        existing.jobs = existing.jobs ? `${existing.jobs}, ${c.job}` : c.job
      }
    } else {
      byPerson.set(key, {
        person_id: c.person_id,
        name: c.name,
        profile_path: c.profile_path,
        jobs: c.job || '',
      })
    }
  }
  return [...byPerson.values()]
}

// creditsToRequest maps a fetched Credits object back into the write shape,
// carrying tmdb_id through so existing people dedupe rather than duplicate.
// Used to re-submit the current cast/crew unchanged while flipping the lock
// (e.g. the "unlock / hand back to TMDB" action).
export function creditsToRequest(credits: Credits, locked: boolean): SetCreditsRequest {
  return {
    cast: credits.cast.map(c => ({
      ...(c.tmdb_id ? { tmdb_id: c.tmdb_id } : {}),
      name: c.name,
      profile_path: c.profile_path,
      character: c.character,
      order: c.order,
    })),
    crew: credits.crew.map(c => ({
      ...(c.tmdb_id ? { tmdb_id: c.tmdb_id } : {}),
      name: c.name,
      profile_path: c.profile_path,
      job: c.job,
      department: c.department,
    })),
    locked,
  }
}
