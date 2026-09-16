import type { CastCredit, CrewCredit } from '../api'

// A person tile in a cast/crew rail.
export interface PersonEntry {
  key: string
  name: string
  role?: string
  imagePath?: string
}

// Cast in billing order, character as the role.
export function castToEntries(cast: CastCredit[]): PersonEntry[] {
  return [...cast]
    .sort((a, b) => a.order - b.order)
    .map(c => ({
      key: `cast-${c.person_id}-${c.order}`,
      name: c.name,
      role: c.character || undefined,
      imagePath: c.profile_path || undefined,
    }))
}

// Crew deduped by person — a person credited with several jobs (e.g. Writer +
// Producer) shows once with the jobs joined. First-seen order is preserved.
export function crewToEntries(crew: CrewCredit[]): PersonEntry[] {
  const byPerson = new Map<string, { name: string; imagePath: string; jobs: string[] }>()
  for (const c of crew) {
    const entry = byPerson.get(c.person_id) ?? { name: c.name, imagePath: c.profile_path, jobs: [] }
    if (c.job && !entry.jobs.includes(c.job)) entry.jobs.push(c.job)
    byPerson.set(c.person_id, entry)
  }
  return [...byPerson.entries()].map(([personId, e]) => ({
    key: `crew-${personId}`,
    name: e.name,
    role: e.jobs.join(', ') || undefined,
    imagePath: e.imagePath || undefined,
  }))
}
