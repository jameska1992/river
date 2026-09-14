import type { Credits, SetCreditsRequest } from '../api'

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
